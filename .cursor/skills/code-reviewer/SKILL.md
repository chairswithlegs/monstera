---
name: local-code-reviewer
description: Reviews uncommitted changes for architectural soundness, code quality, and test adequacy; then runs the linter and unit tests. Use when the user asks to review changes, review uncommitted code, or run a pre-commit check.
---

# Local Code Reviewer

Reviews uncommitted changes in the workspace, then runs linter and tests.

## Workflow

1. **Get changes**: Run `git diff` and `git status` (and optionally `git diff --staged`) to see what is modified/added.
2. **Review**: Apply the review criteria below to the changed files.
3. **Report**: Summarize findings with clear severity (Critical / Suggestion / Nice to have).
4. **Verify**: Run linter and unit tests and report results.

## Review Criteria

### 1. Architectural soundness

- **Layers**: `internal/service` must not import `internal/api`. Dependencies point inward toward `internal/domain`.
- **HTTP/domain boundary**: Store and service must not use `net/http` or status codes; only handlers map errors to HTTP via `api.HandleError`.
- **IDs**: Use ULIDs from `internal/uid`. Config from env via `internal/config` only.

### 2. Code quality

- **Errors**: Use `%w` when wrapping; never `%v`. Sentinels in owning package (e.g. `internal/domain/errors.go`). Store translates `pgx.ErrNoRows` → `domain.ErrNotFound`; unique violations → `domain.ErrConflict`. No double logging (service/store return; handler logs in `HandleError`).
- **Style**: No narrative comments; use `slog` for logging, no `fmt.Printf`/`log.Println`.
- **Handlers**: Pattern `svc call → if err { api.HandleError(...); return }; write response`.

### 3. Tests

- **Presence**: New or modified behavior should have corresponding tests (e.g. `*_test.go` next to code).
- **Conventions**: Use `require` for preconditions, `assert` for verifications; `t.Helper()` in helpers; table-driven tests with a `name` field; prefer fakes over mocks for simple interfaces; HTTP tests use `httptest` and testify.
- **Coverage**: Tests should cover success and important failure paths; no skipping or disabling tests to pass.

## Feedback format

- **Critical**: Must fix (architecture violation, missing error handling, no tests for new behavior, or linter/test failure).
- **Suggestion**: Should consider (convention drift, weak tests, style).
- **Nice to have**: Optional improvement.

## After the review

1. **Linter** (from repo root):
   ```bash
   golangci-lint run
   ```
2. **Unit tests**:
   ```bash
   make test
   ```
   Or `go test ./...` if `make test` is not available.
3. Report any linter or test failures and treat them as **Critical** in the summary.

## Project rules reference

When reviewing, apply the project’s `.cursor/rules` as needed: `error-handling.mdc`, `lint-and-test.mdc`, `testing.mdc`, `monstera-project.mdc`, and any domain-specific rules (e.g. `store.mdc`, `api-mastodon.mdc`) for touched areas.
