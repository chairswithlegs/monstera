# Remote data hydration

This document describes how Monstera proactively fetches data from remote servers. It covers the triggers, what data is fetched, and the limitations. It does not cover inbox processing, where remote servers push data to Monstera directly (see [03-activitypub-implementation.md](03-activitypub-implementation.md)).

## Overview

Monstera learns about remote accounts, statuses, and relationships through two mechanisms: inbound federation (inbox) and proactive fetching. Proactive fetching is triggered by local user actions and handled asynchronously by the **backfill worker**. Separately, link preview cards are fetched for all new statuses, and stale remote accounts are re-fetched on access.

## Backfill triggers

Four user actions cause Monstera to request a backfill for a remote account:

| Trigger | Location |
|---------|----------|
| Viewing a remote account's profile (`GET /api/v1/accounts/:id`) | `internal/api/mastodon/accounts.go` |
| Viewing a remote account's statuses (`GET /api/v1/accounts/:id/statuses`) | `internal/api/mastodon/accounts.go` |
| A local user follows a remote account | `internal/service/follow_service.go` |
| Search with `resolve=true` and a `user@domain` query | `internal/service/search_service.go` |

All four publish a message to the `BACKFILL` NATS JetStream stream. Messages are deduplicated by `accountID-dateHour`, so repeated views within the same hour do not queue redundant work. A cooldown window (default 24 hours) prevents re-backfilling the same account in quick succession.

## Backfill worker

The backfill worker (`internal/activitypub/backfill_worker.go`) consumes messages from the `BACKFILL` stream with up to 3 concurrent consumers. For each remote account, it fetches three collections:

### Outbox

Fetches the remote account's outbox collection and paginates through it, creating local statuses for any public or unlisted Notes. Private and direct statuses are skipped. Statuses already stored locally (matched by AP ID) are skipped.

- **Cap**: `maxPages` (default 2) limits the number of collection pages fetched.
- **Rate limiting**: 500ms delay between page fetches.

### Featured (pinned statuses)

Fetches the remote account's featured collection in a single request. Matches items against statuses already stored locally and updates the account's pinned status list. If the fetch fails, existing pins are preserved rather than cleared.

### Following

Fetches the remote account's following collection and paginates through it. For each actor IRI, resolves the actor via `RemoteAccountResolver` and creates a local follow relationship. Unknown actors are resolved (WebFinger + actor fetch) and stored. Duplicate follows are silently discarded.

- **Cap**: `maxPages` (default 2) limits the number of collection pages fetched; `maxItems` (default 200, `BACKFILL_MAX_ITEMS`) caps the total number of actor IRIs resolved across those pages. Once `maxItems` is reached the worker stops resolving further follows even if more pages remain. Set `BACKFILL_MAX_ITEMS=0` to disable the cap.
- **Rate limiting**: 500ms delay between page fetches.

### Collection counts

During actor resolution, Monstera fetches the `totalItems` field from the remote account's followers, following, and outbox collections to populate profile counts. These are lightweight HEAD-style fetches and are non-critical; failures return 0.

## Remote account resolution

`RemoteAccountResolver` (`internal/activitypub/remote_resolver.go`) is the single path for creating or updating remote accounts from actor documents. It is used by the backfill worker, search, and inbox handlers.

### Resolution flow

1. **WebFinger lookup** — `GET https://{domain}/.well-known/webfinger?resource=acct:user@domain` to discover the actor IRI.
2. **Actor document fetch** — `GET {actor_iri}` with `Accept: application/activity+json`. Response body is limited to 5MB.
3. **Sanitization** — Username and display name use a strict policy (plain text only). Bio/note uses a remote content policy that preserves formatting.
4. **Upsert** — `CreateOrUpdateRemoteAccount` stores or updates the account locally.

When resolving by IRI (e.g. during backfill), WebFinger is skipped and the actor document is fetched directly.

### Staleness

Remote accounts are considered stale after 1 hour. Accessing a stale account triggers a background re-fetch of the actor document. If the re-fetch fails, the cached version is returned and a warning is logged.

## Link preview cards

Separate from backfill, a `card_subscriber` listens for status creation events (both local and remote) and fetches OpenGraph metadata from the first external URL in the status content. Mention and hashtag links are excluded.

- **HTTP client**: SSRF-protected, `Monstera/1.0` user agent.
- **Body limit**: 1MB.
- **Failure handling**: Non-critical. Failures are logged but do not block or retry. The status is left without a card.
- **Extracted metadata**: `og:title`, `og:description`, `og:image`, with fallback to `<title>` and `<meta name="description">`.

## What Monstera does not do

- **No media proxying** — Remote media URLs are stored as-is and served directly to clients. Monstera does not download, cache, or re-serve remote media.
- **No followers collection crawling** — Only the `following` collection is crawled during backfill. Follower data comes exclusively from observed federation activity (Follow activities received via inbox).
- **No scheduled polling** — There are no background jobs that periodically re-crawl remote data. Staleness is checked on access; backfill is triggered by user actions.
- **No on-demand status fetching** — If a remote status is not already stored locally, there is no mechanism to fetch it by URI outside of the backfill worker's outbox crawl.

## HTTP client security

All outbound HTTP requests use the SSRF-protected client (`internal/ssrf/http_client.go`):

- **Allowed protocols**: HTTP and HTTPS only, ports 80 and 443 only.
- **Blocked addresses**: Loopback, private ranges (10.x, 172.16.x, 192.168.x), link-local, multicast, and reserved ranges. IPv6 is blocked.
- **Blocked hostnames**: `localhost`, `*.internal`, `*.local`, raw IP addresses.
- **Timeouts**: 5-second default for dial, read, and request.
- **Redirects**: Maximum 3 redirects.
