---
name: system-architect
description: Reviews application architecture from a holistic view. Ensures subsystems are focused and loosely coupled, code is maintainable and readable, and conventions are consistent. Use when reviewing architecture, assessing design, or when the user asks about coupling, layers, or codebase structure.
---

# System Architect

Use this skill when reviewing the architecture of the application. Take a holistic view: layers, subsystems, dependencies, and conventions.

## Scope of review

1. **Layers and dependency direction** — Dependencies must point inward (toward `internal/domain`). No cycles.
2. **Subsystem focus** — Each area (store, cache, media, email, activitypub, nats, oauth, api) has a clear responsibility and minimal surface.
3. **Loose coupling** — Subsystems interact via interfaces and domain types; no layer below `internal/api` depends on HTTP or API types.
4. **Maintainability and readability** — Consistent patterns, clear naming, appropriate error handling and logging.
5. **Conventions** — Project rules (`.cursor/rules/`) and docs (`docs/spec.md`, `docs/architecture/`) are respected.

## Project-specific rules (Monstera-fed)

- **internal/domain** — Zero internal imports. Pure types and sentinel errors only.
- **internal/service** — Never imports `internal/api`. Depends only on domain and infra interfaces (store, cache, media, email, etc.).
- **internal/api** — Only place that imports `net/http` and maps errors to HTTP status (via `api.HandleError`).
- **Errors** — Wrap with `%w`; map to HTTP only in handlers. Sentinels in owning package (`domain`, cache, media, email).
- **IDs** — ULIDs via `internal/uid`.
- **Config** — 12-factor; env vars only via `internal/config`.

## Review checklist

### Coupling and boundaries

- [ ] No import of `internal/api` from `internal/service` or infra packages.
- [ ] No `net/http` or HTTP status codes below `internal/api`.
- [ ] Cross-subsystem calls go through interfaces (e.g. store, cache) not concrete packages where it would create unwanted coupling.
- [ ] `internal/domain` has no imports from other `internal/*` packages.

### Focus and clarity

- [ ] Each package has a single, clear responsibility (see `docs/architecture/` for intended boundaries).
- [ ] New code lives in the right layer (domain vs service vs api vs infra).
- [ ] Shared types live in `internal/domain`; API-specific DTOs in `internal/api/.../apimodel` or equivalent.

### Maintainability and conventions

- [ ] Error handling: wrap with context using `%w`; use sentinels where appropriate; no `%v` when wrapping.
- [ ] Tests: `require` for preconditions, `assert` for checks; `t.Helper()` in helpers; table-driven where useful.
- [ ] Logging: `slog` only; no `fmt.Printf` or `log.Println`.
- [ ] Naming and style align with existing code and `.cursor/rules/`.

### Documentation and consistency

- [ ] Architecture decisions and non-obvious choices are reflected in `docs/architecture/` or ADRs where appropriate.
- [ ] NATS streams and subjects follow `docs/nats_conventions.md`.
- [ ] New behavior matches patterns described in `docs/spec.md` and implementation order.

## How to apply

1. **Identify scope** — Full codebase, a subsystem, or a change (e.g. new feature, refactor).
2. **Trace dependencies** — Use `go list -deps` or inspect imports to verify direction and absence of cycles.
3. **Check boundaries** — Ensure store, cache, media, email, activitypub, nats, oauth, and api stay within their contracts.
4. **Verify conventions** — Cross-check against `.cursor/rules/` (monstera-project, error-handling, testing) and docs.
5. **Report** — Summarize findings: what’s aligned, what’s violated, and concrete recommendations (with file/package references).

## Reference

- **Architecture docs** — `docs/architecture/01-project-foundation.md` through `10-admin-portal-and-moderation.md` describe intended design per subsystem.
- **Spec and order** — `docs/spec.md`, `docs/roadmap.md`, `docs/nats_conventions.md`.
