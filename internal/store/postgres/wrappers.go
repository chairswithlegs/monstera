package postgres

import (
	"context"

	db "github.com/chairswithlegs/monstera/internal/store/postgres/generated"
)

// Wrapper methods translate pgx/pgconn errors to domain errors (ErrNotFound, ErrConflict).
// They shadow the embedded *db.Queries methods so store callers receive domain errors.

func (s *PostgresStore) GetEmailToken(ctx context.Context, tokenHash string) (db.EmailToken, error) {
	t, err := s.q.GetEmailToken(ctx, tokenHash)
	return t, mapErr(err)
}

func (s *PostgresStore) GetHashtagByName(ctx context.Context, lower string) (db.Hashtag, error) {
	h, err := s.q.GetHashtagByName(ctx, lower)
	return h, mapErr(err)
}

func (s *PostgresStore) GetReport(ctx context.Context, id string) (db.Report, error) {
	r, err := s.q.GetReport(ctx, id)
	return r, mapErr(err)
}

func (s *PostgresStore) CountRemoteAccounts(ctx context.Context) (int64, error) {
	n, err := s.q.CountRemoteAccounts(ctx)
	return n, mapErr(err)
}

func (s *PostgresStore) CreateEmailToken(ctx context.Context, arg db.CreateEmailTokenParams) (db.EmailToken, error) {
	t, err := s.q.CreateEmailToken(ctx, arg)
	return t, mapErr(err)
}
