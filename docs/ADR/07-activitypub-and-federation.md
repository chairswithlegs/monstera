# ADR 07 — ActivityPub & Federation

> **Status:** Draft v0.1 — Feb 24, 2026  
> **Addresses:** `design-prompts/07-activitypub-and-federation.md`

---

## Design Decisions (answered before authoring)

| Question | Decision |
|----------|----------|
| Polymorphic `object` field | **`json.RawMessage`** with typed accessor methods — avoids custom `UnmarshalJSON` complexity; callers decode on demand |
| `@context` value | **Array literal** — `["https://www.w3.org/ns/activitystreams", "https://w3id.org/security/v1", mastodonExtensions]` |
| Mastodon extensions in context | Phase 1: `sensitive`, `manuallyApprovesFollowers`, `Hashtag`, `toot:Emoji`. Deferred: `movedTo` (Phase 2 account migration), `featured`, `featuredTags` |
| `to`/`cc` addressing | `[]string` with constant `PublicAddress = "https://www.w3.org/ns/activitystreams#Public"` |
| Content negotiation on `GET /users/:username` | **AP JSON for all requests** — Monstera-fed has no HTML profile (users bring their own client). `Content-Type: application/activity+json` always. |
| Inbox processing mode | **Synchronous** on the HTTP handler goroutine for Phase 1. Async goroutine pool is a Phase 2 enhancement. |
| Remote media on `Create{Note}` | **Store `remote_url` only** — no fetch in Phase 1. Lazy-fetch on first access is Phase 2. |
| Idempotency | `INSERT … ON CONFLICT (ap_id) DO NOTHING` — duplicate activities silently ignored |
| `Undo{Like}` activity ID tracking | **`ap_id` column on `favourites`** — consistent with `follows.ap_id` and `statuses.ap_id`; enables exact match on incoming `Undo{Like}` by activity ID, not just `(actor, object)` heuristic |
| `Undo{Announce}` tracking | Boosts are stored as statuses with `ap_id` — already covered |
| Shared inbox delivery | **Deduplicate by inbox URL** — one delivery per unique shared inbox, not per follower |
| `featured` collection | **Empty stub** in Phase 1 — prevents 404 from remote instances fetching pinned posts |
| Federation worker consumer type | **Pull consumer** — backpressure-friendly; configurable concurrency |
| NATS delivery subject | `federation.deliver.{activityType}` — e.g. `federation.deliver.create`, `federation.deliver.follow` |

---

## 1. `internal/ap/vocab.go` — ActivityStreams / AP Vocabulary Types

### Constants

```go
package ap

import "encoding/json"

// PublicAddress is the ActivityStreams public addressing constant.
// Activities addressed to this IRI are visible to anyone.
const PublicAddress = "https://www.w3.org/ns/activitystreams#Public"

// mastodonExtContext is the Mastodon-specific extension context map included
// in outgoing activities. It maps short names to their full IRIs so that
// fields like "sensitive" and "manuallyApprovesFollowers" are understood
// by remote servers.
var mastodonExtContext = map[string]any{
    "manuallyApprovesFollowers": "as:manuallyApprovesFollowers",
    "sensitive":                 "as:sensitive",
    "Hashtag":                   "as:Hashtag",
    "toot":                      "http://joinmastodon.org/ns#",
    "Emoji":                     "toot:Emoji",
    "featured": map[string]string{
        "@id":   "toot:featured",
        "@type": "@id",
    },
    "featuredTags": map[string]string{
        "@id":   "toot:featuredTags",
        "@type": "@id",
    },
}

// DefaultContext is the canonical @context value for all outgoing AP activities.
// Structured as a JSON array: [AS2 namespace, Security namespace, Mastodon extensions].
var DefaultContext = []any{
    "https://www.w3.org/ns/activitystreams",
    "https://w3id.org/security/v1",
    mastodonExtContext,
}
```

### Base Types

```go
// Object is the base ActivityStreams object. All AP types embed it.
// Fields shared across Actor, Note, Activity, and Collection types.
type Object struct {
    Context interface{} `json:"@context,omitempty"`
    ID      string      `json:"id"`
    Type    string      `json:"type"`
}

// PublicKey is the RSA public key embedded in an Actor document.
// Used by remote servers to verify HTTP Signatures on outgoing deliveries.
type PublicKey struct {
    ID           string `json:"id"`
    Owner        string `json:"owner"`
    PublicKeyPem string `json:"publicKeyPem"`
}

// Endpoints holds the Actor's special endpoint URLs.
// Only sharedInbox is used in Phase 1.
type Endpoints struct {
    SharedInbox string `json:"sharedInbox,omitempty"`
}

// Icon represents an Actor's avatar or header image.
type Icon struct {
    Type      string `json:"type"` // "Image"
    MediaType string `json:"mediaType,omitempty"`
    URL       string `json:"url"`
}

// Tag represents a Hashtag or Mention tag embedded in a Note.
type Tag struct {
    Type string `json:"type"` // "Hashtag" | "Mention"
    Href string `json:"href,omitempty"`
    Name string `json:"name"` // "#golang" for hashtags, "@user@domain" for mentions
}

// Attachment represents a media attachment on a Note.
type Attachment struct {
    Type      string `json:"type"` // "Document"
    MediaType string `json:"mediaType,omitempty"`
    URL       string `json:"url"`
    Name      string `json:"name,omitempty"` // alt text
    Blurhash  string `json:"blurhash,omitempty"`
    Width     int    `json:"width,omitempty"`
    Height    int    `json:"height,omitempty"`
}
```

### Actor

```go
// Actor represents an AP Person (user account). Served at GET /users/:username.
//
// Fields follow Mastodon's Actor shape so that remote instances recognise
// all profile metadata. Fields not relevant to Phase 1 (e.g. movedTo) are
// omitted and added as their features are implemented.
type Actor struct {
    Context                   interface{} `json:"@context"`
    ID                        string      `json:"id"`
    Type                      string      `json:"type"` // "Person" | "Service" (for bot accounts)
    PreferredUsername          string      `json:"preferredUsername"`
    Name                      string      `json:"name,omitempty"`
    Summary                   string      `json:"summary,omitempty"` // bio HTML
    URL                       string      `json:"url"`
    Inbox                     string      `json:"inbox"`
    Outbox                    string      `json:"outbox"`
    Followers                 string      `json:"followers"`
    Following                 string      `json:"following"`
    Featured                  string      `json:"featured,omitempty"`
    PublicKey                 PublicKey    `json:"publicKey"`
    Endpoints                 *Endpoints  `json:"endpoints,omitempty"`
    Icon                      *Icon       `json:"icon,omitempty"`
    Image                     *Icon       `json:"image,omitempty"` // header image
    ManuallyApprovesFollowers bool        `json:"manuallyApprovesFollowers"`
    Published                 string      `json:"published,omitempty"` // ISO 8601
}
```

### Note

```go
// Note represents an AP Note (status/post). The core content type in the
// Mastodon federation protocol.
//
// ContentMap is a Mastodon extension that maps language codes to localised
// content. Phase 1 populates it with a single entry when language is known.
type Note struct {
    Context      interface{}  `json:"@context,omitempty"`
    ID           string       `json:"id"`
    Type         string       `json:"type"` // "Note"
    AttributedTo string       `json:"attributedTo"`
    Content      string       `json:"content"` // rendered HTML
    ContentMap   map[string]string `json:"contentMap,omitempty"`
    Source       *NoteSource  `json:"source,omitempty"`
    To           []string     `json:"to"`
    Cc           []string     `json:"cc,omitempty"`
    InReplyTo    *string      `json:"inReplyTo"` // null or parent Note IRI
    Published    string       `json:"published"` // ISO 8601
    Updated      string       `json:"updated,omitempty"`
    URL          string       `json:"url"`
    Sensitive    bool         `json:"sensitive"`
    Summary      *string      `json:"summary"` // content warning; null if none
    Tag          []Tag        `json:"tag,omitempty"`
    Attachment   []Attachment `json:"attachment,omitempty"`
    Replies      *OrderedCollection `json:"replies,omitempty"`
}

// NoteSource preserves the original plain-text or Markdown source.
// Mastodon includes this for editable posts.
type NoteSource struct {
    Content   string `json:"content"`
    MediaType string `json:"mediaType"` // "text/plain"
}
```

### Activity

```go
// Activity is the generic AP Activity wrapper. Used for Follow, Like, Announce,
// Create, Delete, Update, Undo, Accept, Reject, and Block.
//
// The ObjectRaw field holds the polymorphic "object" — it can be a string
// (IRI reference) or an embedded JSON object (e.g. a full Note). Callers
// use the accessor methods to decode it.
type Activity struct {
    Context   interface{}     `json:"@context,omitempty"`
    ID        string          `json:"id"`
    Type      string          `json:"type"`
    Actor     string          `json:"actor"`
    ObjectRaw json.RawMessage `json:"object"`
    To        []string        `json:"to,omitempty"`
    Cc        []string        `json:"cc,omitempty"`
    Published string          `json:"published,omitempty"`
}

// ObjectID returns the object field as a plain IRI string.
// Returns ("", false) if the object is an embedded JSON object rather than a string.
func (a *Activity) ObjectID() (string, bool) {
    var id string
    if err := json.Unmarshal(a.ObjectRaw, &id); err != nil {
        return "", false
    }
    return id, true
}

// ObjectActivity unmarshals the object field as an embedded Activity.
// Used for Undo{Follow}, Accept{Follow}, Reject{Follow} where the object
// is the original Follow activity.
func (a *Activity) ObjectActivity() (*Activity, error) {
    var inner Activity
    if err := json.Unmarshal(a.ObjectRaw, &inner); err != nil {
        return nil, err
    }
    return &inner, nil
}

// ObjectNote unmarshals the object field as a Note.
// Used for Create{Note} and Update{Note}.
func (a *Activity) ObjectNote() (*Note, error) {
    var note Note
    if err := json.Unmarshal(a.ObjectRaw, &note); err != nil {
        return nil, err
    }
    return &note, nil
}

// ObjectActor unmarshals the object field as an Actor.
// Used for Update{Person}.
func (a *Activity) ObjectActor() (*Actor, error) {
    var actor Actor
    if err := json.Unmarshal(a.ObjectRaw, &actor); err != nil {
        return nil, err
    }
    return &actor, nil
}

// ObjectType peeks at the "type" field of the embedded object without fully
// unmarshalling it. Returns "" if the object is a plain string IRI.
func (a *Activity) ObjectType() string {
    var peek struct {
        Type string `json:"type"`
    }
    if err := json.Unmarshal(a.ObjectRaw, &peek); err != nil {
        return ""
    }
    return peek.Type
}
```

### Collections

```go
// OrderedCollection represents an AP OrderedCollection.
// Used for outbox, followers, and following endpoints.
type OrderedCollection struct {
    Context    interface{} `json:"@context,omitempty"`
    ID         string      `json:"id"`
    Type       string      `json:"type"` // "OrderedCollection"
    TotalItems int         `json:"totalItems"`
    First      string      `json:"first,omitempty"` // URL of first page
}

// OrderedCollectionPage represents a page within an OrderedCollection.
type OrderedCollectionPage struct {
    Context      interface{}     `json:"@context,omitempty"`
    ID           string          `json:"id"`
    Type         string          `json:"type"` // "OrderedCollectionPage"
    TotalItems   int             `json:"totalItems"`
    PartOf       string          `json:"partOf"`
    Next         string          `json:"next,omitempty"`
    Prev         string          `json:"prev,omitempty"`
    OrderedItems []json.RawMessage `json:"orderedItems"`
}
```

### Helper Functions

```go
// DomainFromActorID extracts the domain (host) from an AP actor IRI.
// Used for domain block checks and account domain population.
//
// Example: "https://remote.example.com/users/alice" → "remote.example.com"
func DomainFromActorID(actorID string) string {
    u, err := url.Parse(actorID)
    if err != nil {
        return ""
    }
    return u.Host
}

// DomainFromKeyID extracts the domain from an HTTP Signature key ID.
// Key IDs are typically "https://remote.example.com/users/alice#main-key".
func DomainFromKeyID(keyID string) string {
    return DomainFromActorID(keyID)
}

// WrapInCreate wraps a Note in a Create activity with the given activity ID.
func WrapInCreate(activityID string, note *Note) *Activity {
    raw, _ := json.Marshal(note)
    return &Activity{
        Context:   DefaultContext,
        ID:        activityID,
        Type:      "Create",
        Actor:     note.AttributedTo,
        ObjectRaw: raw,
        To:        note.To,
        Cc:        note.Cc,
        Published: note.Published,
    }
}

// NewFollowActivity constructs a Follow activity.
func NewFollowActivity(activityID, actorID, targetID string) *Activity {
    raw, _ := json.Marshal(targetID)
    return &Activity{
        Context:   DefaultContext,
        ID:        activityID,
        Type:      "Follow",
        Actor:     actorID,
        ObjectRaw: raw,
    }
}

// NewAcceptActivity wraps an inner activity (typically Follow) in an Accept.
func NewAcceptActivity(activityID, actorID string, inner *Activity) *Activity {
    raw, _ := json.Marshal(inner)
    return &Activity{
        Context:   DefaultContext,
        ID:        activityID,
        Type:      "Accept",
        Actor:     actorID,
        ObjectRaw: raw,
    }
}

// NewRejectActivity wraps an inner activity in a Reject.
func NewRejectActivity(activityID, actorID string, inner *Activity) *Activity {
    raw, _ := json.Marshal(inner)
    return &Activity{
        Context:   DefaultContext,
        ID:        activityID,
        Type:      "Reject",
        Actor:     actorID,
        ObjectRaw: raw,
    }
}

// NewUndoActivity wraps an inner activity in an Undo.
func NewUndoActivity(activityID, actorID string, inner *Activity) *Activity {
    raw, _ := json.Marshal(inner)
    return &Activity{
        Context:   DefaultContext,
        ID:        activityID,
        Type:      "Undo",
        Actor:     actorID,
        ObjectRaw: raw,
    }
}

// NewLikeActivity constructs a Like activity.
func NewLikeActivity(activityID, actorID, objectID string) *Activity {
    raw, _ := json.Marshal(objectID)
    return &Activity{
        Context:   DefaultContext,
        ID:        activityID,
        Type:      "Like",
        Actor:     actorID,
        ObjectRaw: raw,
    }
}

// NewAnnounceActivity constructs an Announce (boost) activity.
func NewAnnounceActivity(activityID, actorID, objectID string, to, cc []string, published string) *Activity {
    raw, _ := json.Marshal(objectID)
    return &Activity{
        Context:   DefaultContext,
        ID:        activityID,
        Type:      "Announce",
        Actor:     actorID,
        ObjectRaw: raw,
        To:        to,
        Cc:        cc,
        Published: published,
    }
}

// NewDeleteActivity constructs a Delete activity with a Tombstone object.
func NewDeleteActivity(activityID, actorID, objectID string) *Activity {
    tombstone := map[string]string{
        "id":   objectID,
        "type": "Tombstone",
    }
    raw, _ := json.Marshal(tombstone)
    return &Activity{
        Context:   DefaultContext,
        ID:        activityID,
        Type:      "Delete",
        Actor:     actorID,
        ObjectRaw: raw,
    }
}

// NewBlockActivity constructs a Block activity.
func NewBlockActivity(activityID, actorID, targetID string) *Activity {
    raw, _ := json.Marshal(targetID)
    return &Activity{
        Context:   DefaultContext,
        ID:        activityID,
        Type:      "Block",
        Actor:     actorID,
        ObjectRaw: raw,
    }
}
```

### Activity ID Generation

```go
// ActivityID generates a canonical activity IRI for locally-created activities.
// Format: https://{instanceDomain}/activities/{ulid}
//
// The ULID provides time-sortability and uniqueness. The full IRI is globally
// unique and resolvable (though Monstera-fed does not serve GETs on activity IRIs
// in Phase 1 — remote servers use them as opaque identifiers).
//
// Used for all outgoing activities: Create, Delete, Follow, Undo, Accept,
// Reject, Like, Announce, Block. The generated ID is stored in the relevant
// domain table's ap_id column (follows.ap_id, favourites.ap_id, statuses.ap_id)
// so that Undo activities can reference the original.
func ActivityID(instanceDomain string) string {
    return "https://" + instanceDomain + "/activities/" + uid.New()
}
```

