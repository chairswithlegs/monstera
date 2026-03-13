# Monstera

A self-hosted **ActivityPub server** written in Go that exposes the **Mastodon-compatible REST API**. Use any Mastodon client (Ivory, Tusky, Elk, Mona, Pinafore, etc.) to connect without modification.

## Features

- **Mastodon API** — Accounts, statuses, timelines, notifications, media, search, streaming (SSE)
- **ActivityPub federation** — Follow and interact with users on other instances (Mastodon, Pleroma, etc.)
- **OAuth 2.0** — Authorization Code and PKCE for client apps
- **UI** — Next.js UI for users, reports, moderation, instance settings
- **Horizontally scalable** — Stateless API pods; PostgreSQL + NATS JetStream for federation and SSE fan-out

## Requirements

| Component | Required | Notes |
|-----------|----------|--------|
| **Go 1.26+** | Yes | Build and run |
| **PostgreSQL 16+** | Yes | Primary data store |
| **NATS** (JetStream) | Yes | Federation delivery queue, SSE pub/sub |
| S3-compatible storage | No | Use `MEDIA_DRIVER=local` for dev/small deploys |

## Quick start

### Using Docker Compose

```bash
docker compose up -d
```

This starts the app on port 8080, PostgreSQL on 5433, and NATS. Run migrations and seed test users:

```bash
make migrate
make seed
```

Then open the instance at `http://localhost:8080` and sign in (e.g. `admin` / `password`. Default test accounts:

| Username | Email              | Password  | Role  |
|----------|--------------------|-----------|-------|
| admin    | admin@example.com  | password  | admin |
| alice    | alice@example.com  | password  | user  |

## Configuration

All configuration is via environment variables (12-factor).

### Required

| Variable | Description |
|----------|-------------|
| `INSTANCE_DOMAIN` | Public hostname (e.g. `social.example.com`) |
| `DATABASE_URL` | PostgreSQL connection string |
| `NATS_URL` | NATS server URL |
| `MEDIA_BASE_URL` | Base URL for media (e.g. `https://social.example.com/media`) |
| `EMAIL_FROM` | From address for outgoing email |
| `SECRET_KEY_BASE` | 64+ hex chars for signing and key derivation |

### Server

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_ENV` | `development` or `production` | `development` |
| `APP_PORT` | HTTP listen port | `8080` |
| `INSTANCE_NAME` | Instance display name | `Monstera` |
| `LOG_LEVEL` | `debug`, `info`, `warn`, or `error` | `info` |
| `METRICS_TOKEN` | Optional token for `/metrics` (empty = no auth) | — |
| `VERSION` | App version string | `0.0.0-dev` |

### Database

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_MAX_OPEN_CONNS` | Max open connections in pool | `20` |
| `DATABASE_MAX_IDLE_CONNS` | Max idle connections in pool | `5` |

### NATS

| Variable | Description | Default |
|----------|-------------|---------|
| `NATS_CREDS_FILE` | Path to NATS credentials file (optional) | — |

### Cache

| Variable | Description | Default |
|----------|-------------|---------|
| `CACHE_DRIVER` | `memory` | `memory` |

### Media

| Variable | Description | Default |
|----------|-------------|---------|
| `MEDIA_DRIVER` | `local` or `s3` | `local` |
| `MEDIA_LOCAL_PATH` | Directory for local uploads (required if `MEDIA_DRIVER=local`) | — |
| `MEDIA_S3_BUCKET` | S3 bucket (required if `MEDIA_DRIVER=s3`) | — |
| `MEDIA_S3_REGION` | S3 region (required if `MEDIA_DRIVER=s3`) | — |
| `MEDIA_S3_ENDPOINT` | S3 endpoint (for MinIO etc.) | — |
| `MEDIA_CDN_BASE` | Optional CDN base URL for media | — |
| `MEDIA_MAX_BYTES` | Max upload size in bytes | `10485760` (10 MiB) |

### Email

| Variable | Description | Default |
|----------|-------------|---------|
| `EMAIL_DRIVER` | `noop` or `smtp` | `noop` |
| `EMAIL_FROM_NAME` | From name for outgoing email | `Monstera` |
| `EMAIL_SMTP_HOST` | SMTP host (required if `EMAIL_DRIVER=smtp`) | — |
| `EMAIL_SMTP_PORT` | SMTP port | `587` |
| `EMAIL_SMTP_USERNAME` | SMTP username | — |
| `EMAIL_SMTP_PASSWORD` | SMTP password | — |

### Federation

| Variable | Description | Default |
|----------|-------------|---------|
| `FEDERATION_WORKER_CONCURRENCY` | Number of federation delivery workers | `5` |
| `FEDERATION_INSECURE_SKIP_TLS_VERIFY` | Skip TLS verification for federation (dev only) | dev: `true`, prod: `false` |
| `MAX_STATUS_CHARS` | Max characters per status | `500` |

See [docs/tech_stack.md](docs/tech_stack.md) and `internal/config` for full configuration options.

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
| [docs/tech_stack.md](docs/tech_stack.md) | Technologies and libraries |
| [docs/roadmap.md](docs/roadmap.md) | Open questions, deferred features, and future phases |
| [docs/architecture/](docs/architecture/) | Architecture and design docs |

See [docs/README.md](docs/README.md) for the full index.

## License

See repository for license information.
