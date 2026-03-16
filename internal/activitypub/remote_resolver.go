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

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/ssrf"
	"github.com/microcosm-cc/bluemonday"
)

var ErrWebFingerRequestFailed = errors.New("webfinger request failed")

const (
	// staleRemoteActorDuration is the duration after which a remote actor stored in the system is
	// considered stale and should be refreshed.
	staleRemoteActorDuration = 1 * time.Hour
)

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
//
// TODO: this service makes a variety of outbound HTTP requests to externally sourced IRI endpoints.
// There's probably some SSRF risks here. Consider using something secure_egress_http_client.go.
type RemoteAccountResolver struct {
	accounts       service.AccountService
	instanceDomain string
	httpClient     *http.Client
}

// NewRemoteAccountResolver returns a resolver that uses the given account service and actor fetch.
// instanceDomain is used to skip resolution for local accounts (e.g. "example.com").
func NewRemoteAccountResolver(accounts service.AccountService, cfg *config.Config) *RemoteAccountResolver {
	var client *http.Client
	if cfg.AppEnv != "production" && cfg.FederationInsecureSkipTLS {
		client = &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // G402: intentional for development federation with self-signed certs
			},
		}
	} else {
		client = ssrf.NewHTTPClient(ssrf.HTTPClientOptions{})
	}

	return &RemoteAccountResolver{
		accounts:       accounts,
		instanceDomain: strings.TrimSpace(strings.ToLower(cfg.InstanceDomain)),
		httpClient:     client,
	}
}

// ResolveRemoteAccount resolves the given handle (user@domain) to a domain.Account.
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
	if err == nil && !r.isRemoteActorAccountStale(existing) {
		return existing, nil
	}
	// TODO: there is an edge case here where one of the following operations could fail
	// but we have a (stale) account in the database we could return instead.
	// Same situation with ResolveRemoteAccountByIRI.
	actorIRI, err := r.fetchWebFingerActorIRI(ctx, acct)
	if err != nil {
		return nil, fmt.Errorf("ResolveRemoteAccount webfinger: %w", err)
	}
	actor, err := r.fetchActor(ctx, actorIRI)
	if err != nil {
		return nil, fmt.Errorf("ResolveRemoteAccount actor fetch: %w", err)
	}
	return r.SyncActorToStore(ctx, actor)
}

// ResolveRemoteAccountByIRI resolves the given actor IRI to a domain.Account.
// If the account already exists and is not stale, returns it.
// Otherwise, fetches the actor document, creates/updates the account, and returns it.
//
// TODO: currently, we aren't fetching additional details from a remote actor's IRI endpoints.
// This means we don't have things like the actor's followers count, following count, or statuses.
// When resolving an actor, we should also fetch these details.
func (r *RemoteAccountResolver) ResolveRemoteAccountByIRI(ctx context.Context, actorIRI string) (*domain.Account, error) {
	acc, err := r.accounts.GetByAPID(ctx, actorIRI)
	if err == nil && !r.isRemoteActorAccountStale(acc) {
		return acc, nil
	}
	actor, err := r.fetchActor(ctx, actorIRI)
	if err != nil {
		return nil, fmt.Errorf("ResolveRemoteAccountByIRI actor fetch: %w", err)
	}
	return r.SyncActorToStore(ctx, actor)
}

// fetchActor fetches an Actor document from the given IRI using HTTP GET.
func (r *RemoteAccountResolver) fetchActor(ctx context.Context, actorIRI string) (*vocab.Actor, error) {
	var actor vocab.Actor
	if err := r.resolveIRIDocument(ctx, actorIRI, &actor); err != nil {
		return nil, fmt.Errorf("actor fetch: %w", err)
	}
	return &actor, nil
}

func (r *RemoteAccountResolver) fetchWebFingerActorIRI(ctx context.Context, acct string) (string, error) {
	parts := strings.SplitN(acct, "@", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("%w: invalid acct", ErrWebFingerRequestFailed)
	}
	resource := "acct:" + acct
	u := "https://" + parts[1] + "/.well-known/webfinger?resource=" + resource
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("%w: webfinger request: %w", ErrWebFingerRequestFailed, err)
	}
	req.Header.Set("Accept", "application/jrd+json, application/json")
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: webfinger do: %w", ErrWebFingerRequestFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: webfinger returned %d", ErrWebFingerRequestFailed, resp.StatusCode)
	}
	var jrd JRD
	if err := json.NewDecoder(resp.Body).Decode(&jrd); err != nil {
		return "", fmt.Errorf("%w: webfinger decode: %w", ErrWebFingerRequestFailed, err)
	}
	for _, link := range jrd.Links {
		if link.Rel == "self" && (link.Type == "application/activity+json" || link.Type == "") && link.Href != "" {
			return link.Href, nil
		}
	}
	return "", fmt.Errorf("%w: no self link with application/activity+json", ErrWebFingerRequestFailed)
}

// SyncActorToStore creates or updates a domain.Account from an Actor document.
func (r *RemoteAccountResolver) SyncActorToStore(ctx context.Context, actor *vocab.Actor) (*domain.Account, error) {
	// It is generally expected that Mastodon compatible clients will always set the preferredUsername,
	// however it is not required by the ActivityPub spec.
	// For now, we will throw an error if the username is missing.
	// In the future, we may want to add fallback logic for broader interoperability.
	username := actor.PreferredUsername
	if username == "" {
		return nil, errors.New("SyncActorToStore: username is missing")
	}

	// Sanitize username strictly
	// Sanitize display name and note using UGC policy to retain formatting elements.
	username = bluemonday.StrictPolicy().Sanitize(username)
	displayName := bluemonday.UGCPolicy().Sanitize(actor.Name)
	note := bluemonday.UGCPolicy().Sanitize(actor.Summary)

	apRaw, _ := json.Marshal(actor)
	sanitized := *actor
	sanitized.PreferredUsername = username
	sanitized.Name = displayName
	sanitized.Summary = note
	in := vocab.ActorToServiceInput(&sanitized, apRaw)
	acc, err := r.accounts.CreateOrUpdateRemoteAccount(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("SyncActorToStore: %w", err)
	}
	return acc, nil
}

func (r *RemoteAccountResolver) isRemoteActorAccountStale(acc *domain.Account) bool {
	return acc.UpdatedAt.Before(time.Now().Add(-staleRemoteActorDuration))
}

func (r *RemoteAccountResolver) resolveIRIDocument(ctx context.Context, iri string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, iri, nil)
	if err != nil {
		return fmt.Errorf("resolveIRIDocument new request: %w", err)
	}
	req.Header.Set("Accept", "application/activity+json")
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("resolveIRIDocument request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resolveIRIDocument: status %d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(out)
	if err != nil {
		return fmt.Errorf("resolveIRIDocument decode: %w", err)
	}
	return nil
}