---

## 1a. Schema Addendum — `favourites.ap_id`

The `favourites` table (ADR 02, migration 000020) needs an `ap_id` column for federation parity with `follows.ap_id` and `statuses.ap_id`.

### Migration: `000024_add_ap_id_to_favourites.up.sql`

```sql
ALTER TABLE favourites
    ADD COLUMN ap_id TEXT UNIQUE;
```

### Migration: `000024_add_ap_id_to_favourites.down.sql`

```sql
ALTER TABLE favourites
    DROP COLUMN IF EXISTS ap_id;
```

### Updated `sqlc` Queries: `favourites.sql`

```sql
-- name: CreateFavourite :one
-- (replaces the original from ADR 02)
INSERT INTO favourites (id, account_id, status_id, ap_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetFavouriteByAPID :one
-- Used by incoming Undo{Like} when the remote server references the Like by ID.
SELECT * FROM favourites WHERE ap_id = $1;

-- name: GetFavouriteByAccountAndStatus :one
-- Fallback for incoming Undo{Like} when the remote server embeds actor + object
-- instead of referencing the Like activity ID directly.
SELECT * FROM favourites WHERE account_id = $1 AND status_id = $2;
```

### Store Interface Addition

```go
// Added to FavouriteStore in internal/store/store.go:

type FavouriteStore interface {
    // ... existing methods ...
    GetFavouriteByAPID(ctx context.Context, apID string) (Favourite, error)
    GetFavouriteByAccountAndStatus(ctx context.Context, accountID, statusID string) (Favourite, error)
}
```

**Rationale:** Incoming `Undo{Like}` activities are handled with a two-step lookup:
1. Try `GetFavouriteByAPID` using the inner Like activity's ID — exact match.
2. Fall back to `GetFavouriteByAccountAndStatus` by resolving the actor → account and object → status — covers implementations that don't preserve the original Like ID in their Undo payload.

This matches how `follows` are handled: `GetFollowByAPID` for exact match, `GetFollow(accountID, targetID)` as fallback.

---

## 2. `internal/ap/blocklist.go` — Domain Block Cache

The blocklist is checked on every inbound activity and before every outbound delivery. It must be fast (no DB round-trip per request) and consistent across replicas.

### Design

```go
package ap

import (
    "context"
    "encoding/json"
    "log/slog"
    "sync"
    "time"

    "github.com/yourorg/monstera-fed/internal/cache"
    "github.com/yourorg/monstera-fed/internal/store"
)

// blocklistCacheKey is the single cache key holding the serialized domain block map.
const blocklistCacheKey = "domain_blocks"

// blocklistCacheTTL is how long the cached block list lives before requiring
// a refresh from the database. The TTL is intentionally long because domain
// blocks change infrequently (admin action only) and the cache is explicitly
// invalidated on writes via Refresh().
const blocklistCacheTTL = 1 * time.Hour

// BlocklistCache provides fast domain block lookups backed by the cache layer.
//
// Architecture:
//   - On startup (and periodically on cache miss), all rows from domain_blocks
//     are loaded into a single cache entry keyed "domain_blocks".
//   - The cached value is a JSON-encoded map[string]string (domain → severity).
//   - Individual lookups deserialize from the cached map. The map is typically
//     small (tens to low hundreds of domains), so deserialization is cheap.
//   - When an admin adds or removes a domain block, the service layer calls
//     Refresh() to invalidate and reload the cache.
//
// Multi-replica note: with CACHE_DRIVER=redis, Refresh() on one replica
// updates the shared cache entry visible to all replicas. With
// CACHE_DRIVER=memory, each replica maintains an independent copy — a block
// added on one replica takes up to blocklistCacheTTL to propagate to others.
// This is an accepted limitation of the memory driver (documented as dev-only).
type BlocklistCache struct {
    store  store.DomainBlockStore
    cache  cache.Store
    logger *slog.Logger
    mu     sync.Mutex // serialises Refresh to avoid thundering herd on cold start
}

// NewBlocklistCache constructs a BlocklistCache. Callers should invoke
// Refresh() once during startup to warm the cache.
func NewBlocklistCache(s store.DomainBlockStore, c cache.Store, logger *slog.Logger) *BlocklistCache {
    return &BlocklistCache{store: s, cache: c, logger: logger}
}

// IsBlocked returns true if the domain is blocked at any severity level
// (silence or suspend). Used by both the inbox handler and the federation
// worker to short-circuit processing/delivery.
func (b *BlocklistCache) IsBlocked(ctx context.Context, domain string) (bool, error) {
    severity, err := b.Severity(ctx, domain)
    if err != nil {
        return false, err
    }
    return severity != "", nil
}

// IsSuspended returns true if the domain is blocked at the "suspend" level.
// Suspended domains have all activities rejected and no content is delivered.
func (b *BlocklistCache) IsSuspended(ctx context.Context, domain string) (bool, error) {
    severity, err := b.Severity(ctx, domain)
    if err != nil {
        return false, err
    }
    return severity == "suspend", nil
}

// IsSilenced returns true if the domain is blocked at the "silence" level.
// Silenced domains have content hidden from public timelines but follows
// still work.
func (b *BlocklistCache) IsSilenced(ctx context.Context, domain string) (bool, error) {
    severity, err := b.Severity(ctx, domain)
    if err != nil {
        return false, err
    }
    return severity == "silence", nil
}

// Severity returns the block severity for the domain: "silence", "suspend",
// or "" (empty string) if the domain is not blocked.
func (b *BlocklistCache) Severity(ctx context.Context, domain string) (string, error) {
    blocks, err := b.loadCached(ctx)
    if err != nil {
        return "", err
    }
    return blocks[domain], nil
}

// Refresh reloads all domain blocks from the database into the cache.
// Called on startup and after admin adds/removes a domain block.
//
// The mutex prevents multiple concurrent refreshes (e.g. if two admin
// requests arrive simultaneously, or startup races with the first inbound
// activity).
func (b *BlocklistCache) Refresh(ctx context.Context) error {
    b.mu.Lock()
    defer b.mu.Unlock()

    blocks, err := b.store.ListDomainBlocks(ctx)
    if err != nil {
        return fmt.Errorf("blocklist: load from DB: %w", err)
    }

    m := make(map[string]string, len(blocks))
    for _, block := range blocks {
        m[block.Domain] = block.Severity
    }

    if err := cache.SetJSON(ctx, b.cache, blocklistCacheKey, m, blocklistCacheTTL); err != nil {
        b.logger.Warn("blocklist: failed to write cache", "error", err)
    }

    b.logger.Info("blocklist refreshed", "count", len(m))
    return nil
}

// loadCached returns the domain→severity map from cache, falling back to
// a full DB reload on cache miss.
func (b *BlocklistCache) loadCached(ctx context.Context) (map[string]string, error) {
    var m map[string]string
    if hit, _ := cache.GetJSON(ctx, b.cache, blocklistCacheKey, &m); hit {
        return m, nil
    }
    if err := b.Refresh(ctx); err != nil {
        return nil, err
    }
    // After Refresh, the cache is populated — read it back.
    if hit, _ := cache.GetJSON(ctx, b.cache, blocklistCacheKey, &m); hit {
        return m, nil
    }
    return map[string]string{}, nil
}
```

---

## 3. `internal/ap/inbox.go` — Inbox Processor

### Types and Constructor

```go
package ap

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log/slog"
    "net/url"
    "strings"

    "github.com/yourorg/monstera-fed/internal/cache"
    "github.com/yourorg/monstera-fed/internal/config"
    "github.com/yourorg/monstera-fed/internal/store"
    db "github.com/yourorg/monstera-fed/internal/store/postgres/generated"
    "github.com/yourorg/monstera-fed/internal/uid"
)

// PermanentError wraps an error that should not be retried. The inbox handler
// logs it at warn level and returns 202 (accepted but dropped).
type PermanentError struct {
    Err error
}

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

// EventPublisher publishes SSE events to connected clients via NATS.
// Defined as an interface so the inbox processor doesn't depend on the
// NATS streaming package directly.
type EventPublisher interface {
    PublishStatusEvent(ctx context.Context, accountID, eventType string, payload json.RawMessage) error
    PublishNotificationEvent(ctx context.Context, accountID string, payload json.RawMessage) error
}

// InboxProcessor dispatches verified incoming AP activities to type-specific
// handlers. It is the core of Monstera-fed's federation inbox.
//
// All methods are safe for concurrent use. The processor is constructed once
// at startup and shared across all inbox HTTP handler goroutines.
//
// Error semantics:
//   - nil: activity processed successfully.
//   - *PermanentError: activity is malformed or unsupported; do not retry.
//     The HTTP handler returns 202 Accepted (the AP spec does not distinguish
//     success from "accepted but ignored").
//   - any other error: transient failure (DB, cache). The HTTP handler may
//     return 500, signalling the sender to retry.
type InboxProcessor struct {
    store      store.Store
    cache      cache.Store
    blocklist  *BlocklistCache
    events     EventPublisher
    cfg        *config.Config
    logger     *slog.Logger
}

// NewInboxProcessor constructs an InboxProcessor.
func NewInboxProcessor(
    s store.Store,
    c cache.Store,
    bl *BlocklistCache,
    events EventPublisher,
    cfg *config.Config,
    logger *slog.Logger,
) *InboxProcessor {
    return &InboxProcessor{
        store:     s,
        cache:     c,
        blocklist: bl,
        events:    events,
        cfg:       cfg,
        logger:    logger,
    }
}
```

### Dispatch

```go
// Process is the main entry point. It dispatches a verified incoming activity
// to the appropriate handler based on the activity type.
//
// Pre-conditions (enforced by the HTTP handler before calling Process):
//   - The HTTP Signature has been verified (the sender is authenticated).
//   - The request body has been parsed into an Activity struct.
//
// Domain blocks are checked here before dispatching — blocked domains are
// silently dropped with a debug-level log.
func (p *InboxProcessor) Process(ctx context.Context, activity *Activity) error {
    actorDomain := DomainFromActorID(activity.Actor)
    if actorDomain == "" {
        return &PermanentError{Err: fmt.Errorf("cannot extract domain from actor %q", activity.Actor)}
    }

    // Domain block check — suspended domains are completely rejected.
    suspended, err := p.blocklist.IsSuspended(ctx, actorDomain)
    if err != nil {
        return fmt.Errorf("inbox: blocklist check: %w", err)
    }
    if suspended {
        p.logger.Debug("inbox: dropped activity from suspended domain",
            "domain", actorDomain, "type", activity.Type, "id", activity.ID)
        return nil
    }

    switch activity.Type {
    case "Follow":
        return p.handleFollow(ctx, activity)
    case "Accept":
        return p.handleAcceptFollow(ctx, activity)
    case "Reject":
        return p.handleRejectFollow(ctx, activity)
    case "Undo":
        return p.handleUndo(ctx, activity)
    case "Create":
        return p.handleCreate(ctx, activity, actorDomain)
    case "Announce":
        return p.handleAnnounce(ctx, activity, actorDomain)
    case "Like":
        return p.handleLike(ctx, activity)
    case "Delete":
        return p.handleDelete(ctx, activity)
    case "Update":
        return p.handleUpdate(ctx, activity)
    case "Block":
        return p.handleBlock(ctx, activity)
    default:
        p.logger.Debug("inbox: unsupported activity type",
            "type", activity.Type, "id", activity.ID)
        return nil
    }
}
```

### Activity Handlers

#### Follow

```go
// handleFollow processes an incoming Follow activity.
//
// Flow:
//  1. Resolve the target (the followed local account) from the object field.
//  2. Resolve or create the remote actor account.
//  3. Check idempotency: if a follow with this ap_id already exists, ignore.
//  4. Create the follow record. State depends on target.locked:
//     - locked=false → state="accepted", immediately send Accept{Follow}.
//     - locked=true  → state="pending", create a follow_request notification.
//  5. Create a "follow" notification for the target user.
func (p *InboxProcessor) handleFollow(ctx context.Context, activity *Activity) error {
    targetID, ok := activity.ObjectID()
    if !ok {
        return &PermanentError{Err: fmt.Errorf("Follow object is not an actor IRI")}
    }

    targetUsername := usernameFromActorIRI(targetID, p.cfg.InstanceDomain)
    if targetUsername == "" {
        return &PermanentError{Err: fmt.Errorf("Follow target %q is not a local user", targetID)}
    }

    target, err := p.store.GetLocalAccountByUsername(ctx, targetUsername)
    if err != nil {
        return &PermanentError{Err: fmt.Errorf("Follow target not found: %s", targetUsername)}
    }

    actor, err := p.resolveRemoteAccount(ctx, activity.Actor)
    if err != nil {
        return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
    }

    // Idempotency: check for existing follow with this AP ID.
    if activity.ID != "" {
        if existing, _ := p.store.GetFollowByAPID(ctx, activity.ID); existing.ID != "" {
            p.logger.Debug("inbox: duplicate Follow ignored", "ap_id", activity.ID)
            return nil
        }
    }

    state := "accepted"
    notifType := "follow"
    if target.Locked {
        state = "pending"
        notifType = "follow_request"
    }

    apID := &activity.ID
    follow, err := p.store.CreateFollow(ctx, db.CreateFollowParams{
        ID:        uid.New(),
        AccountID: actor.ID,
        TargetID:  target.ID,
        State:     state,
        ApID:      apID,
    })
    if err != nil {
        if isUniqueViolation(err) {
            return nil // already following — idempotent
        }
        return fmt.Errorf("inbox: create follow: %w", err)
    }

    // Create notification for the target.
    p.createNotification(ctx, target.ID, actor.ID, notifType, nil)

    // If auto-accepted, send Accept{Follow} back to the remote actor.
    if state == "accepted" {
        return p.sendAcceptFollow(ctx, &target, &actor, activity, follow.ID)
    }

    return nil
}
```

#### Accept{Follow} / Reject{Follow}

```go
// handleAcceptFollow processes an incoming Accept{Follow} activity.
// Marks the pending follow as accepted.
func (p *InboxProcessor) handleAcceptFollow(ctx context.Context, activity *Activity) error {
    inner, err := activity.ObjectActivity()
    if err != nil {
        objectID, ok := activity.ObjectID()
        if !ok {
            return &PermanentError{Err: fmt.Errorf("Accept object is not a Follow activity or IRI")}
        }
        // Some implementations send the Follow ID as a plain string.
        follow, err := p.store.GetFollowByAPID(ctx, objectID)
        if err != nil {
            return &PermanentError{Err: fmt.Errorf("Accept: follow not found for ap_id %q", objectID)}
        }
        return p.store.AcceptFollow(ctx, follow.ID)
    }

    // Inner is the original Follow activity — look it up by AP ID.
    if inner.ID != "" {
        follow, err := p.store.GetFollowByAPID(ctx, inner.ID)
        if err == nil {
            return p.store.AcceptFollow(ctx, follow.ID)
        }
    }

    // Fallback: resolve by actor → target relationship.
    actorAccount, err := p.store.GetAccountByAPID(ctx, inner.Actor)
    if err != nil {
        return &PermanentError{Err: fmt.Errorf("Accept: actor not found %q", inner.Actor)}
    }
    targetID, _ := inner.ObjectID()
    targetAccount, err := p.store.GetAccountByAPID(ctx, targetID)
    if err != nil {
        return &PermanentError{Err: fmt.Errorf("Accept: target not found %q", targetID)}
    }
    follow, err := p.store.GetFollow(ctx, actorAccount.ID, targetAccount.ID)
    if err != nil {
        return &PermanentError{Err: fmt.Errorf("Accept: follow relationship not found")}
    }
    return p.store.AcceptFollow(ctx, follow.ID)
}

// handleRejectFollow processes an incoming Reject{Follow} activity.
// Deletes the pending follow.
func (p *InboxProcessor) handleRejectFollow(ctx context.Context, activity *Activity) error {
    inner, err := activity.ObjectActivity()
    if err != nil {
        objectID, ok := activity.ObjectID()
        if !ok {
            return &PermanentError{Err: fmt.Errorf("Reject object is not a Follow activity or IRI")}
        }
        follow, err := p.store.GetFollowByAPID(ctx, objectID)
        if err != nil {
            return nil // already deleted or never existed — idempotent
        }
        return p.store.DeleteFollow(ctx, follow.AccountID, follow.TargetID)
    }

    if inner.ID != "" {
        follow, err := p.store.GetFollowByAPID(ctx, inner.ID)
        if err == nil {
            return p.store.DeleteFollow(ctx, follow.AccountID, follow.TargetID)
        }
    }

    actorAccount, _ := p.store.GetAccountByAPID(ctx, inner.Actor)
    targetID, _ := inner.ObjectID()
    targetAccount, _ := p.store.GetAccountByAPID(ctx, targetID)
    if actorAccount.ID != "" && targetAccount.ID != "" {
        return p.store.DeleteFollow(ctx, actorAccount.ID, targetAccount.ID)
    }
    return nil
}
```

