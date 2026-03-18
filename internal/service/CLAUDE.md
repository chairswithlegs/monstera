# Service Layer Conventions

When building or editing code in `internal/service`, follow these practices.

## Caller-agnostic naming

- Method names describe the **domain operation**, not who calls them.
- Use `Local` / `Remote` suffixes to distinguish lifecycle differences (e.g. `CreateLocal`, `CreateRemote`, `DeleteRemote`).
- Do **not** use caller-specific names such as `FromInbox`, `FromAPI`, or `ForFederation`.

## Domain terminology only

- Use the terms from `internal/domain`: **reblog**, **favourite**, **status**, **account**, **follow**.
- Do **not** use external/protocol terms in the service layer: **boost**, **announce**, **like**, **note**, **actor**, **toot**, **post**.
- External terms belong at the caller boundary (inbox, API handlers). Callers that translate external → domain concepts should add a brief mapping comment (e.g. `// AP Announce -> domain reblog`).

## Local vs remote convention

- The base verb (`Create`, `Update`, `Delete`) is the **local** path: increments counts, sends notifications.
- The `*Remote` variant handles federated-in objects: different count semantics (e.g. no account statuses count increment for remote statuses).
- **Both local and remote methods should emit domain events.** It is up to consumers (e.g. the federation subscriber, notification subscriber, SSE subscriber) to determine what actions to take based on locality.

## Interface size discipline

- Keep interfaces focused. Avoid methods that are a thin store pass-through used by only one caller, prefer folding it into a higher-level operation or keep it as a private implementation detail.

## Input types

- Use dedicated input structs per operation (e.g. `CreateStatusInput`, `CreateRemoteStatusInput`, `CreateWithContentInput`) if the number of arguments would otherwise exceed 3.
- Do not overload a single input struct with fields that only apply to one code path.

## Service decomposition

Large services should be split when they mix non-overlapping concerns. The status and follow domains demonstrate the pattern:

- **StatusWriteService** — local status CRUD (Create, Update, Delete, UpdateQuoteApprovalPolicy, RevokeQuote)
- **RemoteStatusWriteService** — remote status operations from federation (CreateRemote, UpdateRemote, DeleteRemote, CreateRemoteReblog, DeleteRemoteReblog, CreateRemoteFavourite, DeleteRemoteFavourite)
- **StatusInteractionService** — user-initiated interactions (CreateReblog, DeleteReblog, CreateFavourite, DeleteFavourite, Bookmark, Unbookmark, Pin, Unpin, RecordVote)
- **ScheduledStatusService** — scheduled status management (CreateScheduledStatus, UpdateScheduledStatus, DeleteScheduledStatus, PublishDueStatuses)
- **FollowService** — local follow/unfollow, block, mute, and relationship queries
- **RemoteFollowService** — remote follow operations from federation (CreateRemoteFollow, AcceptFollow, DeleteRemoteFollow, CreateRemoteBlock, HasLocalFollower, GetFollowerInboxURLsPaginated)
- **TagFollowService** — hashtag follow/unfollow (FollowTag, UnfollowTag, ListFollowedTags)

## Centralized enrichment

`StatusService.EnrichStatuses(ctx, statuses, opts)` is the canonical way to hydrate statuses with accounts, mentions, tags, media, and viewer-relative flags. Services that need enriched statuses should delegate to it rather than duplicating enrichment logic. Use `EnrichOpts` to control optional fields (IncludeCard, IncludePoll, ViewerID).

## Local/remote safety guards

Methods that only apply to remote entities must call `requireRemote(local, "MethodName")` at the top. Methods that only apply to local entities should call `requireLocal(local, "MethodName")` when the locality is determined by a caller-supplied input (e.g. a status fetched from the store). If the method itself explicitly sets locality (e.g. `Local: true` in a create path), the guard is unnecessary. These return `domain.ErrForbidden` if the guard fails.
