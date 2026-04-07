# Documentation

Overview of the Monstera documentation layout.

## Reference

| Document | Description |
|----------|-------------|
| [configuration.md](configuration.md) | Environment variable reference. |
| [tech_stack.md](tech_stack.md) | Technologies and libraries. |
| [adding-a-locale.md](adding-a-locale.md) | Adding a locale. |

## Architecture and design

Design documents describe how the system is built and how components interact. They are based on the current codebase.

| Document | Topic |
|----------|--------|
| [01-high-level-system-architecture.md](architecture/01-high-level-system-architecture.md) | Major system components, end-to-end request flow, and configuration. |
| [02-sse.md](architecture/02-sse.md) | SSE streaming: Hub, NATS subjects, stream keys, and event publishing. |
| [03-activitypub-implementation.md](architecture/03-activitypub-implementation.md) | ActivityPub: WebFinger, NodeInfo, actor, inbox, outbox, and federation delivery. |
| [04-authentication-authorization.md](architecture/04-authentication-authorization.md) | OAuth 2.0, token lookup, API middleware, and Next.js UI auth. |
| [05-database.md](architecture/05-database.md) | PostgreSQL, migrations, store interface, and sqlc. |
| [06-domain-events-and-outbox.md](architecture/06-domain-events-and-outbox.md) | Transactional outbox, domain events, and event subscribers. |
| [07-notifications-and-push.md](architecture/07-notifications-and-push.md) | Notification creation, Web Push subscriptions, and push delivery. |
| [08-rate-limiting-and-shared-cache.md](architecture/08-rate-limiting-and-shared-cache.md) | Rate limiting, shared cache, and the NATS KV store. |
| [09-background-scheduler.md](architecture/09-background-scheduler.md) | Distributed background job scheduler and registered jobs. |
| [10-remote-data-hydration.md](architecture/10-remote-data-hydration.md) | Proactive remote data fetching: backfill triggers, worker, account resolution, and link cards. |