#### Undo (dispatch)

```go
// handleUndo dispatches to the appropriate undo handler based on the inner
// object's type.
func (p *InboxProcessor) handleUndo(ctx context.Context, activity *Activity) error {
    innerType := activity.ObjectType()

    switch innerType {
    case "Follow":
        return p.handleUndoFollow(ctx, activity)
    case "Like":
        return p.handleUndoLike(ctx, activity)
    case "Announce":
        return p.handleUndoAnnounce(ctx, activity)
    default:
        // Some implementations send Undo with the object as a plain IRI.
        // Try to resolve it as a Follow (most common Undo target).
        if objectID, ok := activity.ObjectID(); ok {
            if follow, err := p.store.GetFollowByAPID(ctx, objectID); err == nil {
                return p.store.DeleteFollow(ctx, follow.AccountID, follow.TargetID)
            }
            if fav, err := p.store.GetFavouriteByAPID(ctx, objectID); err == nil {
                return p.undoFavourite(ctx, fav)
            }
        }
        p.logger.Debug("inbox: unsupported Undo object type", "type", innerType, "id", activity.ID)
        return nil
    }
}

// handleUndoFollow removes a follow relationship.
func (p *InboxProcessor) handleUndoFollow(ctx context.Context, activity *Activity) error {
    inner, err := activity.ObjectActivity()
    if err != nil {
        return &PermanentError{Err: fmt.Errorf("Undo{Follow} object is not a Follow activity")}
    }

    // Try exact match by AP ID first.
    if inner.ID != "" {
        follow, err := p.store.GetFollowByAPID(ctx, inner.ID)
        if err == nil {
            return p.store.DeleteFollow(ctx, follow.AccountID, follow.TargetID)
        }
    }

    // Fallback: resolve by actor → target.
    actorAccount, err := p.store.GetAccountByAPID(ctx, inner.Actor)
    if err != nil {
        return nil // actor not known locally — nothing to undo
    }
    targetID, _ := inner.ObjectID()
    targetAccount, err := p.store.GetAccountByAPID(ctx, targetID)
    if err != nil {
        return nil
    }
    return p.store.DeleteFollow(ctx, actorAccount.ID, targetAccount.ID)
}

// handleUndoLike removes a favourite.
func (p *InboxProcessor) handleUndoLike(ctx context.Context, activity *Activity) error {
    inner, err := activity.ObjectActivity()
    if err != nil {
        return &PermanentError{Err: fmt.Errorf("Undo{Like} object is not a Like activity")}
    }

    // Try exact match by AP ID.
    if inner.ID != "" {
        fav, err := p.store.GetFavouriteByAPID(ctx, inner.ID)
        if err == nil {
            return p.undoFavourite(ctx, fav)
        }
    }

    // Fallback: resolve by actor + liked status.
    actorAccount, err := p.store.GetAccountByAPID(ctx, inner.Actor)
    if err != nil {
        return nil
    }
    objectID, _ := inner.ObjectID()
    status, err := p.store.GetStatusByAPID(ctx, objectID)
    if err != nil {
        return nil
    }
    fav, err := p.store.GetFavouriteByAccountAndStatus(ctx, actorAccount.ID, status.ID)
    if err != nil {
        return nil
    }
    return p.undoFavourite(ctx, fav)
}

// undoFavourite deletes a favourite and decrements the status counter.
func (p *InboxProcessor) undoFavourite(ctx context.Context, fav store.Favourite) error {
    if err := p.store.DeleteFavourite(ctx, fav.AccountID, fav.StatusID); err != nil {
        return err
    }
    return p.store.DecrementFavouritesCount(ctx, fav.StatusID)
}

// handleUndoAnnounce removes a boost.
func (p *InboxProcessor) handleUndoAnnounce(ctx context.Context, activity *Activity) error {
    inner, err := activity.ObjectActivity()
    if err != nil {
        return &PermanentError{Err: fmt.Errorf("Undo{Announce} object is not an Announce activity")}
    }

    // Look up the boost status by AP ID.
    if inner.ID != "" {
        boost, err := p.store.GetStatusByAPID(ctx, inner.ID)
        if err == nil && boost.ReblogOfID != nil {
            if err := p.store.SoftDeleteStatus(ctx, boost.ID); err != nil {
                return err
            }
            return p.store.DecrementReblogsCount(ctx, *boost.ReblogOfID)
        }
    }

    // Fallback: find the boost by account + reblogged status.
    actorAccount, err := p.store.GetAccountByAPID(ctx, inner.Actor)
    if err != nil {
        return nil
    }
    objectID, _ := inner.ObjectID()
    originalStatus, err := p.store.GetStatusByAPID(ctx, objectID)
    if err != nil {
        return nil
    }
    boost, err := p.store.GetReblogByAccountAndTarget(ctx, actorAccount.ID, originalStatus.ID)
    if err != nil {
        return nil
    }
    if err := p.store.SoftDeleteStatus(ctx, boost.ID); err != nil {
        return err
    }
    return p.store.DecrementReblogsCount(ctx, originalStatus.ID)
}
```

#### Create{Note}

```go
// handleCreate processes an incoming Create{Note} activity.
//
// Flow:
//  1. Unmarshal the inner Note.
//  2. Check idempotency: if a status with this ap_id exists, ignore.
//  3. Resolve the remote author account.
//  4. Check visibility and authorization:
//     - Public/unlisted: always accept.
//     - Followers-only: accept only if the sender has an accepted follow
//       relationship with at least one local user.
//     - Direct: accept only if at least one local user is in the "to" list.
//  5. Check silenced domain: if the sender's domain is silenced, mark the
//     status but still ingest it (it's hidden from public timelines later).
//  6. Extract mentions from tags — create notifications for mentioned local users.
//  7. Store remote media references (remote_url only, no fetch).
//  8. Insert the status.
//  9. If it's a reply to a local status, increment the parent's replies_count.
func (p *InboxProcessor) handleCreate(ctx context.Context, activity *Activity, actorDomain string) error {
    note, err := activity.ObjectNote()
    if err != nil {
        return &PermanentError{Err: fmt.Errorf("Create object is not a Note: %w", err)}
    }
    if note.Type != "Note" {
        return &PermanentError{Err: fmt.Errorf("Create object type %q is not supported", note.Type)}
    }

    // Idempotency.
    if note.ID != "" {
        if _, err := p.store.GetStatusByAPID(ctx, note.ID); err == nil {
            return nil
        }
    }

    author, err := p.resolveRemoteAccount(ctx, activity.Actor)
    if err != nil {
        return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
    }

    visibility := p.resolveVisibility(note, author)

    // Authorization check for non-public visibilities.
    if visibility == "private" {
        hasLocalFollower, err := p.hasLocalFollower(ctx, author.ID)
        if err != nil {
            return err
        }
        if !hasLocalFollower {
            return nil // silently drop — no local user would see it
        }
    }
    if visibility == "direct" {
        if !p.hasLocalRecipient(note.To) {
            return nil
        }
    }

    // Resolve the parent status if this is a reply.
    var inReplyToID *string
    if note.InReplyTo != nil && *note.InReplyTo != "" {
        if parent, err := p.store.GetStatusByAPID(ctx, *note.InReplyTo); err == nil {
            inReplyToID = &parent.ID
        }
    }

    // Store remote media as references (no fetch).
    mediaIDs := p.storeRemoteMedia(ctx, note.Attachment, author.ID)

    // Determine content warning.
    var contentWarning *string
    if note.Summary != nil && *note.Summary != "" {
        contentWarning = note.Summary
    }

    // Store the raw AP JSON.
    apRaw, _ := json.Marshal(note)

    status, err := p.store.CreateStatus(ctx, db.CreateStatusParams{
        ID:             uid.New(),
        Uri:            note.ID,
        AccountID:      author.ID,
        Text:           &note.Content,
        Content:        &note.Content,
        ContentWarning: contentWarning,
        Visibility:     visibility,
        Language:        noteLanguage(note),
        InReplyToID:    inReplyToID,
        ApID:           note.ID,
        ApRaw:          apRaw,
        Sensitive:       note.Sensitive,
        Local:          false,
    })
    if err != nil {
        if isUniqueViolation(err) {
            return nil
        }
        return fmt.Errorf("inbox: create status: %w", err)
    }

    // Attach media to the status.
    for _, mediaID := range mediaIDs {
        _ = p.store.AttachMediaToStatus(ctx, mediaID, status.ID, author.ID)
    }

    // Increment parent replies count.
    if inReplyToID != nil {
        _ = p.store.IncrementRepliesCount(ctx, *inReplyToID)
    }

    // Create mention notifications for local users.
    p.processMentionNotifications(ctx, note.Tag, status.ID, author.ID)

    return nil
}
```

#### Announce (boost) and Like

```go
// handleAnnounce processes an incoming Announce{Note} (boost) activity.
func (p *InboxProcessor) handleAnnounce(ctx context.Context, activity *Activity, actorDomain string) error {
    // Idempotency.
    if activity.ID != "" {
        if _, err := p.store.GetStatusByAPID(ctx, activity.ID); err == nil {
            return nil
        }
    }

    objectID, ok := activity.ObjectID()
    if !ok {
        return &PermanentError{Err: fmt.Errorf("Announce object is not a status IRI")}
    }

    // Resolve the original status.
    original, err := p.store.GetStatusByAPID(ctx, objectID)
    if err != nil {
        // The original post isn't known locally — we can't display the boost.
        // This is expected for posts from servers we don't follow.
        p.logger.Debug("inbox: Announce of unknown status", "object", objectID)
        return nil
    }

    actor, err := p.resolveRemoteAccount(ctx, activity.Actor)
    if err != nil {
        return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
    }

    apRaw, _ := json.Marshal(activity)
    _, err = p.store.CreateStatus(ctx, db.CreateStatusParams{
        ID:         uid.New(),
        Uri:        activity.ID,
        AccountID:  actor.ID,
        Visibility: "public",
        ReblogOfID: &original.ID,
        ApID:       activity.ID,
        ApRaw:      apRaw,
        Local:      false,
    })
    if err != nil {
        if isUniqueViolation(err) {
            return nil
        }
        return fmt.Errorf("inbox: create boost: %w", err)
    }

    _ = p.store.IncrementReblogsCount(ctx, original.ID)

    // Notify the original author if they are local.
    if original.Local {
        p.createNotification(ctx, original.AccountID, actor.ID, "reblog", &original.ID)
    }

    return nil
}

// handleLike processes an incoming Like{Note} activity.
func (p *InboxProcessor) handleLike(ctx context.Context, activity *Activity) error {
    objectID, ok := activity.ObjectID()
    if !ok {
        return &PermanentError{Err: fmt.Errorf("Like object is not a status IRI")}
    }

    status, err := p.store.GetStatusByAPID(ctx, objectID)
    if err != nil {
        p.logger.Debug("inbox: Like of unknown status", "object", objectID)
        return nil
    }

    actor, err := p.resolveRemoteAccount(ctx, activity.Actor)
    if err != nil {
        return fmt.Errorf("inbox: resolve actor %q: %w", activity.Actor, err)
    }

    apID := &activity.ID
    _, err = p.store.CreateFavourite(ctx, db.CreateFavouriteParams{
        ID:        uid.New(),
        AccountID: actor.ID,
        StatusID:  status.ID,
        ApID:      apID,
    })
    if err != nil {
        if isUniqueViolation(err) {
            return nil
        }
        return fmt.Errorf("inbox: create favourite: %w", err)
    }

    _ = p.store.IncrementFavouritesCount(ctx, status.ID)

    if status.Local {
        p.createNotification(ctx, status.AccountID, actor.ID, "favourite", &status.ID)
    }

    return nil
}
```

#### Delete, Update, Block

```go
// handleDelete processes an incoming Delete{Note/Tombstone} or Delete{Person}.
func (p *InboxProcessor) handleDelete(ctx context.Context, activity *Activity) error {
    objectType := activity.ObjectType()

    switch objectType {
    case "Tombstone", "Note", "":
        objectID, ok := activity.ObjectID()
        if !ok {
            // Embedded object — extract ID from it.
            var obj struct{ ID string `json:"id"` }
            if err := json.Unmarshal(activity.ObjectRaw, &obj); err != nil {
                return &PermanentError{Err: fmt.Errorf("Delete: cannot extract object ID")}
            }
            objectID = obj.ID
        }
        if objectID == "" {
            return nil
        }

        // Verify the actor is the author (or the actor is deleting themselves).
        status, err := p.store.GetStatusByAPID(ctx, objectID)
        if err != nil {
            return nil // status not known locally — nothing to delete
        }
        statusAuthor, _ := p.store.GetAccountByID(ctx, status.AccountID)
        if statusAuthor.ApID != activity.Actor {
            return &PermanentError{Err: fmt.Errorf("Delete: actor %q is not the author", activity.Actor)}
        }

        return p.store.SoftDeleteStatus(ctx, status.ID)

    case "Person":
        // Remote account deletion — soft-delete or suspend the remote account.
        // Phase 1: mark the account as suspended. A full cleanup (removing
        // follows, statuses) is a background task for Phase 2.
        account, err := p.store.GetAccountByAPID(ctx, activity.Actor)
        if err != nil {
            return nil
        }
        return p.store.SuspendAccount(ctx, account.ID)

    default:
        p.logger.Debug("inbox: unsupported Delete object type", "type", objectType)
        return nil
    }
}

// handleUpdate processes Update{Note} and Update{Person} activities.
func (p *InboxProcessor) handleUpdate(ctx context.Context, activity *Activity) error {
    objectType := activity.ObjectType()

    switch objectType {
    case "Note":
        note, err := activity.ObjectNote()
        if err != nil {
            return &PermanentError{Err: fmt.Errorf("Update{Note}: %w", err)}
        }

        status, err := p.store.GetStatusByAPID(ctx, note.ID)
        if err != nil {
            return nil // status not known locally
        }

        // Verify the actor is the author.
        author, _ := p.store.GetAccountByID(ctx, status.AccountID)
        if author.ApID != activity.Actor {
            return &PermanentError{Err: fmt.Errorf("Update: actor is not the author")}
        }

        // Snapshot current state for edit history.
        _, _ = p.store.CreateStatusEdit(ctx, db.CreateStatusEditParams{
            ID:             uid.New(),
            StatusID:       status.ID,
            AccountID:      status.AccountID,
            Text:           status.Text,
            Content:        status.Content,
            ContentWarning: status.ContentWarning,
            Sensitive:      status.Sensitive,
        })

        var cw *string
        if note.Summary != nil {
            cw = note.Summary
        }
        _, err = p.store.UpdateStatus(ctx, db.UpdateStatusParams{
            ID:             status.ID,
            Text:           &note.Content,
            Content:        &note.Content,
            ContentWarning: cw,
            Sensitive:      note.Sensitive,
        })
        return err

    case "Person", "Service":
        actor, err := activity.ObjectActor()
        if err != nil {
            return &PermanentError{Err: fmt.Errorf("Update{Person}: %w", err)}
        }
        return p.syncRemoteActor(ctx, actor)

    default:
        p.logger.Debug("inbox: unsupported Update object type", "type", objectType)
        return nil
    }
}

// handleBlock notes a remote block defensively.
// Monstera-fed records that the remote actor has blocked a local user so that
// content is not delivered to the blocking actor. The block is recorded in
// the blocks table (as if the remote user blocked the local user).
func (p *InboxProcessor) handleBlock(ctx context.Context, activity *Activity) error {
    targetID, ok := activity.ObjectID()
    if !ok {
        return &PermanentError{Err: fmt.Errorf("Block object is not an actor IRI")}
    }

    targetUsername := usernameFromActorIRI(targetID, p.cfg.InstanceDomain)
    if targetUsername == "" {
        return nil // not a local user — ignore
    }

    target, err := p.store.GetLocalAccountByUsername(ctx, targetUsername)
    if err != nil {
        return nil
    }

    actor, err := p.resolveRemoteAccount(ctx, activity.Actor)
    if err != nil {
        return fmt.Errorf("inbox: resolve actor: %w", err)
    }

    // Record the block (remote actor blocks local user).
    _, err = p.store.CreateBlock(ctx, db.CreateBlockParams{
        ID:        uid.New(),
        AccountID: actor.ID,
        TargetID:  target.ID,
    })
    if err != nil && !isUniqueViolation(err) {
        return err
    }

    // Remove any existing follow in either direction.
    _ = p.store.DeleteFollow(ctx, actor.ID, target.ID)
    _ = p.store.DeleteFollow(ctx, target.ID, actor.ID)

    return nil
}
```

