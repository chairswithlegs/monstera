---
name: code-reviewer
description: Reviews changes for architectural soundness, code quality, and test adequacy; then runs the linter and unit tests. Use when the user asks to review changes.
---

# Code Reviewer

Reviews changes, then runs linter and tests.

Depending on context, "changes" can refer to either local uncommitted changes, a branch, or a pull request. Request clarification from the user if it is unclear what changes need to be reviewed.

## Workflow

1. **Get changes**: Run `git diff` and `git status` (and optionally `git diff --staged`) to see what is modified/added.
2. **Review**: Apply the review criteria below to the changed files.
3. **Report**: Summarize findings with clear severity (Critical / Suggestion / Nice to have).
4. **Verify**: Run linter and unit tests and report results.

## Review Criteria

### 1. Architectural soundness

- **Layers**: `internal/service` must not import `internal/api`. Dependencies point inward toward `internal/domain`.
- **API/domain boundary**: Store and service must not use `net/http` or status codes; only handlers map errors to HTTP via `api.HandleError`. Business logic should live in the service layer. The API layer should only be concerned with HTTP semantics, DTOs, and calling the correct service methods.
- **IDs**: Use ULIDs from `internal/uid`. Config from env via `internal/config` only.

### 2. Code quality

- **Correctness**: Does the code achieve its stated purpose without bugs or logical errors?
- **Maintainability**: Is the code clean, well-structured, and easy to understand and modify in the future? Consider factors like code clarity, modularity, and adherence to established design patterns.
- **Readability**: Is the code well-commented (where necessary) and consistently formatted according to our project's coding style guidelines?
- **Efficiency**: Are there any obvious performance bottlenecks or resource inefficiencies introduced by the changes?
- **Security**: Are there any potential security vulnerabilities or insecure coding practices?
- **Edge Cases and Error Handling**: Does the code appropriately handle edge cases and potential errors?

### 3. Tests

- **Presence**: New or modified behavior should have corresponding tests (e.g. `*_test.go` next to code).
- **Conventions**: Use `require` for preconditions, `assert` for verifications; `t.Helper()` in helpers; table-driven tests with a `name` field; prefer fakes over mocks for simple interfaces; HTTP tests use `httptest` and testify.
- **Coverage**: Tests should cover success and important failure paths; no skipping or disabling tests to pass.

### 4. Security

- **Authorization/Authentication**: New or updated non-public endpoints have auth guards. Public endpoints may feature optional authorization if required.
- **Visibility**: Status read paths (single status, context, favourited_by, reblogged_by, timelines) must enforce visibility in the **service layer** (e.g. `canViewStatus` / `CanViewStatus`). Unauthenticated viewers must not see private or direct statuses (404, not 403). Viewer identity must be passed into service methods (e.g. `GetByIDEnriched(ctx, id, viewerID)`); handlers derive `viewerID` from request context and pass it through. Do not push visibility rules into SQL/store; keep them in service logic.
- **User blocks**: When determining whether a viewer can see a status, block relationships must be applied in the same service-layer check as visibility: if the viewer has blocked the author or the author has blocked the viewer, the status is not visible (return 404). Use store `IsBlockedEitherDirection` (or equivalent) inside the visibility helper so single-status read, context, and list timeline all respect blocks consistently.

## Feedback format

- **Critical**: Must fix (architecture violation, missing error handling, no tests for new behavior, or linter/test failure).
- **Suggestion**: Should consider (convention drift, weak tests, style).
- **Nice to have**: Minor optional improvements that could be made.

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
