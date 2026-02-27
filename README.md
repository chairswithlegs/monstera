# Monstera-fed

A self-hosted **ActivityPub server** written in Go that exposes the **Mastodon-compatible REST API**. Use any Mastodon client (Ivory, Tusky, Elk, Mona, Pinafore, etc.) to connect without modification.

## Features

- **Mastodon API** — Accounts, statuses, timelines, notifications, media, search, streaming (SSE)
- **ActivityPub federation** — Follow and interact with users on other instances (Mastodon, Pleroma, etc.)
- **OAuth 2.0** — Authorization Code and PKCE for client apps
- **Pluggable backends** — Cache (memory or Redis), media (local or S3), email (noop or SMTP)
- **Admin portal** — Server-rendered UI (HTMX + Pico.css) for users, reports, moderation, instance settings
- **Horizontally scalable** — Stateless API pods; PostgreSQL + NATS JetStream for federation and SSE fan-out

## Requirements

| Component | Required | Notes |
|-----------|----------|--------|
| **Go 1.26+** | Yes | Build and run |
| **PostgreSQL 16+** | Yes | Primary data store |
| **NATS** (JetStream) | Yes | Federation delivery queue, SSE pub/sub |
| Redis / Valkey | No | Use `CACHE_DRIVER=memory` for single-node |
| S3-compatible storage | No | Use `MEDIA_DRIVER=local` for dev/small deploys |

## Quick start

### Using Docker Compose

```bash
docker compose up -d
```

This starts the app on port 8080, PostgreSQL on 5433, and NATS. Run migrations and seed test users:

```bash
# Run migrations (from host; DB is on localhost:5433)
DATABASE_URL="postgres://monstera:monstera@localhost:5433/monstera_fed?sslmode=disable" \
  ./bin/monstera-fed migrate up

make seed
```
Then open the instance at `http://localhost:8080` and sign in (e.g. `admin` / `password`. Default test accounts:

| Username | Email              | Password  | Role  |
|----------|--------------------|-----------|-------|
| admin    | admin@example.com  | password  | admin |
| alice    | alice@example.com  | password  | user  |

### Local build (no Docker)

1. Install PostgreSQL 16+ and NATS with JetStream.
2. Set environment variables (see [Configuration](#configuration)).
3. Build, migrate, and run:

```bash
make build
./bin/monstera-fed migrate up
./bin/monstera-fed serve
```

## Configuration

All configuration is via environment variables.

| Variable | Description | Default |
|----------|-------------|---------|
| `INSTANCE_DOMAIN` | Public hostname (e.g. `social.example.com`) | — |
| `DATABASE_URL` | PostgreSQL connection string | — |
| `NATS_URL` | NATS server URL | — |
| `SECRET_KEY_BASE` | 64+ hex chars for signing | — |
| `CACHE_DRIVER` | `memory` or `redis` | `memory` |
| `MEDIA_DRIVER` | `local` or `s3` | `local` |
| `EMAIL_DRIVER` | `noop` or `smtp` | `noop` |

Full list and deployment options: [docs/SPEC.md §19 Configuration](docs/SPEC.md#19-configuration).

## Development

```bash
make test             # Unit tests
make test-integration # Integration tests (requires Docker Compose up)
make lint             # golangci-lint
make lint-fix         # Auto-fix lint issues
```

## Documentation

| Document | Description |
|----------|-------------|
| [docs/SPEC.md](docs/SPEC.md) | Project specification — architecture, API, data model, config |
| [docs/TECH_STACK.md](docs/TECH_STACK.md) | Technologies and libraries |
| [docs/IMPLEMENTATION_ORDER.md](docs/IMPLEMENTATION_ORDER.md) | Build sequence and validation milestones |
| [docs/NATS_CONVENTIONS.md](docs/NATS_CONVENTIONS.md) | NATS streams and subject naming |
| [docs/ADR/](docs/ADR/) | Architecture decision records (01–10) |
| [docs/PHASE_TWO_FEATURES.md](docs/PHASE_TWO_FEATURES.md) | Deferred features for later phases |

## License

See repository for license information.