### Internal Helpers

```go
// resolveRemoteAccount ensures a remote actor exists in the local DB.
// If the account is already known (by ap_id), it is returned. Otherwise,
// a minimal account record is created from the actor IRI. A full profile
// sync (fetching the Actor document) happens asynchronously or on the next
// Update{Person}.
//
// Phase 1 simplification: creates a stub account with the AP ID and domain.
// The Actor document fetch (for display_name, avatar, etc.) is triggered
// lazily when the account is first viewed via the Mastodon API.
func (p *InboxProcessor) resolveRemoteAccount(ctx context.Context, actorIRI string) (store.Account, error) {
    existing, err := p.store.GetAccountByAPID(ctx, actorIRI)
    if err == nil {
        return existing, nil
    }

    domain := DomainFromActorID(actorIRI)
    username := usernameFromActorIRI(actorIRI, "")

    // Create a stub account. The public key and URLs will be populated
    // when the Actor document is fetched (triggered by HTTP Signature
    // verification, which caches the key and actor data).
    account, err := p.store.CreateAccount(ctx, db.CreateAccountParams{
        ID:           uid.New(),
        Username:     username,
        Domain:       &domain,
        PublicKey:     "", // populated on first Actor fetch
        InboxUrl:     actorIRI + "/inbox",
        OutboxUrl:    actorIRI + "/outbox",
        FollowersUrl: actorIRI + "/followers",
        FollowingUrl: actorIRI + "/following",
        ApID:         actorIRI,
    })
    if err != nil {
        if isUniqueViolation(err) {
            return p.store.GetAccountByAPID(ctx, actorIRI)
        }
        return store.Account{}, err
    }
    return account, nil
}

// syncRemoteActor updates a remote account's profile fields from an Actor document.
func (p *InboxProcessor) syncRemoteActor(ctx context.Context, actor *Actor) error {
    account, err := p.store.GetAccountByAPID(ctx, actor.ID)
    if err != nil {
        return nil // actor not known locally — ignore
    }

    apRaw, _ := json.Marshal(actor)
    _, err = p.store.UpdateAccount(ctx, db.UpdateAccountParams{
        ID:          account.ID,
        DisplayName: &actor.Name,
        Note:        &actor.Summary,
        ApRaw:       apRaw,
        Bot:         actor.Type == "Service",
        Locked:      actor.ManuallyApprovesFollowers,
    })
    if err != nil {
        return fmt.Errorf("inbox: sync actor: %w", err)
    }

    // Update public key if it changed.
    if actor.PublicKey.PublicKeyPem != "" && actor.PublicKey.PublicKeyPem != account.PublicKey {
        _ = p.store.UpdateAccountKeys(ctx, account.ID, actor.PublicKey.PublicKeyPem, apRaw)
        // Evict the cached public key so the next signature verification picks up the new key.
        _ = p.cache.Delete(ctx, pubKeyCacheKey(actor.PublicKey.ID))
    }

    return nil
}

// resolveVisibility maps AP addressing to Mastodon visibility levels.
func (p *InboxProcessor) resolveVisibility(note *Note, author store.Account) string {
    for _, addr := range note.To {
        if addr == PublicAddress {
            return "public"
        }
    }
    for _, addr := range note.Cc {
        if addr == PublicAddress {
            return "unlisted"
        }
    }
    for _, addr := range note.To {
        if addr == author.FollowersUrl {
            return "private"
        }
    }
    return "direct"
}

// hasLocalFollower checks if the remote account has at least one accepted
// follower that is a local account. Used to authorize followers-only posts.
func (p *InboxProcessor) hasLocalFollower(ctx context.Context, remoteAccountID string) (bool, error) {
    followers, err := p.store.GetFollowers(ctx, remoteAccountID, "", 1)
    if err != nil {
        return false, err
    }
    for _, f := range followers {
        if f.Domain == nil {
            return true, nil
        }
    }
    return false, nil
}

// hasLocalRecipient checks if any "to" address resolves to a local user.
func (p *InboxProcessor) hasLocalRecipient(to []string) bool {
    for _, addr := range to {
        username := usernameFromActorIRI(addr, p.cfg.InstanceDomain)
        if username != "" {
            return true
        }
    }
    return false
}

// processMentionNotifications scans tags for Mention entries targeting local
// users and creates notifications.
func (p *InboxProcessor) processMentionNotifications(ctx context.Context, tags []Tag, statusID, fromAccountID string) {
    for _, tag := range tags {
        if tag.Type != "Mention" {
            continue
        }
        username := usernameFromActorIRI(tag.Href, p.cfg.InstanceDomain)
        if username == "" {
            continue
        }
        mentioned, err := p.store.GetLocalAccountByUsername(ctx, username)
        if err != nil {
            continue
        }
        p.createNotification(ctx, mentioned.ID, fromAccountID, "mention", &statusID)
    }
}

// storeRemoteMedia creates media_attachment records for remote media
// without fetching the actual files. Only the remote_url is stored.
func (p *InboxProcessor) storeRemoteMedia(ctx context.Context, attachments []Attachment, accountID string) []string {
    var ids []string
    for _, att := range attachments {
        if att.URL == "" {
            continue
        }
        mediaType := "unknown"
        if strings.HasPrefix(att.MediaType, "image/") {
            mediaType = "image"
        } else if strings.HasPrefix(att.MediaType, "video/") {
            mediaType = "video"
        } else if strings.HasPrefix(att.MediaType, "audio/") {
            mediaType = "audio"
        }

        remoteURL := &att.URL
        var desc *string
        if att.Name != "" {
            desc = &att.Name
        }
        var bh *string
        if att.Blurhash != "" {
            bh = &att.Blurhash
        }

        media, err := p.store.CreateMediaAttachment(ctx, db.CreateMediaAttachmentParams{
            ID:         uid.New(),
            AccountID:  accountID,
            Type:       mediaType,
            StorageKey: "", // no local storage — remote only
            Url:        att.URL,
            RemoteUrl:  remoteURL,
            Description: desc,
            Blurhash:   bh,
        })
        if err != nil {
            p.logger.Warn("inbox: failed to store remote media", "url", att.URL, "error", err)
            continue
        }
        ids = append(ids, media.ID)
    }
    return ids
}

// createNotification creates a notification record. Errors are logged but
// not propagated — a failed notification should not reject the activity.
func (p *InboxProcessor) createNotification(ctx context.Context, recipientID, fromID, notifType string, statusID *string) {
    _, err := p.store.CreateNotification(ctx, db.CreateNotificationParams{
        ID:        uid.New(),
        AccountID: recipientID,
        FromID:    fromID,
        Type:      notifType,
        StatusID:  statusID,
    })
    if err != nil {
        p.logger.Warn("inbox: failed to create notification",
            "type", notifType, "recipient", recipientID, "error", err)
    }
}

// sendAcceptFollow publishes an Accept{Follow} activity back to the remote
// actor. This is called when a Follow is auto-accepted (target.locked=false).
// The actual HTTP delivery happens asynchronously via the federation worker.
func (p *InboxProcessor) sendAcceptFollow(ctx context.Context, target, actor *store.Account, followActivity *Activity, followID string) error {
    // This will be wired to the OutboxPublisher in Stage 3. For now, the
    // method signature establishes the contract.
    // p.outbox.AcceptFollow(ctx, target, actor, followActivity)
    return nil
}

// usernameFromActorIRI extracts the username from a local actor IRI.
// Returns "" if the IRI does not belong to the given instance domain.
//
// Expected format: https://{instanceDomain}/users/{username}
func usernameFromActorIRI(iri, instanceDomain string) string {
    u, err := url.Parse(iri)
    if err != nil {
        return ""
    }
    if instanceDomain != "" && u.Host != instanceDomain {
        return ""
    }
    parts := strings.Split(strings.Trim(u.Path, "/"), "/")
    if len(parts) == 2 && parts[0] == "users" {
        return parts[1]
    }
    return ""
}

// noteLanguage extracts the language from a Note's contentMap.
// Returns a pointer to the language code, or nil if unknown.
func noteLanguage(note *Note) *string {
    for lang := range note.ContentMap {
        return &lang
    }
    return nil
}

// isUniqueViolation checks if a pgx error is a unique constraint violation.
func isUniqueViolation(err error) bool {
    var pgErr interface{ Code() string }
    if errors.As(err, &pgErr) {
        return pgErr.Code() == "23505" // unique_violation
    }
    return false
}
```

---

## 4. `internal/ap/outbox.go` — Outbox Publisher

The OutboxPublisher is called by the service layer when a local user performs an action that must be federated. It builds the AP activity JSON, determines the delivery targets, and enqueues NATS messages for the federation worker.

### Types and Constructor

```go
package ap

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "strings"
    "time"

    "github.com/yourorg/monstera-fed/internal/config"
    "github.com/yourorg/monstera-fed/internal/store"
    db "github.com/yourorg/monstera-fed/internal/store/postgres/generated"
    "github.com/yourorg/monstera-fed/internal/uid"
)

// DeliveryEnqueuer abstracts NATS message publishing so that the outbox
// package doesn't import the NATS client directly.
type DeliveryEnqueuer interface {
    // EnqueueDelivery publishes a federation delivery job to NATS JetStream.
    // activityType is used as the subject suffix (e.g. "create" → "federation.deliver.create").
    EnqueueDelivery(ctx context.Context, activityType string, msg DeliveryMessage) error
}

// DeliveryMessage is the payload published to the FEDERATION NATS stream.
// One message is enqueued per unique target inbox URL.
type DeliveryMessage struct {
    ActivityID  string          `json:"activity_id"`
    Activity    json.RawMessage `json:"activity"`
    TargetInbox string          `json:"target_inbox"`
    SenderID    string          `json:"sender_id"` // local account ID — used to load the signing key
}

// OutboxPublisher creates AP activities from local actions and enqueues
// them for delivery to remote inboxes.
//
// All methods are safe for concurrent use.
type OutboxPublisher struct {
    store    store.Store
    enqueue  DeliveryEnqueuer
    cfg      *config.Config
    logger   *slog.Logger
}

// NewOutboxPublisher constructs an OutboxPublisher.
func NewOutboxPublisher(
    s store.Store,
    enqueue DeliveryEnqueuer,
    cfg *config.Config,
    logger *slog.Logger,
) *OutboxPublisher {
    return &OutboxPublisher{
        store:   s,
        enqueue: enqueue,
        cfg:     cfg,
        logger:  logger,
    }
}
```

### Publishing Methods

#### PublishStatus (Create{Note})

```go
// PublishStatus creates a Create{Note} activity and enqueues delivery to all
// remote followers of the author.
//
// Fan-out strategy:
//  1. Query all remote followers' inbox URLs via GetFollowerInboxURLs.
//  2. Deduplicate by shared inbox: if a remote server advertises a shared
//     inbox (endpoints.sharedInbox), all followers on that server receive
//     a single delivery to the shared inbox rather than N deliveries to
//     individual inboxes.
//  3. Enqueue one NATS message per unique inbox URL.
//
// For public/unlisted statuses, the activity is also delivered to the
// shared inboxes of any servers that are mentioned in the post (so that
// the mentioned users see the mention notification).
func (p *OutboxPublisher) PublishStatus(ctx context.Context, status *store.Status, author *store.Account) error {
    note := p.statusToNote(status, author)
    activityID := ActivityID(p.cfg.InstanceDomain)
    activity := WrapInCreate(activityID, note)

    return p.fanOutToFollowers(ctx, author, "create", activity)
}
```

#### DeleteStatus

```go
// DeleteStatus creates a Delete{Tombstone} activity and delivers it to all
// remote followers.
func (p *OutboxPublisher) DeleteStatus(ctx context.Context, status *store.Status, author *store.Account) error {
    activityID := ActivityID(p.cfg.InstanceDomain)
    activity := NewDeleteActivity(activityID, author.ApID, status.ApID)

    return p.fanOutToFollowers(ctx, author, "delete", activity)
}
```

#### Follow / Unfollow

```go
// Follow creates a Follow activity and sends it to the target's inbox.
// The follow's ap_id is set to the generated activity ID.
func (p *OutboxPublisher) Follow(ctx context.Context, follow *store.Follow, follower, target *store.Account) error {
    activityID := ActivityID(p.cfg.InstanceDomain)

    // Store the AP ID on the follow record.
    apID := &activityID
    // The service layer should have already set follow.ApID; if not, update it.
    if follow.ApID == nil || *follow.ApID == "" {
        _ = p.store.WithTx(ctx, func(tx store.Store) error {
            // Update follow's ap_id — requires a query not yet in the store.
            // For now, the service layer sets it at creation time.
            return nil
        })
    }

    activity := NewFollowActivity(activityID, follower.ApID, target.ApID)

    return p.deliverToInbox(ctx, follower.ID, "follow", activity, target.InboxUrl)
}

// Unfollow creates an Undo{Follow} activity and sends it to the target's inbox.
func (p *OutboxPublisher) Unfollow(ctx context.Context, follow *store.Follow, follower, target *store.Account) error {
    // Reconstruct the original Follow activity.
    followAPID := ""
    if follow.ApID != nil {
        followAPID = *follow.ApID
    }
    originalFollow := NewFollowActivity(followAPID, follower.ApID, target.ApID)

    activityID := ActivityID(p.cfg.InstanceDomain)
    undo := NewUndoActivity(activityID, follower.ApID, originalFollow)

    return p.deliverToInbox(ctx, follower.ID, "undo", undo, target.InboxUrl)
}
```

#### AcceptFollow / RejectFollow

```go
// AcceptFollow creates an Accept{Follow} activity and sends it to the
// follower's inbox.
func (p *OutboxPublisher) AcceptFollow(ctx context.Context, follow *store.Follow, target, follower *store.Account) error {
    followAPID := ""
    if follow.ApID != nil {
        followAPID = *follow.ApID
    }
    originalFollow := NewFollowActivity(followAPID, follower.ApID, target.ApID)

    activityID := ActivityID(p.cfg.InstanceDomain)
    accept := NewAcceptActivity(activityID, target.ApID, originalFollow)

    return p.deliverToInbox(ctx, target.ID, "accept", accept, follower.InboxUrl)
}

// RejectFollow creates a Reject{Follow} activity and sends it to the
// follower's inbox.
func (p *OutboxPublisher) RejectFollow(ctx context.Context, follow *store.Follow, target, follower *store.Account) error {
    followAPID := ""
    if follow.ApID != nil {
        followAPID = *follow.ApID
    }
    originalFollow := NewFollowActivity(followAPID, follower.ApID, target.ApID)

    activityID := ActivityID(p.cfg.InstanceDomain)
    reject := NewRejectActivity(activityID, target.ApID, originalFollow)

    return p.deliverToInbox(ctx, target.ID, "reject", reject, follower.InboxUrl)
}
```

#### Boost / Favourite / Undo variants

