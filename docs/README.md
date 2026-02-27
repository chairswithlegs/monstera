# Documentation

Overview of the Monstera-fed documentation layout.

## Reference

| Document | Description |
|----------|-------------|
| [spec.md](spec.md) | Project specification — architecture, API, data model, configuration |
| [tech_stack.md](tech_stack.md) | Technologies and libraries |
| [roadmap.md](roadmap.md) | Roadmap (build sequence and validation milestones) |
| [nats_conventions.md](nats_conventions.md) | NATS streams and subject naming |
| [open_questions.md](open_questions.md) | Unresolved architecture and product decisions |
| [phase_two_features.md](phase_two_features.md) | Deferred features for later phases |

## Architecture and design

Design documents describe the **desired state** of the system. Planning and build order live in [roadmap.md](roadmap.md). Unresolved decisions are in [open_questions.md](open_questions.md).

| Document | Topic |
|----------|--------|
| [01-project-foundation.md](architecture/01-project-foundation.md) | CLI, router, config, observability |
| [02-data-model-and-migrations.md](architecture/02-data-model-and-migrations.md) | Schema, store interface, migrations |
| [03-cache-abstraction.md](architecture/03-cache-abstraction.md) | Cache interface, memory/Redis drivers |
| [04-media-storage-abstraction.md](architecture/04-media-storage-abstraction.md) | Media interface, local/S3 drivers |
| [05-email-abstraction.md](architecture/05-email-abstraction.md) | Email interface, noop/SMTP drivers |
| [06-oauth-and-authentication.md](architecture/06-oauth-and-authentication.md) | OAuth 2.0, PKCE, sessions |
| [07-activitypub-and-federation.md](architecture/07-activitypub-and-federation.md) | ActivityPub, federation, inbox/outbox |
| [08-mastodon-rest-api.md](architecture/08-mastodon-rest-api.md) | Mastodon REST API handlers |
| [09-sse-streaming-and-nats.md](architecture/09-sse-streaming-and-nats.md) | SSE streaming, NATS pub/sub |
| [10-admin-portal-and-moderation.md](architecture/10-admin-portal-and-moderation.md) | Admin UI, moderation, reports |
