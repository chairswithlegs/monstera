package activitypub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
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
	store           store.Store
	actorFetch      func(ctx context.Context, actorIRI string) (*Actor, error)
	instanceDomain  string
	webfingerClient *http.Client
}

// NewRemoteAccountResolver returns a resolver that uses the given store and actor fetch.
// instanceDomain is used to skip resolution for local accounts (e.g. "example.com").
// If client is non-nil, it is used for WebFinger requests (e.g. with InsecureSkipVerify for development).
func NewRemoteAccountResolver(s store.Store, actorFetch func(context.Context, string) (*Actor, error), instanceDomain string, client *http.Client) *RemoteAccountResolver {
	if client == nil {
		client = &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
			Timeout:       ianaWebFingerTimeout,
		}
	}
	return &RemoteAccountResolver{
		store:           s,
		actorFetch:      actorFetch,
		instanceDomain:  strings.TrimSpace(strings.ToLower(instanceDomain)),
		webfingerClient: client,
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
		acc, err := r.store.GetLocalAccountByUsername(ctx, username)
		if err != nil {
			return nil, fmt.Errorf("ResolveRemoteAccount(local): %w", err)
		}
		return acc, nil
	}
	existing, err := r.store.GetRemoteAccountByUsername(ctx, username, &acctDomain)
	if err == nil {
		return existing, nil
	}
	actorIRI, err := r.fetchWebFingerActorIRI(ctx, acct)
	if err != nil {
		return nil, fmt.Errorf("ResolveRemoteAccount webfinger: %w", err)
	}
	if r.actorFetch == nil {
		return nil, errors.New("ResolveRemoteAccount: actor fetch not configured")
	}
	actor, err := r.actorFetch(ctx, actorIRI)
	if err != nil {
		return nil, fmt.Errorf("ResolveRemoteAccount actor fetch: %w", err)
	}
	return r.syncActorToStore(ctx, actor)
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
	resp, err := r.webfingerClient.Do(req)
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
	existing, err := r.store.GetAccountByAPID(ctx, actor.ID)
	if err != nil {
		username := usernameFromActorIRI(actor.ID, "")
		if username == "" {
			username = "unknown"
		}
		dom := DomainFromActorID(actor.ID)
		apRaw, _ := json.Marshal(actor)
		in := store.CreateAccountInput{
			ID:           uid.New(),
			Username:     username,
			Domain:       &dom,
			DisplayName:  strPtr(actor.Name),
			Note:         strPtr(actor.Summary),
			PublicKey:    actor.PublicKey.PublicKeyPem,
			InboxURL:     actor.Inbox,
			OutboxURL:    actor.Outbox,
			FollowersURL: actor.Followers,
			FollowingURL: actor.Following,
			APID:         actor.ID,
			ApRaw:        apRaw,
			Bot:          actor.Type == actorTypeService,
			Locked:       actor.ManuallyApprovesFollowers,
		}
		acc, createErr := r.store.CreateAccount(ctx, in)
		if createErr != nil {
			return nil, fmt.Errorf("syncActorToStore CreateAccount: %w", createErr)
		}
		return acc, nil
	}
	apRaw, _ := json.Marshal(actor)
	_ = r.store.UpdateAccount(ctx, store.UpdateAccountInput{
		ID:          existing.ID,
		DisplayName: strPtr(actor.Name),
		Note:        strPtr(actor.Summary),
		APRaw:       apRaw,
		Bot:         actor.Type == actorTypeService,
		Locked:      actor.ManuallyApprovesFollowers,
	})
	if actor.PublicKey.PublicKeyPem != "" && actor.PublicKey.PublicKeyPem != existing.PublicKey {
		_ = r.store.UpdateAccountKeys(ctx, existing.ID, actor.PublicKey.PublicKeyPem, apRaw)
	}
	acc, getErr := r.store.GetAccountByAPID(ctx, actor.ID)
	if getErr != nil {
		return nil, fmt.Errorf("syncActorToStore GetAccountByAPID: %w", getErr)
	}
	return acc, nil
}