```go
// Boost creates an Announce{Note} activity and delivers it to all remote
// followers of the booster.
func (p *OutboxPublisher) Boost(ctx context.Context, boost *store.Status, booster *store.Account, originalStatus *store.Status) error {
    activityID := ActivityID(p.cfg.InstanceDomain)

    to := []string{PublicAddress}
    cc := []string{booster.FollowersUrl}

    activity := NewAnnounceActivity(
        activityID,
        booster.ApID,
        originalStatus.ApID,
        to, cc,
        boost.CreatedAt.UTC().Format(time.RFC3339),
    )

    return p.fanOutToFollowers(ctx, booster, "announce", activity)
}

// UndoBoost creates an Undo{Announce} activity.
func (p *OutboxPublisher) UndoBoost(ctx context.Context, boost *store.Status, booster *store.Account) error {
    // Reconstruct the original Announce from the boost's ap_id.
    originalAnnounce := &Activity{
        ID:   boost.ApID,
        Type: "Announce",
        Actor: booster.ApID,
    }

    activityID := ActivityID(p.cfg.InstanceDomain)
    undo := NewUndoActivity(activityID, booster.ApID, originalAnnounce)

    return p.fanOutToFollowers(ctx, booster, "undo", undo)
}

// Favourite creates a Like{Note} activity and delivers it to the status
// author's inbox.
func (p *OutboxPublisher) Favourite(ctx context.Context, favourite *store.Favourite, actor *store.Account, status *store.Status) error {
    activityID := ActivityID(p.cfg.InstanceDomain)
    activity := NewLikeActivity(activityID, actor.ApID, status.ApID)

    // Deliver to the status author's inbox.
    author, err := p.store.GetAccountByID(ctx, status.AccountID)
    if err != nil {
        return fmt.Errorf("outbox: resolve status author: %w", err)
    }

    if author.Domain == nil {
        return nil // local author — no federation needed
    }

    return p.deliverToInbox(ctx, actor.ID, "like", activity, author.InboxUrl)
}

// UndoFavourite creates an Undo{Like} activity.
func (p *OutboxPublisher) UndoFavourite(ctx context.Context, favourite *store.Favourite, actor *store.Account, status *store.Status) error {
    // Reconstruct the original Like from the favourite's ap_id.
    likeAPID := ""
    if favourite.ApID != nil {
        likeAPID = *favourite.ApID
    }
    originalLike := NewLikeActivity(likeAPID, actor.ApID, status.ApID)

    activityID := ActivityID(p.cfg.InstanceDomain)
    undo := NewUndoActivity(activityID, actor.ApID, originalLike)

    author, err := p.store.GetAccountByID(ctx, status.AccountID)
    if err != nil {
        return fmt.Errorf("outbox: resolve status author: %w", err)
    }
    if author.Domain == nil {
        return nil
    }

    return p.deliverToInbox(ctx, actor.ID, "undo", undo, author.InboxUrl)
}
```

### Fan-out and Delivery Helpers

```go
// fanOutToFollowers queries remote follower inbox URLs, deduplicates by
// shared inbox, and enqueues one delivery job per unique inbox.
func (p *OutboxPublisher) fanOutToFollowers(ctx context.Context, author *store.Account, activityType string, activity *Activity) error {
    inboxURLs, err := p.store.GetFollowerInboxURLs(ctx, author.ID)
    if err != nil {
        return fmt.Errorf("outbox: get follower inboxes: %w", err)
    }

    // Deduplicate: many followers on the same server share an inbox URL.
    // The GetFollowerInboxURLs query already returns inbox_url, but some
    // servers use per-user inboxes rather than shared inboxes. For true
    // deduplication, we'd need the shared_inbox_url from the account record.
    // Phase 1 simplification: deduplicate the raw inbox URLs. Shared inbox
    // optimization (querying accounts.shared_inbox_url) is Phase 2.
    seen := make(map[string]bool, len(inboxURLs))
    var unique []string
    for _, url := range inboxURLs {
        if !seen[url] {
            seen[url] = true
            unique = append(unique, url)
        }
    }

    activityJSON, err := json.Marshal(activity)
    if err != nil {
        return fmt.Errorf("outbox: marshal activity: %w", err)
    }

    for _, inbox := range unique {
        msg := DeliveryMessage{
            ActivityID:  activity.ID,
            Activity:    activityJSON,
            TargetInbox: inbox,
            SenderID:    author.ID,
        }
        if err := p.enqueue.EnqueueDelivery(ctx, strings.ToLower(activityType), msg); err != nil {
            p.logger.Error("outbox: failed to enqueue delivery",
                "activity_id", activity.ID,
                "target_inbox", inbox,
                "error", err,
            )
        }
    }

    p.logger.Info("outbox: activity enqueued",
        "type", activity.Type,
        "id", activity.ID,
        "targets", len(unique),
    )

    return nil
}

// deliverToInbox enqueues a single delivery to a specific inbox URL.
// Used for point-to-point activities (Follow, Accept, Like) that go to
// one recipient rather than fanning out to all followers.
func (p *OutboxPublisher) deliverToInbox(ctx context.Context, senderID, activityType string, activity *Activity, inboxURL string) error {
    activityJSON, err := json.Marshal(activity)
    if err != nil {
        return fmt.Errorf("outbox: marshal activity: %w", err)
    }

    msg := DeliveryMessage{
        ActivityID:  activity.ID,
        Activity:    activityJSON,
        TargetInbox: inboxURL,
        SenderID:    senderID,
    }

    return p.enqueue.EnqueueDelivery(ctx, strings.ToLower(activityType), msg)
}

// statusToNote converts a local Status + Account to an AP Note suitable
// for federation. All URLs use the instance domain.
func (p *OutboxPublisher) statusToNote(status *store.Status, author *store.Account) *Note {
    domain := p.cfg.InstanceDomain
    noteID := status.ApID
    noteURL := fmt.Sprintf("https://%s/@%s/%s", domain, author.Username, status.ID)

    to, cc := p.resolveAddressing(status, author)

    var inReplyTo *string
    if status.InReplyToID != nil {
        if parent, err := p.store.GetStatusByID(context.Background(), *status.InReplyToID); err == nil {
            inReplyTo = &parent.ApID
        }
    }

    var summary *string
    if status.ContentWarning != nil && *status.ContentWarning != "" {
        summary = status.ContentWarning
    }

    note := &Note{
        Context:      DefaultContext,
        ID:           noteID,
        Type:         "Note",
        AttributedTo: author.ApID,
        Content:      safeDeref(status.Content),
        To:           to,
        Cc:           cc,
        InReplyTo:    inReplyTo,
        Published:    status.CreatedAt.UTC().Format(time.RFC3339),
        URL:          noteURL,
        Sensitive:    status.Sensitive,
        Summary:      summary,
    }

    if status.Language != nil && *status.Language != "" {
        note.ContentMap = map[string]string{*status.Language: safeDeref(status.Content)}
    }

    if status.Text != nil {
        note.Source = &NoteSource{
            Content:   *status.Text,
            MediaType: "text/plain",
        }
    }

    if status.EditedAt != nil {
        note.Updated = status.EditedAt.UTC().Format(time.RFC3339)
    }

    return note
}

// resolveAddressing maps Mastodon visibility to AP to/cc fields.
func (p *OutboxPublisher) resolveAddressing(status *store.Status, author *store.Account) (to, cc []string) {
    switch status.Visibility {
    case "public":
        to = []string{PublicAddress}
        cc = []string{author.FollowersUrl}
    case "unlisted":
        to = []string{author.FollowersUrl}
        cc = []string{PublicAddress}
    case "private":
        to = []string{author.FollowersUrl}
    case "direct":
        // Phase 1: direct messages address specific users.
        // The to list should be populated by the caller from mentioned users.
        // For now, return empty — the service layer will add recipients.
    }
    return to, cc
}

func safeDeref(s *string) string {
    if s == nil {
        return ""
    }
    return *s
}
```

---

## 5. `internal/nats/federation/producer.go` — Delivery Enqueuer

```go
package federation

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/nats-io/nats.go/jetstream"

    "github.com/yourorg/monstera-fed/internal/ap"
    "github.com/yourorg/monstera-fed/internal/observability"
)

// streamName is the NATS JetStream stream for federation delivery.
const streamName = "FEDERATION"

// dlqStreamName is the dead-letter queue stream.
const dlqStreamName = "FEDERATION_DLQ"

// subjectPrefix is the base subject for delivery messages.
const subjectPrefix = "federation.deliver."

// Producer publishes federation delivery messages to NATS JetStream.
type Producer struct {
    js      jetstream.JetStream
    metrics *observability.Metrics
}

// NewProducer constructs a federation delivery Producer.
func NewProducer(js jetstream.JetStream, metrics *observability.Metrics) *Producer {
    return &Producer{js: js, metrics: metrics}
}

// EnsureStreams creates the FEDERATION and FEDERATION_DLQ streams if they
// do not already exist. Called once during startup.
//
// FEDERATION stream configuration:
//   - Subjects: federation.deliver.> (wildcard captures activity type suffix)
//   - Retention: WorkQueuePolicy — message deleted after acknowledgement
//   - Storage: FileStorage — durable across NATS restarts
//   - MaxDeliver: 5 — maximum delivery attempts before message moves to DLQ
//   - AckWait: 60s — time a consumer has to acknowledge before redelivery
//
// FEDERATION_DLQ stream:
//   - Subjects: federation.dlq.>
//   - Retention: LimitsPolicy — retained for admin inspection
//   - MaxAge: 7 days — auto-purged after one week
func EnsureStreams(ctx context.Context, js jetstream.JetStream) error {
    _, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
        Name:      streamName,
        Subjects:  []string{subjectPrefix + ">"},
        Retention: jetstream.WorkQueuePolicy,
        Storage:   jetstream.FileStorage,
        MaxAge:    7 * 24 * time.Hour,
    })
    if err != nil {
        return fmt.Errorf("federation: create stream %s: %w", streamName, err)
    }

    _, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
        Name:      dlqStreamName,
        Subjects:  []string{"federation.dlq.>"},
        Retention: jetstream.LimitsPolicy,
        Storage:   jetstream.FileStorage,
        MaxAge:    7 * 24 * time.Hour,
    })
    if err != nil {
        return fmt.Errorf("federation: create stream %s: %w", dlqStreamName, err)
    }

    return nil
}

// EnqueueDelivery publishes a delivery message to the FEDERATION stream.
// The subject includes the activity type for observability (e.g.
// "federation.deliver.create").
//
// Satisfies the ap.DeliveryEnqueuer interface.
func (p *Producer) EnqueueDelivery(ctx context.Context, activityType string, msg ap.DeliveryMessage) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return fmt.Errorf("federation: marshal delivery message: %w", err)
    }

    subject := subjectPrefix + activityType
    _, err = p.js.Publish(ctx, subject, data)
    if err != nil {
        p.metrics.NATSPublishTotal.WithLabelValues(subject, "error").Inc()
        return fmt.Errorf("federation: publish to %s: %w", subject, err)
    }

    p.metrics.NATSPublishTotal.WithLabelValues(subject, "ok").Inc()
    return nil
}

// EnqueueDLQ moves a failed delivery message to the dead-letter queue.
// Called by the worker after exhausting all retry attempts.
func (p *Producer) EnqueueDLQ(ctx context.Context, activityType string, msg ap.DeliveryMessage) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    subject := "federation.dlq." + activityType
    _, err = p.js.Publish(ctx, subject, data)
    return err
}
```

---

## 6. `internal/nats/federation/worker.go` — Federation Delivery Worker

