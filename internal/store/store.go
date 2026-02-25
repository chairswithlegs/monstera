package store

import (
	"context"

	db "github.com/chairswithlegs/monstera-fed/internal/store/postgres/generated"
)

// Store is the single dependency that services take for all database access.
// It composes the sqlc Querier and adds transaction support.
type Store interface {
	db.Querier
	WithTx(ctx context.Context, fn func(Store) error) error
}
