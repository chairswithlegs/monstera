# Monstera

A lightweight, self-hosted **ActivityPub server** written in Go that exposes the **Mastodon-compatible REST API**.

## Overview

Monstera is designed for small-to-medium communities who want to participate in the Fediverse without the operational overhead of running a full Mastodon stack. Built in Go with PostgreSQL and NATS, it runs efficiently on modest hardware while still allowing for horizontal scalability, allowing your instance to grow to meet the size of your community.

While Monstera includes a UI for server administration, it does not include a Mastodon style UI. It does however implement the Mastodon REST API, meaning that popular Mastodon clients (e.g. Ivory, Elk, etc.) can be used with Monstera. 

## Features

- **Mastodon API** — Accounts, statuses, timelines, notifications, media, search, streaming (SSE)
- **ActivityPub federation** — Follow and interact with users on other instances (Mastodon, Pleroma, etc.)
- **OAuth 2.0** — Authorization Code and PKCE for client apps
- **UI** — Next.js web interface for users, reports, moderation, and instance settings
- **Horizontally scalable** — Stateless API pods; PostgreSQL + NATS JetStream for federation delivery and SSE fan-out

## Requirements

| Component | Required | Notes |
|-----------|----------|--------|
| **Go 1.26+** | Yes | Build and run |
| **PostgreSQL 16+** | Yes | Primary data store |
| **NATS** (JetStream) | Yes | Federation delivery queue, SSE pub/sub |
| S3-compatible storage | No | Use `MEDIA_DRIVER=local` for dev/small deploys |

## Quick start

```bash
make start
```

This builds and starts everything in Docker (server, UI, PostgreSQL, NATS, MinIO), runs migrations, and seeds test data. Once running, open `http://localhost:8080` and sign in with `admin` / `password`.

For local development (dependencies in Docker, server and UI running natively):

```bash
make start-dev
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for more details on the development setup.

## Deployment

Monstera is designed to run in containers. Docker images for the API server and UI are published to GitHub Container Registry on each release:

- `ghcr.io/chairswithlegs/server`
- `ghcr.io/chairswithlegs/ui`

### Kubernetes

For Kubernetes deployments, use the official Helm chart: [monstera-chart](https://github.com/chairswithlegs/monstera-chart).

## Configuration

Monstera is configured entirely via environment variables. See [docs/configuration.md](docs/configuration.md) for the complete reference.

## Development

```bash
make test             # Unit tests
make test-integration # Integration tests (requires Docker Compose up)
make lint             # golangci-lint
make lint-fix         # Auto-fix lint issues
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development setup guide and code conventions.

## Documentation

| Document | Description |
|----------|-------------|
| [docs/configuration.md](docs/configuration.md) | Environment variable reference |
| [docs/tech_stack.md](docs/tech_stack.md) | Technologies and libraries |
| [docs/architecture/](docs/architecture/) | Architecture and design docs |

See [docs/README.md](docs/README.md) for the full index.

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, code conventions, and the PR process.

## License

See repository for license information.