```go
package federation

import (
    "bytes"
    "context"
    "crypto/rsa"
    "crypto/x509"
    "encoding/json"
    "encoding/pem"
    "fmt"
    "io"
    "log/slog"
    "net/http"
    "strings"
    "time"

    "github.com/nats-io/nats.go/jetstream"

    "github.com/yourorg/monstera-fed/internal/ap"
    "github.com/yourorg/monstera-fed/internal/cache"
    "github.com/yourorg/monstera-fed/internal/config"
    "github.com/yourorg/monstera-fed/internal/observability"
    "github.com/yourorg/monstera-fed/internal/store"
)

// consumerName is the durable consumer name on the FEDERATION stream.
const consumerName = "federation-worker"

// FederationWorker consumes delivery jobs from the FEDERATION JetStream
// stream and POSTs AP activities to remote inboxes with HTTP Signature
// authentication.
//
// Concurrency: the worker spawns cfg.FederationWorkerConcurrency goroutines,
// each independently fetching and processing messages. Backpressure is
// inherent in the pull consumer model — workers only fetch the next message
// after completing the current one.
type FederationWorker struct {
    js        jetstream.JetStream
    consumer  jetstream.Consumer
    producer  *Producer
    http      *http.Client
    store     store.Store
    cache     cache.Store
    blocklist *ap.BlocklistCache
    cfg       *config.Config
    logger    *slog.Logger
    metrics   *observability.Metrics
}

// NewFederationWorker constructs a FederationWorker. Call Start() to begin
// consuming messages.
func NewFederationWorker(
    js jetstream.JetStream,
    producer *Producer,
    s store.Store,
    c cache.Store,
    bl *ap.BlocklistCache,
    cfg *config.Config,
    logger *slog.Logger,
    metrics *observability.Metrics,
) *FederationWorker {
    client := &http.Client{
        Timeout: 30 * time.Second,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            if len(via) >= 3 {
                return fmt.Errorf("too many redirects")
            }
            return nil
        },
    }

    return &FederationWorker{
        js:        js,
        producer:  producer,
        http:      client,
        store:     s,
        cache:     c,
        blocklist: bl,
        cfg:       cfg,
        logger:    logger,
        metrics:   metrics,
    }
}

// Start creates the durable pull consumer and launches worker goroutines.
// Blocks until ctx is cancelled.
func (w *FederationWorker) Start(ctx context.Context) error {
    consumer, err := w.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
        Durable:       consumerName,
        AckPolicy:     jetstream.AckExplicitPolicy,
        AckWait:       60 * time.Second,
        MaxDeliver:    5,
        FilterSubject: subjectPrefix + ">",
    })
    if err != nil {
        return fmt.Errorf("federation: create consumer: %w", err)
    }
    w.consumer = consumer

    concurrency := w.cfg.FederationWorkerConcurrency
    if concurrency <= 0 {
        concurrency = 5
    }

    w.logger.Info("federation worker started",
        "concurrency", concurrency,
        "consumer", consumerName,
    )

    for i := 0; i < concurrency; i++ {
        go w.runWorker(ctx, i)
    }

    <-ctx.Done()
    w.logger.Info("federation worker stopping")
    return nil
}

// runWorker is the main loop for a single worker goroutine. It fetches one
// message at a time, processes it, and acknowledges or NAKs.
func (w *FederationWorker) runWorker(ctx context.Context, workerID int) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        msgs, err := w.consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
        if err != nil {
            if ctx.Err() != nil {
                return
            }
            continue
        }

        for msg := range msgs.Messages() {
            w.processMessage(ctx, msg)
        }
    }
}

// processMessage handles a single delivery message.
//
// Outcome matrix:
//   - 2xx from target inbox → Ack (success)
//   - 410 Gone → Ack + trigger follow cleanup
//   - 429 Too Many Requests → Nak with backoff (retry later)
//   - Other 4xx → Ack (permanent failure — do not retry)
//   - 5xx or network error → Nak with backoff (transient — retry)
//   - MaxDeliver exceeded → move to DLQ, Ack
func (w *FederationWorker) processMessage(ctx context.Context, msg jetstream.Msg) {
    start := time.Now()

    var delivery ap.DeliveryMessage
    if err := json.Unmarshal(msg.Data(), &delivery); err != nil {
        w.logger.Error("federation: unmarshal delivery message", "error", err)
        msg.Ack() // permanent — malformed message
        w.metrics.FederationDeliveriesTotal.WithLabelValues("rejected").Inc()
        return
    }

    logger := w.logger.With(
        "activity_id", delivery.ActivityID,
        "target_inbox", delivery.TargetInbox,
    )

    // Domain block check — don't deliver to blocked domains.
    targetDomain := ap.DomainFromActorID(delivery.TargetInbox)
    if blocked, _ := w.blocklist.IsBlocked(ctx, targetDomain); blocked {
        logger.Debug("federation: skipped delivery to blocked domain", "domain", targetDomain)
        msg.Ack()
        w.metrics.FederationDeliveriesTotal.WithLabelValues("rejected").Inc()
        return
    }

    // Load the sender's private key for HTTP Signature signing.
    sender, err := w.store.GetAccountByID(ctx, delivery.SenderID)
    if err != nil {
        logger.Error("federation: sender account not found", "sender_id", delivery.SenderID, "error", err)
        msg.Ack() // permanent — sender deleted
        w.metrics.FederationDeliveriesTotal.WithLabelValues("rejected").Inc()
        return
    }

    privateKey, err := parsePrivateKey(sender.PrivateKey)
    if err != nil {
        logger.Error("federation: parse sender private key", "error", err)
        msg.Ack()
        w.metrics.FederationDeliveriesTotal.WithLabelValues("rejected").Inc()
        return
    }

    // Build and sign the HTTP request.
    keyID := sender.ApID + "#main-key"
    statusCode, err := w.deliverHTTP(ctx, delivery, keyID, privateKey)

    duration := time.Since(start).Seconds()
    w.metrics.FederationDeliveryDurationSeconds.Observe(duration)

    if err != nil {
        logger.Warn("federation: delivery failed",
            "error", err, "duration_ms", int(duration*1000))
        w.handleDeliveryFailure(ctx, msg, delivery, 0, logger)
        return
    }

    logger.Info("federation: delivery complete",
        "status", statusCode, "duration_ms", int(duration*1000))

    switch {
    case statusCode >= 200 && statusCode < 300:
        msg.Ack()
        w.metrics.FederationDeliveriesTotal.WithLabelValues("success").Inc()

    case statusCode == 410:
        // Gone — the remote inbox no longer exists. Acknowledge the message
        // and clean up: the remote server has likely shut down or the account
        // was deleted.
        msg.Ack()
        w.metrics.FederationDeliveriesTotal.WithLabelValues("success").Inc()
        logger.Info("federation: remote inbox returned 410 Gone",
            "domain", targetDomain)

    case statusCode == 429:
        w.handleDeliveryFailure(ctx, msg, delivery, statusCode, logger)

    case statusCode >= 400 && statusCode < 500:
        // Other 4xx — permanent client error (bad request, unauthorized, etc.)
        msg.Ack()
        w.metrics.FederationDeliveriesTotal.WithLabelValues("rejected").Inc()
        logger.Warn("federation: permanent failure", "status", statusCode)

    default:
        // 5xx — transient server error, retry.
        w.handleDeliveryFailure(ctx, msg, delivery, statusCode, logger)
    }
}

// deliverHTTP performs the actual HTTP POST to the remote inbox.
func (w *FederationWorker) deliverHTTP(ctx context.Context, delivery ap.DeliveryMessage, keyID string, privateKey *rsa.PrivateKey) (int, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.TargetInbox,
        bytes.NewReader(delivery.Activity))
    if err != nil {
        return 0, fmt.Errorf("build request: %w", err)
    }

    req.Header.Set("Content-Type", "application/activity+json")
    req.Header.Set("User-Agent", fmt.Sprintf("Monstera-fed/0.1 (+https://%s)", w.cfg.InstanceDomain))
    req.Header.Set("Accept", "application/activity+json")

    if err := ap.Sign(req, keyID, privateKey); err != nil {
        return 0, fmt.Errorf("sign request: %w", err)
    }

    resp, err := w.http.Do(req)
    if err != nil {
        return 0, fmt.Errorf("http post: %w", err)
    }
    defer resp.Body.Close()
    io.Copy(io.Discard, resp.Body) // drain body to allow connection reuse

    return resp.StatusCode, nil
}

// handleDeliveryFailure decides whether to retry (Nak with delay) or move
// the message to the DLQ.
//
// Retry schedule (exponential backoff):
//   Attempt 1: immediate (already happened)
//   Attempt 2: 5 minutes
//   Attempt 3: 30 minutes
//   Attempt 4: 2 hours
//   Attempt 5: 12 hours
//   After attempt 5: move to FEDERATION_DLQ
func (w *FederationWorker) handleDeliveryFailure(ctx context.Context, msg jetstream.Msg, delivery ap.DeliveryMessage, statusCode int, logger *slog.Logger) {
    meta, _ := msg.Metadata()
    attempt := 1
    if meta != nil {
        attempt = int(meta.NumDelivered)
    }

    if attempt >= 5 {
        logger.Warn("federation: max retries exceeded, moving to DLQ",
            "attempt", attempt, "status", statusCode)
        activityType := subjectSuffix(msg.Subject())
        _ = w.producer.EnqueueDLQ(ctx, activityType, delivery)
        msg.Ack()
        w.metrics.FederationDeliveriesTotal.WithLabelValues("failure").Inc()
        return
    }

    delay := retryDelay(attempt)
    logger.Info("federation: scheduling retry",
        "attempt", attempt, "delay", delay, "status", statusCode)

    msg.NakWithDelay(delay)
    w.metrics.FederationDeliveriesTotal.WithLabelValues("failure").Inc()
}

// retryDelay returns the backoff duration for the given attempt number.
func retryDelay(attempt int) time.Duration {
    switch attempt {
    case 1:
        return 5 * time.Minute
    case 2:
        return 30 * time.Minute
    case 3:
        return 2 * time.Hour
    case 4:
        return 12 * time.Hour
    default:
        return 12 * time.Hour
    }
}

// parsePrivateKey parses a PEM-encoded RSA private key string.
// The private_key column may be nil for remote accounts (which don't have
// private keys) — callers must check before calling.
func parsePrivateKey(keyPEM *string) (*rsa.PrivateKey, error) {
    if keyPEM == nil || *keyPEM == "" {
        return nil, fmt.Errorf("no private key")
    }
    block, _ := pem.Decode([]byte(*keyPEM))
    if block == nil {
        return nil, fmt.Errorf("failed to decode PEM block")
    }
    key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
    if err != nil {
        // Try PKCS8 as a fallback.
        parsed, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
        if err2 != nil {
            return nil, fmt.Errorf("parse private key: %w (pkcs1: %v)", err2, err)
        }
        rsaKey, ok := parsed.(*rsa.PrivateKey)
        if !ok {
            return nil, fmt.Errorf("private key is not RSA")
        }
        return rsaKey, nil
    }
    return key, nil
}

// subjectSuffix extracts the activity type from a NATS subject.
// "federation.deliver.create" → "create"
func subjectSuffix(subject string) string {
    parts := strings.Split(subject, ".")
    if len(parts) > 0 {
        return parts[len(parts)-1]
    }
    return "unknown"
}
```

### Configuration Addition

```go
// Added to internal/config/config.go:

// FederationWorkerConcurrency controls how many goroutines concurrently
// pull delivery jobs from the FEDERATION NATS stream. Default: 5.
// Higher values increase delivery throughput at the cost of more outbound
// HTTP connections and CPU.
FederationWorkerConcurrency int // default: 5; env: FEDERATION_WORKER_CONCURRENCY
```

### Startup Wiring

```go
// In cmd/monstera-fed/serve.go, after NATS connection and before HTTP server start:

// Ensure NATS JetStream streams exist.
if err := federation.EnsureStreams(ctx, js); err != nil {
    logger.Error("failed to create federation streams", "error", err)
    os.Exit(1)
}

// Build federation producer and worker.
fedProducer := federation.NewProducer(js, metrics)
blocklist := ap.NewBlocklistCache(store, cacheStore, logger)
if err := blocklist.Refresh(ctx); err != nil {
    logger.Error("failed to load domain blocklist", "error", err)
    os.Exit(1)
}

outbox := ap.NewOutboxPublisher(store, fedProducer, cfg, logger)
inbox := ap.NewInboxProcessor(store, cacheStore, blocklist, sseHub, cfg, logger)

fedWorker := federation.NewFederationWorker(
    js, fedProducer, store, cacheStore, blocklist, cfg, logger, metrics,
)
go func() {
    if err := fedWorker.Start(ctx); err != nil {
        logger.Error("federation worker exited", "error", err)
    }
}()
```

---

## 7. `internal/api/activitypub/` — AP HTTP Handlers

All handlers live under `internal/api/activitypub/`. Each file exports a handler struct constructed via dependency injection. Handlers never reference the NATS client or federation worker directly — they work through the `InboxProcessor` and the `store.Store` interface.

### Route Registration

Routes are registered in `internal/api/router.go` within `NewRouter`:

```go
// --- ActivityPub / Federation routes (no auth required) ---------------------

apDeps := activitypub.Deps{
    Store:     deps.Store,
    Cache:     deps.Cache,
    Inbox:     inbox,
    Blocklist: blocklist,
    Config:    deps.Config,
    Logger:    deps.Logger,
}

// Well-known discovery endpoints.
r.Get("/.well-known/webfinger", activitypub.NewWebFingerHandler(apDeps).ServeHTTP)
r.Get("/.well-known/nodeinfo", activitypub.NewNodeInfoPointerHandler(apDeps).ServeHTTP)
r.Get("/nodeinfo/2.0", activitypub.NewNodeInfoHandler(apDeps).ServeHTTP)

// Actor and collections.
r.Get("/users/{username}", activitypub.NewActorHandler(apDeps).ServeHTTP)
r.Get("/users/{username}/outbox", activitypub.NewOutboxHandler(apDeps).ServeHTTP)
r.Get("/users/{username}/followers", activitypub.NewFollowersHandler(apDeps).ServeHTTP)
r.Get("/users/{username}/following", activitypub.NewFollowingHandler(apDeps).ServeHTTP)
r.Get("/users/{username}/collections/featured", activitypub.NewFeaturedHandler(apDeps).ServeHTTP)

// Inboxes.
r.Post("/users/{username}/inbox", activitypub.NewInboxHandler(apDeps).ServeHTTP)
r.Post("/inbox", activitypub.NewInboxHandler(apDeps).ServeHTTP) // shared inbox
```

### Shared Dependencies

```go
package activitypub

import (
    "log/slog"

    "github.com/yourorg/monstera-fed/internal/ap"
    "github.com/yourorg/monstera-fed/internal/cache"
    "github.com/yourorg/monstera-fed/internal/config"
    "github.com/yourorg/monstera-fed/internal/store"
)

// Deps collects all dependencies needed by AP HTTP handlers.
// Passed to every handler constructor; avoids long parameter lists.
type Deps struct {
    Store     store.Store
    Cache     cache.Store
    Inbox     *ap.InboxProcessor
    Blocklist *ap.BlocklistCache
    Config    *config.Config
    Logger    *slog.Logger
}

// writeJSON is a shared helper that sets Content-Type and encodes JSON.
// AP endpoints use "application/activity+json" unless noted otherwise.
func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}

// writeJRD writes a JRD (JSON Resource Descriptor) response.
func writeJRD(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/jrd+json; charset=utf-8")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}
```

---

### 7a. `webfinger.go` — WebFinger Lookup

```go
package activitypub

import (
    "fmt"
    "net/http"
    "strings"
)

// WebFingerHandler handles GET /.well-known/webfinger?resource=acct:user@domain
//
// Returns a JRD (RFC 7033) document that maps an acct: URI to the AP Actor URL.
// This is the primary discovery mechanism: remote servers query WebFinger to
// find a user's Actor URL before fetching the full Actor document.
//
// Required query parameter: resource (e.g. "acct:alice@example.com")
// Response Content-Type: application/jrd+json
type WebFingerHandler struct {
    deps Deps
}

func NewWebFingerHandler(deps Deps) *WebFingerHandler {
    return &WebFingerHandler{deps: deps}
}

// webFingerResponse is the JRD response envelope.
type webFingerResponse struct {
    Subject string          `json:"subject"`
    Aliases []string        `json:"aliases"`
    Links   []webFingerLink `json:"links"`
}

type webFingerLink struct {
    Rel  string `json:"rel"`
    Type string `json:"type,omitempty"`
    Href string `json:"href,omitempty"`
}

func (h *WebFingerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    resource := r.URL.Query().Get("resource")
    if resource == "" {
        writeError(w, http.StatusBadRequest, "missing resource parameter")
        return
    }

    // Parse "acct:username@domain"
    if !strings.HasPrefix(resource, "acct:") {
        writeError(w, http.StatusBadRequest, "resource must use acct: scheme")
        return
    }

    acct := strings.TrimPrefix(resource, "acct:")
    parts := strings.SplitN(acct, "@", 2)
    if len(parts) != 2 {
        writeError(w, http.StatusBadRequest, "invalid acct URI")
        return
    }

    username := parts[0]
    domain := parts[1]

    // Only answer for our own domain.
    if !strings.EqualFold(domain, h.deps.Config.InstanceDomain) {
        writeError(w, http.StatusNotFound, "account not found")
        return
    }

    // Look up local account.
    account, err := h.deps.Store.GetLocalAccountByUsername(r.Context(), username)
    if err != nil {
        writeError(w, http.StatusNotFound, "account not found")
        return
    }

    if account.Suspended {
        writeError(w, http.StatusNotFound, "account not found")
        return
    }

    actorURL := fmt.Sprintf("https://%s/users/%s", h.deps.Config.InstanceDomain, account.Username)

    resp := webFingerResponse{
        Subject: resource,
        Aliases: []string{actorURL},
        Links: []webFingerLink{
            {
                Rel:  "self",
                Type: "application/activity+json",
                Href: actorURL,
            },
        },
    }

    // Cache WebFinger responses for 1 hour — the mapping is stable.
    w.Header().Set("Cache-Control", "max-age=3600")
    writeJRD(w, http.StatusOK, resp)
}
```

---

### 7b. `nodeinfo.go` — NodeInfo 2.0

NodeInfo requires two endpoints: a pointer at `/.well-known/nodeinfo` and the full document at `/nodeinfo/2.0`.

#### Schema Addendum — `CountLocalStatuses` Query

NodeInfo reports `usage.localPosts`. This needs a new `sqlc` query not present in the existing schema:

```sql
-- Added to internal/store/postgres/queries/statuses.sql

-- name: CountLocalStatuses :one
-- Count of local, non-deleted statuses — used for NodeInfo and instance stats.
SELECT COUNT(*) FROM statuses WHERE local = TRUE AND deleted_at IS NULL;
```

**Store interface addition** (in `StatusStore`):

```go
CountLocalStatuses(ctx context.Context) (int64, error)
```

#### Handler

```go
package activitypub

import (
    "fmt"
    "net/http"
)

// NodeInfoPointerHandler serves the well-known nodeinfo pointer document.
// GET /.well-known/nodeinfo
//
// This document tells discovery clients where to find the full NodeInfo
// document. It's a one-element array of links.
type NodeInfoPointerHandler struct {
    deps Deps
}

func NewNodeInfoPointerHandler(deps Deps) *NodeInfoPointerHandler {
    return &NodeInfoPointerHandler{deps: deps}
}

type nodeInfoPointerResponse struct {
    Links []nodeInfoPointerLink `json:"links"`
}

type nodeInfoPointerLink struct {
    Rel  string `json:"rel"`
    Href string `json:"href"`
}

func (h *NodeInfoPointerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    resp := nodeInfoPointerResponse{
        Links: []nodeInfoPointerLink{
            {
                Rel:  "http://nodeinfo.diaspora.software/ns/schema/2.0",
                Href: fmt.Sprintf("https://%s/nodeinfo/2.0", h.deps.Config.InstanceDomain),
            },
        },
    }

    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.Header().Set("Cache-Control", "max-age=1800")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(resp)
}

// NodeInfoHandler serves the full NodeInfo 2.0 document.
// GET /nodeinfo/2.0
//
// Dynamic fields (user count, post count) are queried on each request
// but could be cached with a short TTL in a future optimization.
type NodeInfoHandler struct {
    deps Deps
}

func NewNodeInfoHandler(deps Deps) *NodeInfoHandler {
    return &NodeInfoHandler{deps: deps}
}

type nodeInfoResponse struct {
    Version           string            `json:"version"`
    Software          nodeInfoSoftware  `json:"software"`
    Protocols         []string          `json:"protocols"`
    Usage             nodeInfoUsage     `json:"usage"`
    OpenRegistrations bool              `json:"openRegistrations"`
    Metadata          map[string]any    `json:"metadata"`
}

type nodeInfoSoftware struct {
    Name    string `json:"name"`
    Version string `json:"version"`
}

type nodeInfoUsage struct {
    Users      nodeInfoUsers `json:"users"`
    LocalPosts int64         `json:"localPosts"`
}

type nodeInfoUsers struct {
    Total int64 `json:"total"`
}

func (h *NodeInfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    userCount, err := h.deps.Store.CountLocalAccounts(ctx)
    if err != nil {
        h.deps.Logger.Error("nodeinfo: count accounts", "error", err)
        userCount = 0
    }

    postCount, err := h.deps.Store.CountLocalStatuses(ctx)
    if err != nil {
        h.deps.Logger.Error("nodeinfo: count statuses", "error", err)
        postCount = 0
    }

    // registration_mode from instance_settings: "open", "approval", "closed"
    regMode, _ := h.deps.Store.GetSetting(ctx, "registration_mode")
    openReg := regMode == "open"

    resp := nodeInfoResponse{
        Version: "2.0",
        Software: nodeInfoSoftware{
            Name:    "monstera-fed",
            Version: h.deps.Config.Version, // set at build time via -ldflags
        },
        Protocols: []string{"activitypub"},
        Usage: nodeInfoUsage{
            Users:      nodeInfoUsers{Total: userCount},
            LocalPosts: postCount,
        },
        OpenRegistrations: openReg,
        Metadata:          map[string]any{},
    }

    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.Header().Set("Cache-Control", "max-age=1800")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(resp)
}
```

