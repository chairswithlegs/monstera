package activitypub

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

const ianaWebFingerTimeout = 5 * time.Second

// JRDLink is a link in a WebFinger JRD (RFC 7033).
type JRDLink struct {
	Rel  string `json:"rel"`
	Type string `json:"type,omitempty"`
	Href string `json:"href"`
}

// JRD is a WebFinger JSON Resource Descriptor (RFC 7033).
type JRD struct {
	Subject string    `json:"subject"`
	Links   []JRDLink `json:"links"`
	Aliases []string  `json:"aliases,omitempty"`
}

// RemoteAccountResolver resolves a remote account by acct (user@domain) via WebFinger and actor fetch.
// Used by the Mastodon search API when resolve=true.
type RemoteAccountResolver struct {
	accounts       service.AccountService
	instanceDomain string
	httpClient     *http.Client
}

// NewRemoteAccountResolver returns a resolver that uses the given account service and actor fetch.
// instanceDomain is used to skip resolution for local accounts (e.g. "example.com").
func NewRemoteAccountResolver(accounts service.AccountService, instanceDomain string, cfg *config.Config) *RemoteAccountResolver {
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       ianaWebFingerTimeout,
	}
	if cfg.FederationInsecureSkipTLS {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // G402: intentional for development federation with self-signed certs
		}
	}
	return &RemoteAccountResolver{
		accounts:       accounts,
		instanceDomain: strings.TrimSpace(strings.ToLower(instanceDomain)),
		httpClient:     client,
	}
}

// ResolveRemoteAccount resolves the given acct (user@domain) to a domain.Account.
// For local accounts (domain matches instanceDomain), looks up from store.
// For remote accounts, fetches WebFinger, then the actor document, and creates or updates the account.
func (r *RemoteAccountResolver) ResolveRemoteAccount(ctx context.Context, acct string) (*domain.Account, error) {
	acct = strings.TrimSpace(acct)
	parts := strings.SplitN(acct, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("ResolveRemoteAccount: invalid acct %q", acct)
	}
	username, acctDomain := parts[0], strings.ToLower(parts[1])
	if acctDomain == r.instanceDomain {
		slog.DebugContext(ctx, "ResolveRemoteAccount called with local account, using local resolution", slog.String("username", username), slog.String("acctDomain", acctDomain))
		acc, err := r.accounts.GetLocalByUsername(ctx, username)
		if err != nil {
			return nil, fmt.Errorf("ResolveRemoteAccount(local): %w", err)
		}
		return acc, nil
	}
	existing, err := r.accounts.GetByUsername(ctx, username, &acctDomain)
	if err == nil {
		return existing, nil
	}
	actorIRI, err := r.fetchWebFingerActorIRI(ctx, acct)
	if err != nil {
		return nil, fmt.Errorf("ResolveRemoteAccount webfinger: %w", err)
	}
	actor, err := r.FetchActor(ctx, actorIRI)
	if err != nil {
		return nil, fmt.Errorf("ResolveRemoteAccount actor fetch: %w", err)
	}
	return r.syncActorToStore(ctx, actor)
}

// FetchActor fetches an Actor document from the given IRI using HTTP GET.
func (r *RemoteAccountResolver) FetchActor(ctx context.Context, actorIRI string) (*Actor, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actorIRI, nil)
	if err != nil {
		return nil, fmt.Errorf("actor fetch new request: %w", err)
	}
	req.Header.Set("Accept", "application/activity+json")
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("actor fetch request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("actor fetch: status %d", resp.StatusCode)
	}
	var actor Actor
	if err := json.NewDecoder(resp.Body).Decode(&actor); err != nil {
		return nil, fmt.Errorf("actor fetch decode: %w", err)
	}
	return &actor, nil
}

func (r *RemoteAccountResolver) fetchWebFingerActorIRI(ctx context.Context, acct string) (string, error) {
	parts := strings.SplitN(acct, "@", 2)
	if len(parts) != 2 {
		return "", errors.New("invalid acct")
	}
	resource := "acct:" + acct
	u := "https://" + parts[1] + "/.well-known/webfinger?resource=" + resource
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("webfinger request: %w", err)
	}
	req.Header.Set("Accept", "application/jrd+json, application/json")
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("webfinger do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("webfinger returned %d", resp.StatusCode)
	}
	var jrd JRD
	if err := json.NewDecoder(resp.Body).Decode(&jrd); err != nil {
		return "", fmt.Errorf("webfinger decode: %w", err)
	}
	for _, link := range jrd.Links {
		if link.Rel == "self" && (link.Type == "application/activity+json" || link.Type == "") && link.Href != "" {
			return link.Href, nil
		}
	}
	return "", errors.New("webfinger: no self link with application/activity+json")
}

func (r *RemoteAccountResolver) syncActorToStore(ctx context.Context, actor *Actor) (*domain.Account, error) {
	dom := DomainFromActorID(actor.ID)
	username := usernameFromActorIRI(actor.ID, dom)
	if username == "" {
		username = "unknown"
	}
	apRaw, _ := json.Marshal(actor)
	in := service.CreateOrUpdateRemoteInput{
		APID:         actor.ID,
		Username:     username,
		Domain:       dom,
		DisplayName:  &actor.Name,
		Note:         &actor.Summary,
		PublicKey:    actor.PublicKey.PublicKeyPem,
		InboxURL:     actor.Inbox,
		OutboxURL:    actor.Outbox,
		FollowersURL: actor.Followers,
		FollowingURL: actor.Following,
		Bot:          actor.Type == actorTypeService,
		Locked:       actor.ManuallyApprovesFollowers,
		ApRaw:        apRaw,
	}
	acc, err := r.accounts.CreateOrUpdateRemoteAccount(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("syncActorToStore: %w", err)
	}
	return acc, nil
}
