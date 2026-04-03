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
| `MONSTERA_INSTANCE_DOMAIN` | Handle domain — appears in `@user@domain` and WebFinger (e.g. `social.example.com`) |
| `MONSTERA_UI_URL` | Full URL to the Next.js UI (e.g. `https://social.example.com`) |
| `DATABASE_HOST` | PostgreSQL hostname (e.g. `postgres`) |
| `NATS_URL` | NATS server URL (e.g. `nats://nats:4222`) |
| `MEDIA_BASE_URL` | Base URL served for uploaded media (e.g. `https://social.example.com/media`) |
| `EMAIL_FROM` | From address for outgoing email (e.g. `noreply@social.example.com`) |
| `SECRET_KEY_BASE` | 64+ hex chars (32+ bytes) for signing and key derivation |

### Server

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_ENV` | `development` or `production` | `development` |
| `APP_PORT` | HTTP listen port | `8080` |
| `MONSTERA_SERVER_URL` | Base URL for ActivityPub IRIs (e.g. `https://api.example.com`). Defaults to `https://{MONSTERA_INSTANCE_DOMAIN}`. Set this when your API server lives on a different hostname than your handle domain. | — |
| `LOG_LEVEL` | `debug`, `info`, `warn`, or `error` | `info` |
| `METRICS_TOKEN` | Bearer token for `/metrics` endpoint (empty = no auth) | — |
| `VERSION` | App version string | `0.0.0-dev` |

### Database

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_PORT` | PostgreSQL port | `5432` |
| `DATABASE_NAME` | Database name | `monstera` |
| `DATABASE_USERNAME` | Database user | `monstera` |
| `DATABASE_PASSWORD` | Database password | `monstera` |
| `DATABASE_MAX_OPEN_CONNS` | Max open connections in pool | `20` |
| `DATABASE_SSL_MODE` | PostgreSQL SSL mode | `disable` |

### NATS

| Variable | Description | Default |
|----------|-------------|---------|
| `NATS_CREDS_FILE` | Path to NATS credentials file | — |

### Cache

| Variable | Description | Default |
|----------|-------------|---------|
| `CACHE_DRIVER` | Cache backend (`memory`) | `memory` |

### Media

| Variable | Description | Default |
|----------|-------------|---------|
| `MEDIA_DRIVER` | Storage backend: `local` or `s3` | `local` |
| `MEDIA_LOCAL_PATH` | Directory for local uploads (required when `MEDIA_DRIVER=local`) | — |
| `MEDIA_S3_BUCKET` | S3 bucket name (required when `MEDIA_DRIVER=s3`) | — |
| `MEDIA_S3_REGION` | S3 region (required when `MEDIA_DRIVER=s3`) | — |
| `MEDIA_S3_ENDPOINT` | S3-compatible endpoint override (e.g. for MinIO) | — |
| `MEDIA_CDN_BASE` | CDN base URL to prefix media URLs | — |
| `MEDIA_MAX_BYTES` | Max upload size in bytes | `10485760` (10 MiB) |

### Email

| Variable | Description | Default |
|----------|-------------|---------|
| `EMAIL_DRIVER` | Email backend: `noop` or `smtp` | `noop` |
| `EMAIL_FROM_NAME` | Sender display name | `Monstera` |
| `EMAIL_SMTP_HOST` | SMTP hostname (required when `EMAIL_DRIVER=smtp`) | — |
| `EMAIL_SMTP_PORT` | SMTP port | `587` |
| `EMAIL_SMTP_USERNAME` | SMTP username | — |
| `EMAIL_SMTP_PASSWORD` | SMTP password | — |

### Federation

| Variable | Description | Default |
|----------|-------------|---------|
| `FEDERATION_WORKER_CONCURRENCY` | Number of parallel federation delivery workers | `5` |
| `FEDERATION_INSECURE_SKIP_TLS_VERIFY` | Skip TLS verification for outbound federation requests | dev: `true`, prod: `false` |
| `BACKFILL_MAX_PAGES` | Max outbox pages to fetch per remote account backfill | `2` |
| `BACKFILL_COOLDOWN_HOURS` | Minimum hours between backfills for the same account | `24` |

### Push notifications

| Variable | Description | Default |
|----------|-------------|---------|
| `VAPID_PUBLIC_KEY` | VAPID public key for Web Push (leave unset to disable push) | — |
| `VAPID_PRIVATE_KEY` | VAPID private key for Web Push | — |

### Limits

| Variable | Description | Default |
|----------|-------------|---------|
| `MAX_STATUS_CHARS` | Max characters per status | `500` |
| `MAX_REQUEST_BODY_BYTES` | Max request body size in bytes | `1048576` (1 MiB) |

### Rate limiting

| Variable | Description | Default |
|----------|-------------|---------|
| `RATE_LIMIT_AUTH_PER_WINDOW` | Max authenticated requests per window (0 = disabled) | `300` |
| `RATE_LIMIT_AUTH_WINDOW_SECONDS` | Authenticated rate-limit window in seconds | `300` |
| `RATE_LIMIT_PUBLIC_PER_WINDOW` | Max unauthenticated requests per window (0 = disabled) | `300` |
| `RATE_LIMIT_PUBLIC_WINDOW_SECONDS` | Unauthenticated rate-limit window in seconds | `300` |

See `internal/config/config.go` for the authoritative list.

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
