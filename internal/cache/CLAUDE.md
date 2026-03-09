# Cache Layer

Design doc: `docs/architecture/01-high-level-system-architecture.md`

## Conventions

- Implements the `cache.Store` interface (`Get`, `Set`, `Delete`, `Exists`).
- `Get` returns `cache.ErrCacheMiss` on miss — not `domain.ErrNotFound`.
- Driver selected at startup via `cfg.CacheDriver` env var.
- All cache keys use a namespaced format: `{entity}:{id}` (e.g., `account:01ABC`, `admin_session:hextoken`).
- TTLs are caller-specified per `Set` call, not global.
