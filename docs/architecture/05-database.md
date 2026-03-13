# Database

This document describes the database layer: schema ownership, migrations, and how the application accesses data.

## Technology

- **PostgreSQL 16+** is the primary data store. All durable application state (accounts, users, statuses, follows, notifications, OAuth tokens, media metadata, domain blocks, reports, etc.) lives in PostgreSQL.
- **Cache** is used for ephemeral data (token lookup, timeline cache, idempotency, HTTP signature replay). It is not described as part of the “database” layer; see [tech_stack.md](../tech_stack.md).

## Schema and migrations

- **Migrations** are SQL files in `internal/store/migrations/`, named sequentially (e.g. `000001_...up.sql`, `000001_...down.sql`). They are applied at startup (or via `migrate up` CLI) using `github.com/golang-migrate/migrate/v4`.
- **Schema ownership**: The migrations define tables, indexes, and constraints. No ORM; the application relies on these SQL migrations as the single source of truth for schema.

## Data access

- **Queries** are written in SQL in `internal/store/postgres/queries/`. The **sqlc** tool generates Go code from these SQL files into `internal/store/postgres/generated/` (e.g. `db.go`, `querier.go`, `*.sql.go`, `models.go`). The generated code is type-safe and uses `*pgxpool.Pool` and the `Queries` struct.
- **Store interface** (`internal/store/store.go`): A large `Store` interface defines all persistence operations in terms of domain types (from `internal/domain`). Parameters and results use domain structs, not raw SQL or database-specific types.
- **Postgres implementation** (`internal/store/postgres/store.go`): The concrete implementation composes the sqlc `Queries` and implements `Store` by calling the generated methods and converting rows to domain types. Errors from the driver (e.g. `pgx.ErrNoRows`, unique violation) are translated to domain sentinels (e.g. `domain.ErrNotFound`, `domain.ErrConflict`) and wrapped with context.

## Usage pattern

- **Service layer** (`internal/service/`) depends only on `store.Store` (and other interfaces such as cache, media, event bus). It never imports postgres or sqlc output directly.
- **Writes** that need multiple operations in a single transaction use `store.WithTx(ctx, func(store.Store) error { ... })`. The postgres implementation runs the callback with a store that uses a transaction.