**Configuration addition** — `Version` field in `config.Config`:

```go
// Version is the build version, injected via -ldflags at compile time.
// Example: go build -ldflags "-X main.version=0.1.0"
// The serve command passes it into Config during startup.
Version string // default: "0.0.0-dev"
```

---

### 7c. `actor.go` — AP Actor Document

```go
package activitypub

import (
    "fmt"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"

    "github.com/yourorg/monstera-fed/internal/ap"
)

// ActorHandler serves the AP Actor document for a local user.
// GET /users/{username}
//
// Always returns application/activity+json regardless of the Accept header.
// Monstera-fed has no HTML profile; all /users/:username requests serve AP JSON.
//
// Remote servers fetch this document to:
//  - Discover the user's inbox/outbox/followers/following URLs
//  - Obtain the user's RSA public key for HTTP Signature verification
//  - Display the user's profile summary, avatar, and display name
type ActorHandler struct {
    deps Deps
}

func NewActorHandler(deps Deps) *ActorHandler {
    return &ActorHandler{deps: deps}
}

func (h *ActorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    username := chi.URLParam(r, "username")
    domain := h.deps.Config.InstanceDomain

    account, err := h.deps.Store.GetLocalAccountByUsername(r.Context(), username)
    if err != nil {
        writeError(w, http.StatusNotFound, "actor not found")
        return
    }

    if account.Suspended {
        // Mastodon returns 410 Gone for suspended accounts.
        writeError(w, http.StatusGone, "account suspended")
        return
    }

    actorURL := fmt.Sprintf("https://%s/users/%s", domain, account.Username)

    actor := ap.Actor{
        Context:           ap.DefaultContext,
        ID:                actorURL,
        Type:              actorType(account.Bot),
        PreferredUsername: account.Username,
        Name:              safeDeref(account.DisplayName),
        Summary:           safeDeref(account.Note),
        Inbox:             fmt.Sprintf("%s/inbox", actorURL),
        Outbox:            fmt.Sprintf("%s/outbox", actorURL),
        Followers:         fmt.Sprintf("%s/followers", actorURL),
        Following:         fmt.Sprintf("%s/following", actorURL),
        URL:               actorURL,
        Published:         account.CreatedAt.UTC().Format(time.RFC3339),
        PublicKey: ap.PublicKey{
            ID:           actorURL + "#main-key",
            Owner:        actorURL,
            PublicKeyPem: account.PublicKey,
        },
        Endpoints: &ap.Endpoints{
            SharedInbox: fmt.Sprintf("https://%s/inbox", domain),
        },
        ManuallyApprovesFollowers: account.Locked,
        Featured:                  fmt.Sprintf("%s/collections/featured", actorURL),
    }

    // Avatar
    if account.AvatarMediaID != nil {
        attachment, err := h.deps.Store.GetMediaAttachment(r.Context(), *account.AvatarMediaID)
        if err == nil {
            actor.Icon = &ap.Icon{
                Type:      "Image",
                MediaType: mediaTypeFromURL(attachment.URL),
                URL:       attachment.URL,
            }
        }
    }

    // Header image
    if account.HeaderMediaID != nil {
        attachment, err := h.deps.Store.GetMediaAttachment(r.Context(), *account.HeaderMediaID)
        if err == nil {
            actor.Image = &ap.Icon{
                Type:      "Image",
                MediaType: mediaTypeFromURL(attachment.URL),
                URL:       attachment.URL,
            }
        }
    }

    w.Header().Set("Cache-Control", "max-age=180")
    writeJSON(w, http.StatusOK, actor)
}

// actorType returns "Service" for bot accounts, "Person" for regular users.
// Mastodon convention: bots are AP type "Service".
func actorType(bot bool) string {
    if bot {
        return "Service"
    }
    return "Person"
}

// mediaTypeFromURL infers the MIME type from a URL's file extension.
func mediaTypeFromURL(url string) string {
    switch {
    case strings.HasSuffix(url, ".png"):
        return "image/png"
    case strings.HasSuffix(url, ".gif"):
        return "image/gif"
    case strings.HasSuffix(url, ".webp"):
        return "image/webp"
    default:
        return "image/jpeg"
    }
}
```

**Vocab type addition** — `Actor` needs `Featured`, `URL`, and `Image` fields (verifying these are in vocab.go from Stage 1):

```go
// Added to the Actor struct in internal/ap/vocab.go if not already present:
Featured string  `json:"featured,omitempty"`
URL      string  `json:"url,omitempty"`
Image    *Icon   `json:"image,omitempty"`
```

---

### 7d. `outbox.go` — AP Outbox Collection

```go
package activitypub

import (
    "fmt"
    "net/http"
    "strconv"
    "time"

    "github.com/go-chi/chi/v5"

    "github.com/yourorg/monstera-fed/internal/ap"
)

// OutboxHandler serves the AP Outbox for a local user.
// GET /users/{username}/outbox
//
// Returns an OrderedCollection. If the "page" query parameter is present
// and "true", returns an OrderedCollectionPage containing the most recent
// public statuses wrapped as Create{Note} activities.
//
// Only public statuses are included — unlisted, private, and direct are
// excluded from the outbox for privacy. This matches Mastodon's behavior.
//
// Pagination uses the "max_id" query parameter (ULID cursor).
type OutboxHandler struct {
    deps Deps
}

func NewOutboxHandler(deps Deps) *OutboxHandler {
    return &OutboxHandler{deps: deps}
}

const outboxPageSize = 20

func (h *OutboxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    username := chi.URLParam(r, "username")
    domain := h.deps.Config.InstanceDomain
    ctx := r.Context()

    account, err := h.deps.Store.GetLocalAccountByUsername(ctx, username)
    if err != nil {
        writeError(w, http.StatusNotFound, "actor not found")
        return
    }

    if account.Suspended {
        writeError(w, http.StatusGone, "account suspended")
        return
    }

    outboxURL := fmt.Sprintf("https://%s/users/%s/outbox", domain, account.Username)

    // If no "page" param, return the collection root with totalItems.
    if r.URL.Query().Get("page") != "true" {
        // Count public statuses for this account.
        statuses, err := h.deps.Store.GetAccountStatuses(ctx, account.ID, "", outboxPageSize)
        if err != nil {
            writeError(w, http.StatusInternalServerError, "internal server error")
            return
        }

        // We don't have a dedicated count query for public-only account statuses,
        // so totalItems is approximate (shows total non-boost statuses).
        totalItems := len(statuses) // minimum items — accurate count is Phase 2

        collection := ap.OrderedCollection{
            Context:    ap.DefaultContext,
            ID:         outboxURL,
            Type:       "OrderedCollection",
            TotalItems: totalItems,
            First:      outboxURL + "?page=true",
        }

        writeJSON(w, http.StatusOK, collection)
        return
    }

    // Paginated collection page.
    maxID := r.URL.Query().Get("max_id")
    statuses, err := h.deps.Store.GetAccountStatuses(ctx, account.ID, maxID, int32(outboxPageSize))
    if err != nil {
        writeError(w, http.StatusInternalServerError, "internal server error")
        return
    }

    // Filter to public-only in application code.
    var items []any
    for _, s := range statuses {
        if s.Visibility != "public" {
            continue
        }

        note := ap.Note{
            ID:           s.ApID,
            Type:         "Note",
            AttributedTo: account.ApID,
            Content:      safeDeref(s.Content),
            Published:    s.CreatedAt.UTC().Format(time.RFC3339),
            URL:          fmt.Sprintf("https://%s/@%s/%s", domain, account.Username, s.ID),
            Sensitive:    s.Sensitive,
            To:           []string{ap.PublicAddress},
            Cc:           []string{account.FollowersUrl},
        }

        if s.ContentWarning != nil && *s.ContentWarning != "" {
            note.Summary = s.ContentWarning
        }

        activity := ap.WrapInCreate(s.ApID+"/activity", &note)
        items = append(items, activity)
    }

    pageID := outboxURL + "?page=true"
    if maxID != "" {
        pageID += "&max_id=" + maxID
    }

    page := ap.OrderedCollectionPage{
        Context:      ap.DefaultContext,
        ID:           pageID,
        Type:         "OrderedCollectionPage",
        PartOf:       outboxURL,
        OrderedItems: items,
    }

    // Build "next" link if we have a full page of results.
    if len(statuses) == outboxPageSize {
        lastID := statuses[len(statuses)-1].ID
        page.Next = outboxURL + "?page=true&max_id=" + lastID
    }

    writeJSON(w, http.StatusOK, page)
}
```

---

### 7e. `collections.go` — Followers, Following, Featured

```go
package activitypub

import (
    "fmt"
    "net/http"

    "github.com/go-chi/chi/v5"

    "github.com/yourorg/monstera-fed/internal/ap"
)

// FollowersHandler serves the followers collection for a local user.
// GET /users/{username}/followers
//
// Returns an OrderedCollection with only totalItems — individual followers
// are NOT enumerated for privacy. Mastodon does the same: remote servers can
// see the count but not iterate the list. Local followers are served through
// the REST API with proper authentication.
type FollowersHandler struct {
    deps Deps
}

func NewFollowersHandler(deps Deps) *FollowersHandler {
    return &FollowersHandler{deps: deps}
}

func (h *FollowersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    username := chi.URLParam(r, "username")
    domain := h.deps.Config.InstanceDomain
    ctx := r.Context()

    account, err := h.deps.Store.GetLocalAccountByUsername(ctx, username)
    if err != nil {
        writeError(w, http.StatusNotFound, "actor not found")
        return
    }

    if account.Suspended {
        writeError(w, http.StatusGone, "account suspended")
        return
    }

    count, err := h.deps.Store.CountFollowers(ctx, account.ID)
    if err != nil {
        h.deps.Logger.Error("followers: count", "error", err)
        count = 0
    }

    collection := ap.OrderedCollection{
        Context:    ap.DefaultContext,
        ID:         fmt.Sprintf("https://%s/users/%s/followers", domain, account.Username),
        Type:       "OrderedCollection",
        TotalItems: int(count),
    }

    w.Header().Set("Cache-Control", "max-age=180")
    writeJSON(w, http.StatusOK, collection)
}

// FollowingHandler serves the following collection for a local user.
// GET /users/{username}/following
//
// Same privacy model as followers: totalItems only, no enumeration.
type FollowingHandler struct {
    deps Deps
}

func NewFollowingHandler(deps Deps) *FollowingHandler {
    return &FollowingHandler{deps: deps}
}

func (h *FollowingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    username := chi.URLParam(r, "username")
    domain := h.deps.Config.InstanceDomain
    ctx := r.Context()

    account, err := h.deps.Store.GetLocalAccountByUsername(ctx, username)
    if err != nil {
        writeError(w, http.StatusNotFound, "actor not found")
        return
    }

    if account.Suspended {
        writeError(w, http.StatusGone, "account suspended")
        return
    }

    count, err := h.deps.Store.CountFollowing(ctx, account.ID)
    if err != nil {
        h.deps.Logger.Error("following: count", "error", err)
        count = 0
    }

    collection := ap.OrderedCollection{
        Context:    ap.DefaultContext,
        ID:         fmt.Sprintf("https://%s/users/%s/following", domain, account.Username),
        Type:       "OrderedCollection",
        TotalItems: int(count),
    }

    w.Header().Set("Cache-Control", "max-age=180")
    writeJSON(w, http.StatusOK, collection)
}

// FeaturedHandler serves the featured (pinned statuses) collection.
// GET /users/{username}/collections/featured
//
// Phase 1: returns an empty OrderedCollection. This prevents remote servers
// from receiving a 404 when they fetch the featured collection URL advertised
// in the Actor document. Pinned posts are a Phase 2 feature.
type FeaturedHandler struct {
    deps Deps
}

func NewFeaturedHandler(deps Deps) *FeaturedHandler {
    return &FeaturedHandler{deps: deps}
}

func (h *FeaturedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    username := chi.URLParam(r, "username")
    domain := h.deps.Config.InstanceDomain

    account, err := h.deps.Store.GetLocalAccountByUsername(r.Context(), username)
    if err != nil {
        writeError(w, http.StatusNotFound, "actor not found")
        return
    }

    if account.Suspended {
        writeError(w, http.StatusGone, "account suspended")
        return
    }

    collection := ap.OrderedCollection{
        Context:      ap.DefaultContext,
        ID:           fmt.Sprintf("https://%s/users/%s/collections/featured", domain, account.Username),
        Type:         "OrderedCollection",
        TotalItems:   0,
        OrderedItems: []any{},
    }

    w.Header().Set("Cache-Control", "max-age=180")
    writeJSON(w, http.StatusOK, collection)
}
```

---

### 7f. `inbox.go` — Inbox HTTP Handler

```go
package activitypub

import (
    "encoding/json"
    "io"
    "net/http"
    "strings"

    "github.com/go-chi/chi/v5"

    "github.com/yourorg/monstera-fed/internal/ap"
)

// InboxHandler processes incoming AP activities.
// POST /users/{username}/inbox  (per-user inbox)
// POST /inbox                   (shared inbox)
//
// Processing pipeline:
//  1. Validate Content-Type (application/activity+json or application/ld+json).
//  2. Read and buffer the request body (needed for HTTP Signature digest verification).
//  3. Verify the HTTP Signature via ap.Verify. This authenticates the sender
//     and proves the body has not been tampered with.
//  4. Parse the activity JSON.
//  5. Validate that the activity's actor matches the key owner (key attribution check).
//  6. Dispatch to InboxProcessor.Process.
//  7. Return 202 Accepted.
//
// Error responses:
//  - 400: malformed JSON, missing Content-Type, missing activity fields
//  - 401: invalid or missing HTTP Signature
//  - 403: domain blocked (suspended)
//  - 202: accepted (even if processing encounters a non-fatal error)
type InboxHandler struct {
    deps Deps
}

func NewInboxHandler(deps Deps) *InboxHandler {
    return &InboxHandler{deps: deps}
}

// maxInboxBodySize prevents abuse by limiting how much data we'll read.
const maxInboxBodySize = 256 * 1024 // 256 KB

func (h *InboxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    logger := h.deps.Logger

    // 1. Validate Content-Type.
    ct := r.Header.Get("Content-Type")
    if !isActivityPubContentType(ct) {
        writeError(w, http.StatusBadRequest, "Content-Type must be application/activity+json or application/ld+json")
        return
    }

    // 2. Read body (ap.Verify also needs the raw body for digest verification,
    // and it replaces r.Body with a new reader after reading).
    r.Body = http.MaxBytesReader(w, r.Body, maxInboxBodySize)

    // 3. Verify HTTP Signature.
    // ap.Verify reads the body for digest verification and restores r.Body.
    keyID, err := ap.Verify(ctx, r, ap.DefaultKeyFetcher(h.deps.Cache), h.deps.Cache)
    if err != nil {
        logger.Debug("inbox: signature verification failed", "error", err)
        writeError(w, http.StatusUnauthorized, "invalid HTTP Signature")
        return
    }

    // 4. Read the (already-verified) body for activity parsing.
    body, err := io.ReadAll(r.Body)
    if err != nil {
        writeError(w, http.StatusBadRequest, "failed to read request body")
        return
    }

    var activity ap.Activity
    if err := json.Unmarshal(body, &activity); err != nil {
        logger.Debug("inbox: malformed activity JSON", "error", err)
        writeError(w, http.StatusBadRequest, "malformed activity JSON")
        return
    }

    // Basic validation.
    if activity.ID == "" || activity.Type == "" || activity.Actor == "" {
        writeError(w, http.StatusBadRequest, "activity must have id, type, and actor")
        return
    }

    // 5. Key attribution check: the key's owner domain must match the activity's
    // actor domain. This prevents a compromised key on server A from forging
    // activities as a user on server B.
    keyDomain := ap.DomainFromKeyID(keyID)
    actorDomain := ap.DomainFromActorID(activity.Actor)
    if keyDomain != actorDomain {
        logger.Warn("inbox: key attribution mismatch",
            "key_domain", keyDomain, "actor_domain", actorDomain)
        writeError(w, http.StatusUnauthorized, "key attribution mismatch")
        return
    }

    // 6. Domain block check (before spending resources on processing).
    if suspended, _ := h.deps.Blocklist.IsSuspended(ctx, actorDomain); suspended {
        logger.Debug("inbox: activity from suspended domain",
            "domain", actorDomain, "type", activity.Type)
        // Return 202 to avoid leaking block status to the sender.
        w.WriteHeader(http.StatusAccepted)
        return
    }

    // 7. Dispatch to InboxProcessor (synchronous in Phase 1).
    if err := h.deps.Inbox.Process(ctx, &activity); err != nil {
        if _, ok := err.(*ap.PermanentError); ok {
            logger.Warn("inbox: permanent processing error",
                "type", activity.Type, "actor", activity.Actor, "error", err)
        } else {
            logger.Error("inbox: processing error",
                "type", activity.Type, "actor", activity.Actor, "error", err)
        }
        // Return 202 regardless — don't leak internal errors to remote servers,
        // and don't cause them to retry.
    }

    w.WriteHeader(http.StatusAccepted)
}

// isActivityPubContentType checks if the Content-Type is valid for AP.
// Mastodon sends "application/activity+json" but the spec also permits
// 'application/ld+json; profile="https://www.w3.org/ns/activitystreams"'.
func isActivityPubContentType(ct string) bool {
    ct = strings.ToLower(strings.TrimSpace(ct))
    if strings.HasPrefix(ct, "application/activity+json") {
        return true
    }
    if strings.HasPrefix(ct, "application/ld+json") {
        return true
    }
    return false
}
```

