# Monstera-fed — Testing & Linting Guide

> Conventions, tooling, and configuration for testing and static analysis.

---

## Tools

| Tool | Version | Purpose |
|------|---------|---------|
| `go test` | Go 1.26 stdlib | Test runner |
| [testify](https://github.com/stretchr/testify) | v1.10.0 | Assertions (`assert`, `require`), HTTP test helpers, mocking |
| [golangci-lint](https://golangci-lint.run) | v2.9.0+ | Linter aggregator and formatter |

```bash
go get github.com/stretchr/testify@v1.10.0
```

---

## Test Organisation

Tests live alongside the code they test, using Go's standard `_test.go` convention.

```
internal/
├── service/
│   ├── account_service.go
│   ├── account_service_test.go      ← unit tests
│   ├── status_service.go
│   └── status_service_test.go
├── api/mastodon/
│   ├── accounts.go
│   └── accounts_test.go             ← HTTP handler tests
├── store/postgres/
│   └── queries_test.go              ← integration tests (require DB)
└── ap/
    ├── httpsig.go
    └── httpsig_test.go
```

### Build tags for integration tests

Tests that require external infrastructure (PostgreSQL, NATS, Redis) use a build tag so they don't run in the default `go test ./...` pass:

```go
//go:build integration

package postgres_test
```

Run them explicitly:

```bash
go test -tags=integration ./internal/store/postgres/...
```

---

## Test Conventions

### Use `require` for preconditions, `assert` for verifications

`require` fails the test immediately on failure — use it for setup steps where continuing would be meaningless. `assert` records the failure and continues — use it for the actual assertions so you see all failures at once.

```go
func TestCreateAccount(t *testing.T) {
    svc, cleanup := setupAccountService(t)
    t.Cleanup(cleanup)

    account, err := svc.Create(ctx, input)
    require.NoError(t, err)          // stop here if creation failed
    require.NotNil(t, account)

    assert.Equal(t, "alice", account.Username)
    assert.False(t, account.Bot)
    assert.WithinDuration(t, time.Now(), account.CreatedAt, 5*time.Second)
}
```

### Table-driven tests

Use table-driven tests when exercising the same function with varying inputs. Name each case clearly — the name appears in failure output.

```go
func TestRender(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        wantHTML string
        wantTags []string
    }{
        {
            name:     "plain text",
            input:    "hello world",
            wantHTML: "<p>hello world</p>",
        },
        {
            name:     "hashtag extraction",
            input:    "check out #golang",
            wantHTML: `<p>check out <a href="https://example.com/tags/golang" class="hashtag">#golang</a></p>`,
            wantTags: []string{"golang"},
        },
        {
            name:     "strips raw HTML",
            input:    "<script>alert('xss')</script>hello",
            wantHTML: "<p>hello</p>",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := service.Render(tt.input, mockResolver)
            require.NoError(t, err)
            assert.Equal(t, tt.wantHTML, result.HTML)
            if tt.wantTags != nil {
                assert.Equal(t, tt.wantTags, result.Tags)
            }
        })
    }
}
```

### Parallel tests

Mark tests that don't share mutable state with `t.Parallel()` for faster runs:

```go
func TestScopeSetParse(t *testing.T) {
    t.Parallel()
    // ...
}
```

### Test helpers use `t.Helper()`

Any function that calls `t.Fatal` / `t.Error` / `require` / `assert` should call `t.Helper()` so failure messages point to the caller, not the helper.

```go
func newTestStatus(t *testing.T, svc *service.StatusService, authorID string) *domain.Status {
    t.Helper()
    status, err := svc.Create(ctx, service.CreateStatusInput{
        AccountID: authorID,
        Text:      "test status",
    })
    require.NoError(t, err)
    return status
}
```

---

## Mocking Strategy

Monstera-fed's pluggable interfaces (`store.Store`, `cache.Store`, `media.Store`, `email.Sender`) are designed to be mockable. Prefer hand-written fakes for simple interfaces, and testify's `mock` package for complex interactions where you need to assert call order or arguments.

### Hand-written fake (preferred for simple interfaces)

```go
type fakeCache struct {
    data map[string][]byte
}

func (f *fakeCache) Get(_ context.Context, key string) ([]byte, error) {
    v, ok := f.data[key]
    if !ok {
        return nil, cache.ErrNotFound
    }
    return v, nil
}

func (f *fakeCache) Set(_ context.Context, key string, val []byte, _ time.Duration) error {
    f.data[key] = val
    return nil
}
```

### testify mock (for asserting call patterns)

```go
type MockEmailSender struct {
    mock.Mock
}

func (m *MockEmailSender) Send(ctx context.Context, msg email.Message) error {
    args := m.Called(ctx, msg)
    return args.Error(0)
}

