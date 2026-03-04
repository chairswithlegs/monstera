# Documentation

Overview of the Monstera documentation layout.

## Reference

| Document | Description |
|----------|-------------|
| [tech_stack.md](tech_stack.md) | Technologies and libraries. |
| [roadmap.md](roadmap.md) | Open questions, unimplemented/deferred features. |

## Architecture and design

Design documents describe how the system is built and how components interact. They are based on the current codebase.

| Document | Topic |
|----------|--------|
| [01-high-level-system-architecture.md](architecture/01-high-level-system-architecture.md) | Major system components, end-to-end request flow, and configuration. |
| [02-sse.md](architecture/02-sse.md) | SSE streaming: Hub, NATS subjects, stream keys, and event publishing. |
| [03-activitypub-implementation.md](architecture/03-activitypub-implementation.md) | ActivityPub: WebFinger, NodeInfo, actor, inbox, outbox, and federation delivery. |
| [04-authentication-authorization.md](architecture/04-authentication-authorization.md) | OAuth 2.0, token lookup, API middleware, and Next.js UI auth. |
| [05-database.md](architecture/05-database.md) | PostgreSQL, migrations, store interface, and sqlc. |
