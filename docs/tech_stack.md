# Monstera — Technology Stack

High-level overview of the technologies and libraries used in the Monstera project.

---

## Language & Runtime

| | |
|---|---|
| **Go 1.26+** | Primary language for the API server. Compiles to a single static binary with no runtime dependencies. |

---

## Infrastructure

| Component | Technology | Role |
|-----------|-----------|------|
| Database | **PostgreSQL 16+** | Primary data store. Relational schema for the social graph, JSONB columns for raw ActivityPub payloads. |
| Message Broker | **NATS JetStream** | Durable federation delivery queues (at-least-once) and ephemeral core pub/sub for SSE fan-out across replicas. |
| Cache (local) | **In-memory (ristretto)** | Per-process caching: timeline, token lookup. |
| Cache (shared) | **NATS JetStream KV** | Cross-pod state: rate limiting, idempotency keys, HTTP signature replay prevention. Falls back to in-memory when only one replica is running. |
| Object Storage | **S3-compatible** or **local filesystem** | Media attachment storage (images, video, audio). |

---

## Go Libraries (backend)

### Core

| Library | Import Path | Purpose |
|---------|------------|---------|
| chi | `github.com/go-chi/chi/v5` | HTTP router and middleware stack |
| pgx | `github.com/jackc/pgx/v5` | PostgreSQL driver (connection pooling via `pgxpool`) |
| sqlc | (code generator) | Type-safe Go code generation from SQL; generated code lives in `internal/store/postgres/generated/` |
| golang-migrate | `github.com/golang-migrate/migrate/v4` | SQL-first database migration runner; migrations in `internal/store/migrations/` |
| NATS client | `github.com/nats-io/nats.go` | NATS connection management, core pub/sub, JetStream API |
| ULID | `github.com/oklog/ulid/v2` | Time-sortable, lexicographically ordered unique IDs |
| Cobra | `github.com/spf13/cobra` | CLI subcommands (serve, migrate) |

### Caching

| Library | Import Path | Purpose |
|---------|------------|---------|
| ristretto | `github.com/dgraph-io/ristretto/v2` | In-memory cache with cost-based eviction |
| singleflight | `golang.org/x/sync/singleflight` | Request deduplication to prevent cache stampedes |

### Media & Content

| Library | Import Path | Purpose |
|---------|------------|---------|
| AWS SDK v2 | `github.com/aws/aws-sdk-go-v2/service/s3` | S3-compatible object storage client |
| go-nanoid | `github.com/jaevor/go-nanoid` | Short, URL-safe IDs for media storage keys |
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

## UI

The UI is implemented as a **separate Next.js application** in the `ui/` directory. It talks to the Go API via the Monstera REST API using Bearer token authentication.

| Technology | Role |
|-----------|------|
| **Next.js 16** | React framework (App Router) |
| **React 19** | UI components |
| **TypeScript** | Typed JavaScript |
| **Tailwind CSS 4** | Utility-first styling |
| **shadcn (Radix UI)** | Accessible component primitives |
| **Lucide React** | Icons |

---

## Testing & Linting

| Tool | Purpose |
|------|---------|
| testify | Assertions (`assert`, `require`), mocking |
| golangci-lint | Linter aggregator — runs `staticcheck`, `gosec`, `errcheck`, `testifylint`, and others |
| gofumpt | via golangci-lint formatters — strict `gofmt` superset |

See the root `CLAUDE.md` for test conventions and lint/test commands.

---
