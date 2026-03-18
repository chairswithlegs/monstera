# ActivityPub Package

Federation logic — inbound processing, outbound delivery, remote account resolution, and HTTP Signatures.

## Package layout

```
activitypub/
├── inbox.go            # Inbox interface + dispatcher; routes activities to type-specific handlers
├── inbox_follow.go     # Follow, Accept, Reject, Block handlers
├── inbox_status.go     # Create, Update, Delete, Announce, Like handlers
├── inbox_undo.go       # Undo handlers (Undo Follow, Undo Like, Undo Announce, Undo Block)
├── federation_subscriber.go  # Consumes domain events → builds AP activities → sends to outbox workers
├── remote_resolver.go  # Resolves remote actors via WebFinger + Actor fetch; SyncActorToStore
├── httpsignature.go    # HTTP Signature creation and verification
├── streams.go          # NATS stream/subject definitions for federation
├── vocab/              # AP type definitions and domain↔AP conversion functions
│   ├── vocab.go        # Base types (Object, ObjectType, PublicKey, Tag, Attachment, etc.)
│   ├── actor.go        # Actor struct, ActorToRemoteFields, PropertyValue parsing
│   ├── note.go         # Note struct, LocalStatusToNote, NoteVisibility, NoteStatusFields
│   ├── activity.go     # Activity struct, NewCreateActivity, NewAnnounceActivity, NewLikeActivity, etc.
│   ├── collection.go   # OrderedCollection/Page types
│   └── utils.go        # DomainFromIRI and other helpers
└── internal/           # Outbox delivery workers (not imported outside this package)
    ├── outbox_fanout_worker.go   # Fans out activities to follower inboxes
    ├── outbox_delivery_worker.go # Delivers signed activities to remote inboxes
    └── streams.go                # NATS stream/subject definitions for outbox queues
```

## Inbox conventions

The inbox is a pure translation layer — it maps AP activities to service method calls. It does not contain business logic.

- **Own-domain rejection**: Activities from the instance's own domain are rejected with `ErrInboxFatal` to prevent spoofing.
- **Blocklist check**: Blocked domains are rejected before processing.
- **Remote methods**: Inbox handlers call `*Remote` service methods (`CreateRemote`, `CreateRemoteReblog`, `SuspendRemote`, `CreateRemoteBlock`, etc.), never generic methods that assume a local actor.
- **Actor resolution**: Unknown actors are resolved via `RemoteAccountResolver` which fetches, sanitizes, and stores the Actor document.

## Federation subscriber conventions

The federation subscriber listens to the `DOMAIN_EVENTS` NATS stream and translates domain events into AP activities for outbound delivery.

- **Locality checks**: Use `Account.Domain != nil` (remote) or `Account.Domain == nil` (local). Never use `InboxURL == ""` as a locality proxy.
- **Activity construction**: Use `vocab.New*Activity` constructors from the vocab package.
- **Delivery**: Activities go through fanout (resolves follower lists) then delivery (signs and POSTs to inboxes).

## Vocab package conventions

The `vocab/` subpackage owns all AP type definitions and all conversion logic between domain types and AP types.

- **Inbound**: `ActorToRemoteFields(actor) → RemoteActorFields` extracts stored fields from a fetched Actor document. `NoteStatusFields(note)` extracts stored fields from a fetched Note. The caller (resolver or inbox) is responsible for sanitizing input before calling these.
- **Outbound**: `LocalStatusToNote(input) → (*Note, error)` builds an AP Note from a local domain Status (author must have `Account.Domain == nil`). `NewCreateActivity(...)`, `NewAnnounceActivity(...)`, `NewLikeActivity(...)` wrap objects in activities.
- **To/Cc addressing**: `LocalStatusToNote` sets correct To/Cc based on visibility (Public, Unlisted, Followers-only, Direct). Public addresses include the aliased form `as:Public`.
- **PropertyValue**: Actor profile metadata fields are parsed from `Actor.Attachment` entries with `Type == "PropertyValue"`. The `verified_at` field is not stored — only the originating server knows verification state.
- **ContentMap**: Outbound Notes include `ContentMap` when the status has a language set, for language-aware rendering.
- **Source**: Inbound Notes use `note.Source.Content` (plain text) over `note.Content` (HTML) for the domain `Text` field.

## Remote resolver conventions

`RemoteAccountResolver.SyncActorToStore` is the single path for creating or updating remote accounts from Actor documents. It:

1. Sanitizes username (strict), display name and note (UGC policy)
2. Extracts fields via `vocab.ActorToRemoteFields`
3. Fetches follower/following/status counts from collection endpoints
4. Calls `AccountService.CreateOrUpdateRemoteAccount`

Stale accounts (not refreshed within the configured duration) are re-fetched on access. If the fetch fails, the stale account is returned with a warning log.

## HTTP Signature conventions

- Outbound: Deliveries are signed with the sending actor's RSA private key using `(request-target) host date digest` headers.
- Inbound: The inbox HTTP handler verifies signatures before passing to `Inbox.Process`. Verification fetches the actor's public key from the key ID URL.