---

### Handler Summary Table

| File | Endpoint | Content-Type | Cache | Auth |
|------|----------|-------------|-------|------|
| `webfinger.go` | `GET /.well-known/webfinger` | `application/jrd+json` | `max-age=3600` | None |
| `nodeinfo.go` | `GET /.well-known/nodeinfo` | `application/json` | `max-age=1800` | None |
| `nodeinfo.go` | `GET /nodeinfo/2.0` | `application/json` | `max-age=1800` | None |
| `actor.go` | `GET /users/{username}` | `application/activity+json` | `max-age=180` | None |
| `outbox.go` | `GET /users/{username}/outbox` | `application/activity+json` | None | None |
| `collections.go` | `GET /users/{username}/followers` | `application/activity+json` | `max-age=180` | None |
| `collections.go` | `GET /users/{username}/following` | `application/activity+json` | `max-age=180` | None |
| `collections.go` | `GET /users/{username}/collections/featured` | `application/activity+json` | `max-age=180` | None |
| `inbox.go` | `POST /users/{username}/inbox` | N/A (accepts `activity+json`) | None | HTTP Signature |
| `inbox.go` | `POST /inbox` | N/A (accepts `activity+json`) | None | HTTP Signature |

---

## 8. Response & Activity JSON Examples

### 8a. WebFinger Response

`GET /.well-known/webfinger?resource=acct:alice@monstera-fed.social`

```json
{
  "subject": "acct:alice@monstera-fed.social",
  "aliases": [
    "https://monstera-fed.social/users/alice"
  ],
  "links": [
    {
      "rel": "self",
      "type": "application/activity+json",
      "href": "https://monstera-fed.social/users/alice"
    }
  ]
}
```

### 8b. NodeInfo 2.0 Response

`GET /nodeinfo/2.0`

```json
{
  "version": "2.0",
  "software": {
    "name": "monstera-fed",
    "version": "0.1.0"
  },
  "protocols": ["activitypub"],
  "usage": {
    "users": {
      "total": 142
    },
    "localPosts": 8931
  },
  "openRegistrations": false,
  "metadata": {}
}
```

### 8c. AP Actor

`GET /users/alice` — `Content-Type: application/activity+json`

```json
{
  "@context": [
    "https://www.w3.org/ns/activitystreams",
    "https://w3id.org/security/v1",
    {
      "manuallyApprovesFollowers": "as:manuallyApprovesFollowers",
      "sensitive": "as:sensitive",
      "Hashtag": "as:Hashtag",
      "toot": "http://joinmastodon.org/ns#",
      "Emoji": "toot:Emoji",
      "featured": { "@id": "toot:featured", "@type": "@id" },
      "featuredTags": { "@id": "toot:featuredTags", "@type": "@id" }
    }
  ],
  "id": "https://monstera-fed.social/users/alice",
  "type": "Person",
  "preferredUsername": "alice",
  "name": "Alice",
  "summary": "<p>Plant enthusiast and software developer.</p>",
  "inbox": "https://monstera-fed.social/users/alice/inbox",
  "outbox": "https://monstera-fed.social/users/alice/outbox",
  "followers": "https://monstera-fed.social/users/alice/followers",
  "following": "https://monstera-fed.social/users/alice/following",
  "url": "https://monstera-fed.social/users/alice",
  "featured": "https://monstera-fed.social/users/alice/collections/featured",
  "published": "2026-01-15T10:30:00Z",
  "manuallyApprovesFollowers": false,
  "publicKey": {
    "id": "https://monstera-fed.social/users/alice#main-key",
    "owner": "https://monstera-fed.social/users/alice",
    "publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhki...\n-----END PUBLIC KEY-----\n"
  },
  "endpoints": {
    "sharedInbox": "https://monstera-fed.social/inbox"
  },
  "icon": {
    "type": "Image",
    "mediaType": "image/png",
    "url": "https://monstera-fed.social/media/avatars/alice.png"
  }
}
```

### 8d. Create{Note}

Sent when a local user publishes a public status.

```json
{
  "@context": [
    "https://www.w3.org/ns/activitystreams",
    "https://w3id.org/security/v1",
    {
      "manuallyApprovesFollowers": "as:manuallyApprovesFollowers",
      "sensitive": "as:sensitive",
      "Hashtag": "as:Hashtag",
      "toot": "http://joinmastodon.org/ns#",
      "Emoji": "toot:Emoji",
      "featured": { "@id": "toot:featured", "@type": "@id" },
      "featuredTags": { "@id": "toot:featuredTags", "@type": "@id" }
    }
  ],
  "id": "https://monstera-fed.social/activities/01JMQR3K5V8X2WFGN4HTBA9YCD",
  "type": "Create",
  "actor": "https://monstera-fed.social/users/alice",
  "published": "2026-02-24T14:30:00Z",
  "to": ["https://www.w3.org/ns/activitystreams#Public"],
  "cc": ["https://monstera-fed.social/users/alice/followers"],
  "object": {
    "id": "https://monstera-fed.social/users/alice/statuses/01JMQR3K5V8X2WFGN4HTBA9YAB",
    "type": "Note",
    "attributedTo": "https://monstera-fed.social/users/alice",
    "content": "<p>Just repotted my monstera-fed deliciosa. The root ball was enormous! 🌿</p>",
    "contentMap": {
      "en": "<p>Just repotted my monstera-fed deliciosa. The root ball was enormous! 🌿</p>"
    },
    "source": {
      "content": "Just repotted my monstera-fed deliciosa. The root ball was enormous! 🌿",
      "mediaType": "text/plain"
    },
    "to": ["https://www.w3.org/ns/activitystreams#Public"],
    "cc": ["https://monstera-fed.social/users/alice/followers"],
    "published": "2026-02-24T14:30:00Z",
    "url": "https://monstera-fed.social/@alice/01JMQR3K5V8X2WFGN4HTBA9YAB",
    "sensitive": false,
    "summary": null,
    "tag": [
      {
        "type": "Hashtag",
        "href": "https://monstera-fed.social/tags/plants",
        "name": "#plants"
      }
    ],
    "attachment": [
      {
        "type": "Document",
        "mediaType": "image/jpeg",
        "url": "https://monstera-fed.social/media/statuses/monstera-fed-repot.jpg",
        "name": "A large monstera-fed plant freshly repotted in a terracotta pot"
      }
    ]
  }
}
```

### 8e. Follow

Sent when a local user follows a remote user.

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://monstera-fed.social/activities/01JMQR4N7HZXK9PQ3YWMTC6EFG",
  "type": "Follow",
  "actor": "https://monstera-fed.social/users/alice",
  "object": "https://mastodon.example.com/users/bob"
}
```

### 8f. Accept{Follow}

Sent when a local user (or auto-accept logic) accepts an incoming follow.

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://monstera-fed.social/activities/01JMQR5QA2BFD8VNXR7KSJP4HJ",
  "type": "Accept",
  "actor": "https://monstera-fed.social/users/alice",
  "object": {
    "id": "https://mastodon.example.com/activities/abc123",
    "type": "Follow",
    "actor": "https://mastodon.example.com/users/bob",
    "object": "https://monstera-fed.social/users/alice"
  }
}
```

### 8g. Undo{Follow}

Sent when a local user unfollows a remote user.

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://monstera-fed.social/activities/01JMQR6T3CKGE9WPYS8LNUF5KM",
  "type": "Undo",
  "actor": "https://monstera-fed.social/users/alice",
  "object": {
    "id": "https://monstera-fed.social/activities/01JMQR4N7HZXK9PQ3YWMTC6EFG",
    "type": "Follow",
    "actor": "https://monstera-fed.social/users/alice",
    "object": "https://mastodon.example.com/users/bob"
  }
}
```

### 8h. Announce (Boost)

Sent when a local user boosts a remote status.

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://monstera-fed.social/activities/01JMQR7W5DLHFAXQZT9MPVG6NP",
  "type": "Announce",
  "actor": "https://monstera-fed.social/users/alice",
  "published": "2026-02-24T15:10:00Z",
  "to": ["https://www.w3.org/ns/activitystreams#Public"],
  "cc": ["https://monstera-fed.social/users/alice/followers"],
  "object": "https://mastodon.example.com/users/bob/statuses/109876543210"
}
```

### 8i. Like

Sent when a local user favourites a remote status.

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://monstera-fed.social/activities/01JMQR8Y7EMJGBYRSV0NQWH7QR",
  "type": "Like",
  "actor": "https://monstera-fed.social/users/alice",
  "object": "https://mastodon.example.com/users/bob/statuses/109876543210"
}
```

### 8j. Delete

Sent when a local user deletes their own status.

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "id": "https://monstera-fed.social/activities/01JMQR9Z8FNKHCZSTW1PRXI8ST",
  "type": "Delete",
  "actor": "https://monstera-fed.social/users/alice",
  "object": {
    "id": "https://monstera-fed.social/users/alice/statuses/01JMQR3K5V8X2WFGN4HTBA9YAB",
    "type": "Tombstone"
  }
}
```

---

## 9. Open Questions

| # | Question | Recommendation | Impact if deferred |
|---|----------|---------------|-------------------|
| 1 | **Shared inbox delivery optimization** — Phase 1 deduplicates by raw `inbox_url`. True shared-inbox optimization requires storing and querying `shared_inbox_url` on the `accounts` table, then coalescing all followers on the same server into a single delivery to the shared inbox. | Add `shared_inbox_url TEXT` column to `accounts` in Phase 2. Update `GetFollowerInboxURLs` to `COALESCE(a.shared_inbox_url, a.inbox_url)` and `DISTINCT` the result. | Low — most Mastodon instances already use the same `inbox_url` for all users (the shared inbox), so deduplication by URL catches the majority of cases. The optimization mainly helps with Pleroma/Akkoma instances that use per-user inboxes. |
| 2 | **Inbox async processing** — Phase 1 processes activities synchronously on the HTTP handler goroutine. High-traffic instances may see slow inbox responses under load (e.g., a `Create{Note}` that resolves a remote account triggers an outbound HTTP fetch). | Phase 2: add a bounded goroutine pool (e.g., `errgroup` with `SetLimit(50)`) that the inbox handler submits work to. Return 202 immediately; process in background. Alternatively, enqueue to a NATS `INBOX_PROCESSING` stream. | Medium — only affects instances receiving high volumes of remote activities. The HTTP Signature verification alone takes most of the request time; the processing step is typically fast. |
| 3 | **Remote media lazy-fetch** — Phase 1 stores `remote_url` on media attachments without fetching. Clients must proxy through the remote URL, which has privacy and availability implications. | Phase 2: background job that fetches remote media on first access (or on ingest), stores it locally/in S3, and updates the `url` column. Use a NATS `media.fetch` stream with the same worker pattern as federation delivery. | Medium — functional but suboptimal. Remote media URLs may go stale or be slow. Mastodon fetches immediately on ingest. |
| 4 | **410 Gone inbox cleanup** — The federation worker logs 410 responses but doesn't clean up follow relationships. A remote server returning 410 usually means the account or entire instance is gone. | Phase 2: on 410, look up all `follows` targeting inboxes on that domain and soft-delete them. Optionally mark the domain as "gone" to prevent future delivery attempts. | Low — stale follows consume minimal resources (a few bytes in the DB and occasional failed deliveries that quickly exhaust retries). |
| ~~5~~ | ~~**HTTP Signature key rotation retry**~~ — resolved: **implement in Phase 1**. On `Verify` failure, evict the cached public key, re-fetch from the remote actor's AP endpoint, and retry verification once. ~15 lines in `ap.Verify`. | N/A |
| 6 | **NodeInfo `usage.users.activeMonth` / `usage.users.activeHalfyear`** — NodeInfo 2.0 supports optional active user counts. Mastodon reports these; some aggregation sites use them. | Phase 2: add a query that counts users with at least one status in the last 30/180 days, or track last-activity timestamps on the `users` table. | Low — purely informational. Does not affect federation. |
| 7 | **Outbox `totalItems` accuracy** — The outbox handler currently returns `len(statuses)` from the first page query, not a true count of all public statuses. | Phase 2: add a `CountPublicAccountStatuses(ctx, accountID)` query. The index `idx_statuses_account` supports this efficiently with an additional `visibility = 'public'` filter. | Low — `totalItems` is informational. Remote servers use pagination (`next` links), not the count, to traverse the collection. |

---

## 10. File Layout Summary

```
internal/ap/
├── vocab.go          -- AP/AS2 vocabulary types, constants, helpers
├── httpsig.go        -- HTTP Signature sign/verify (ADR 06)
├── inbox.go          -- InboxProcessor: dispatch + per-activity-type handlers
├── outbox.go         -- OutboxPublisher: create activities, fan-out, enqueue
└── blocklist.go      -- BlocklistCache: domain block enforcement

internal/nats/
├── client.go         -- NATS connection setup
└── federation/
    ├── producer.go   -- EnqueueDelivery, EnsureStreams, DLQ
    └── worker.go     -- Pull consumer, HTTP delivery, retry, signing

internal/api/activitypub/
├── deps.go           -- Deps struct, writeJSON/writeJRD helpers
├── webfinger.go      -- GET /.well-known/webfinger
├── nodeinfo.go       -- GET /.well-known/nodeinfo + GET /nodeinfo/2.0
├── actor.go          -- GET /users/{username}
├── outbox.go         -- GET /users/{username}/outbox
├── collections.go    -- GET /users/{username}/followers|following|collections/featured
└── inbox.go          -- POST /users/{username}/inbox + POST /inbox
```

---

## 11. Schema Addenda Summary

Queries and migrations added by this design output, beyond what exists in ADR 02:

| Addition | Location | Purpose |
|----------|----------|---------|
| `000024_add_ap_id_to_favourites.up.sql` | Migration | `ALTER TABLE favourites ADD COLUMN ap_id TEXT UNIQUE` |
| `GetFavouriteByAPID` | `favourites.sql` | Look up favourite by ActivityPub ID for incoming `Undo{Like}` |
| `GetFavouriteByAccountAndStatus` | `favourites.sql` | Fallback lookup for `Undo{Like}` when `ap_id` is not available |
| `CountLocalStatuses` | `statuses.sql` | Count local non-deleted statuses for NodeInfo |
| `FederationWorkerConcurrency` | `config.go` | Worker goroutine count (default 5) |
| `Version` | `config.go` | Build version string for NodeInfo |
