# Interface and Implementation

Interfaces and their implementations should be colocated — either in the same file or with the implementation(s) in a nested package.

## Same file (simple implementations)

The implementation type is unexported; the constructor returns the interface.

```go
type AccountService interface {
	GetByID(ctx context.Context, id string) (*domain.Account, error)
}

type accountService struct{ store store.Store }

func NewAccountService(s store.Store) AccountService {
	return &accountService{store: s}
}
```

## Nested package (large or multiple implementations)

Interface and shared input/result types live in the parent package (`internal/store/store.go`). The implementation lives in a child package (`internal/store/postgres/`). The constructor returns the parent interface type so callers depend only on the parent.

```go
// internal/store/postgres/store.go
package postgres

func New(pool *pgxpool.Pool) store.Store {
	return &PostgresStore{q: db.New(pool), pool: pool}
}
```
