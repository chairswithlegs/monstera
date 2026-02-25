package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	db "github.com/chairswithlegs/monstera-fed/internal/store/postgres/generated"
)

// PostgresStore wraps the sqlc-generated *db.Queries and satisfies store.Store.
// It holds the pool so that WithTx can begin new transactions.
type PostgresStore struct {
	*db.Queries
	pool *pgxpool.Pool
}

// New constructs a PostgresStore from an open pool.
func New(pool *pgxpool.Pool) store.Store {
	return &PostgresStore{
		Queries: db.New(pool),
		pool:    pool,
	}
}

// WithTx begins a transaction, wraps the connection in a new *db.Queries, and
// calls fn with a transaction-scoped Store. Commits on nil return from fn;
// rolls back on any error.
func (s *PostgresStore) WithTx(ctx context.Context, fn func(store.Store) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txStore := &PostgresStore{
		Queries: db.New(tx),
		pool:    s.pool,
	}
	if err := fn(txStore); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// mapErr translates pgx and PostgreSQL errors to domain errors.
// Callers should use this when delegating to s.Queries from wrapper methods.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}
