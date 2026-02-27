# Monstera-fed — Technology Stack

> High-level overview of the technologies and libraries used in the Monstera-fed project.
> For full design rationale, see [spec.md](spec.md) and the documents in [architecture/](architecture/).

---

## Language & Runtime

| | |
|---|---|
| **Go 1.26+** | Primary language. Compiles to a single static binary with no runtime dependencies. |

---

## Infrastructure

| Component | Technology | Role |
|-----------|-----------|------|
| Database | **PostgreSQL 16+** | Primary data store. Relational schema for the social graph, JSONB columns for raw ActivityPub payloads, full-text search via `tsvector`/`tsquery` (Phase 2). |
| Message Broker | **NATS JetStream** | Durable federation delivery queues (at-least-once) and ephemeral core pub/sub for SSE fan-out across replicas. |
| Cache | **Redis / Valkey** (prod) or **in-memory** (dev) | Timeline caching, idempotency keys, HTTP signature replay prevention, admin session storage. |
| Object Storage | **S3-compatible** (prod) or **local filesystem** (dev) | Media attachment storage (images, video, audio). |
| Reverse Proxy | **NGINX / Envoy** (Kubernetes Ingress) | TLS termination, rate limiting, load balancing. |

---

## Go Libraries

### Core

| Library | Import Path | Purpose |
|---------|------------|---------|
| chi | `github.com/go-chi/chi/v5` | HTTP router and middleware stack |
| pgx | `github.com/jackc/pgx/v5` | PostgreSQL driver (connection pooling via `pgxpool`) |
| sqlc | `github.com/sqlc-dev/sqlc` (code generator) | Type-safe Go code generation from SQL queries |
| golang-migrate | `github.com/golang-migrate/migrate/v4` | SQL-first database migration runner |
| NATS client | `github.com/nats-io/nats.go` | NATS connection management, core pub/sub, JetStream API |
| ULID | `github.com/oklog/ulid/v2` | Time-sortable, lexicographically ordered unique IDs |

### Caching

| Library | Import Path | Purpose |
|---------|------------|---------|
| ristretto | `github.com/dgraph-io/ristretto/v2` | In-memory cache with cost-based eviction (single-node / dev) |
| go-redis | `github.com/redis/go-redis/v9` | Redis / Valkey client (multi-replica prod) |
| singleflight | `golang.org/x/sync/singleflight` | Request deduplication to prevent cache stampedes |

### Media & Content

| Library | Import Path | Purpose |
|---------|------------|---------|
| AWS SDK v2 | `github.com/aws/aws-sdk-go-v2/service/s3` | S3-compatible object storage client |
| go-blurhash | `github.com/buckket/go-blurhash` | BlurHash placeholder generation for images |
| go-nanoid | `github.com/jaevor/go-nanoid` | Short, URL-safe IDs for storage keys |
| webp decoder | `golang.org/x/image/webp` | WebP image format support |
| bluemonday | `github.com/microcosm-cc/bluemonday` | HTML sanitization for rendered content |
| xurls | `mvdan.cc/xurls/v2` | URL detection in plain-text status content |

### Email

| Library | Import Path | Purpose |
|---------|------------|---------|
| jordan-wright/email | `github.com/jordan-wright/email` | Multipart MIME construction, STARTTLS/TLS delivery |

### Authentication

| Library | Import Path | Purpose |
|---------|------------|---------|
| bcrypt | `golang.org/x/crypto/bcrypt` | Password hashing |

### Observability

| Library | Import Path | Purpose |
|---------|------------|---------|
| slog | `log/slog` (stdlib) | Structured JSON logging |
| Prometheus client | `github.com/prometheus/client_golang` | Metrics exposition at `/metrics` |

---

## Admin Portal

| Technology | Role |
|-----------|------|
| **Go `html/template`** | Server-side page rendering |
| **HTMX 2.x** | Partial-page updates without client-side JS framework |
| **Pico.css 2.x** | Classless CSS framework for clean default styling |

All admin assets are vendored and embedded into the Go binary via `go:embed` — no Node.js build step.

---

## Deployment

| Component | Technology |
|-----------|-----------|
| Container image | Multi-stage Dockerfile → `gcr.io/distroless/static:nonroot` |
| Orchestration | **Kubernetes** (Deployment, HPA, ConfigMap) |
| Local development | **Docker Compose** (Go server, PostgreSQL, NATS, optional Redis/MinIO) |
| NATS deployment | [NATS Helm chart](https://github.com/nats-io/k8s) |

---

## Testing & Linting

| Tool | Version | Purpose |
|------|---------|---------|
| testify | `github.com/stretchr/testify` v1.10.0 | Assertions (`assert`, `require`), mocking |
| golangci-lint | v2.9.0+ (external binary) | Linter aggregator — runs `staticcheck`, `gosec`, `errcheck`, `testifylint`, and others |
| gofumpt | via golangci-lint formatters | Strict `gofmt` superset for consistent formatting |

See `.cursor/rules/testing.mdc` for test conventions; `.cursor/rules/lint-and-test.mdc` for lint/test commands and CI.

---

## Phase 2 Additions (Planned)

| Library | Import Path | Purpose |
|---------|------------|---------|
| goldmark | `github.com/yuin/goldmark` | Markdown → HTML rendering for status content |
| otp | `github.com/pquerna/otp` | TOTP-based two-factor authentication |