func TestRegistrationSendsConfirmation(t *testing.T) {
    sender := new(MockEmailSender)
    sender.On("Send", mock.Anything, mock.MatchedBy(func(msg email.Message) bool {
        return msg.Subject == "Confirm your account"
    })).Return(nil).Once()

    svc := service.NewRegistration(sender, /* ... */)
    err := svc.Register(ctx, input)
    require.NoError(t, err)

    sender.AssertExpectations(t)
}
```

---

## HTTP Handler Tests

Use `net/http/httptest` with testify assertions:

```go
func TestGetAccount(t *testing.T) {
    router := setupTestRouter(t)

    req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/01ABC", nil)
    req.Header.Set("Authorization", "Bearer "+testToken)
    rec := httptest.NewRecorder()

    router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)

    var body map[string]any
    err := json.NewDecoder(rec.Body).Decode(&body)
    require.NoError(t, err)
    assert.Equal(t, "alice", body["username"])
}
```

---

## Linting — golangci-lint v2

### Installation

```bash
# macOS
brew install golangci-lint

# or via Go
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Configuration

Place this at the project root as `.golangci.yml`:

```yaml
version: "2"

linters:
  default: standard
  enable:
    - bodyclose
    - errname
    - errorlint
    - exhaustive
    - goconst
    - gocritic
    - gosec
    - mirror
    - misspell
    - nilerr
    - noctx
    - perfsprint
    - prealloc
    - reassign
    - revive
    - sloglint
    - sqlclosecheck
    - testifylint
    - unconvert
    - unparam
    - usestdlibvars
    - usetesting
    - wastedassign
    - wrapcheck
  disable:
    - exhaustruct       # too noisy — not all struct fields need explicit init
    - ireturn            # conflicts with Monstera-fed's interface-based abstractions
    - varnamelen         # short names like ctx, tx, db are idiomatic Go
    - wsl                # whitespace style is enforced by gofumpt instead

  settings:
    errname:
      # Matches Monstera-fed's error naming pattern (ErrSendFailed, ErrNotFound, etc.)
      rules:
        - glob: "Err*"
    exhaustive:
      default-signifies-exhaustive: true
    gocritic:
      disabled-checks:
        - ifElseChain     # switch conversion is not always clearer
    revive:
      rules:
        - name: unexported-return
          disabled: true  # returning unexported types from exported funcs is fine for internal/
    sloglint:
      attr-only: true     # enforce structured slog attributes, no fmt.Sprintf in log calls
    testifylint:
      enable-all: true    # enforce consistent testify usage (require for errors, etc.)

  exclusions:
    generated: strict     # skip sqlc-generated code
    presets:
      - common-false-positives
    rules:
      - path: _test\.go
        linters:
          - gosec         # test files don't need security scanning
          - wrapcheck     # wrapping errors in tests adds noise
      - path: internal/store/postgres/generated/
        linters:
          - ALL           # sqlc output is machine-generated

formatters:
  enable:
    - gofumpt
    - goimports

run:
  timeout: 5m
  go: "1.26"
```

### Key linter choices explained

| Linter | Why |
|--------|-----|
| `testifylint` | Enforces correct testify usage — catches `assert.NoError` where `require.NoError` is needed, flags `assert.Equal(t, nil, err)` anti-patterns |
| `sqlclosecheck` | Catches unclosed `pgx` rows — critical given Monstera-fed's heavy DB usage |
| `sloglint` | Enforces structured `slog` attributes, preventing unstructured string interpolation in log calls |
| `errorlint` | Catches `err == ErrFoo` instead of `errors.Is`, and `err.(*Foo)` instead of `errors.As` |
| `gosec` | Security scanner — catches hardcoded credentials, weak crypto, SQL injection patterns |
| `wrapcheck` | Ensures errors from external packages are wrapped with context before returning |
| `bodyclose` | Catches unclosed HTTP response bodies — important for federation HTTP clients |
| `noctx` | Catches HTTP requests without `context.Context` — all outbound calls should be cancellable |

---

## Makefile Targets

```makefile
.PHONY: test test-integration lint lint-fix

test:
	go test -race -count=1 ./...

test-integration:
	go test -race -count=1 -tags=integration ./...

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix
```

---

## CI Pipeline

Run both lint and test in CI. Lint runs first — no point running tests if the code doesn't pass static analysis.

```yaml
# Example GitHub Actions job
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"

      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: v2.9.0

      - name: Unit tests
        run: go test -race -count=1 ./...

      - name: Integration tests
        run: go test -race -count=1 -tags=integration ./...
        services:
          postgres:
            image: postgres:16-alpine
            env:
              POSTGRES_DB: monstera-fed_test
              POSTGRES_USER: monstera-fed
              POSTGRES_PASSWORD: test
          nats:
            image: nats:2-alpine
            args: ["--jetstream"]
```

---

## Coverage

Generate a coverage report without the generated code:

```bash
go test -race -coverprofile=coverage.out -coverpkg=./internal/... ./...
go tool cover -func=coverage.out
```

There is no hard coverage target. Focus coverage on:

1. **Service layer** — business logic with branching (moderation actions, content rendering, OAuth flows)
2. **AP inbox processing** — activity type dispatch, signature verification edge cases
3. **Content rendering** — HTML sanitization, mention/hashtag extraction
4. **Pagination** — cursor boundary conditions

Low-value targets to skip:

- sqlc-generated code (`internal/store/postgres/generated/`)
- Domain types (`internal/domain/`) — pure structs with no logic
- Configuration loading (`internal/config/`) — tested implicitly by integration tests
