package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/chairswithlegs/monstera/internal/store"
	db "github.com/chairswithlegs/monstera/internal/store/postgres/generated"
)

func (s *PostgresStore) CreateAccountDeletionSnapshot(ctx context.Context, in store.CreateAccountDeletionSnapshotInput) error {
	return mapErr(s.q.CreateAccountDeletionSnapshot(ctx, db.CreateAccountDeletionSnapshotParams{
		ID:            in.ID,
		ApID:          in.APID,
		PrivateKeyPem: in.PrivateKeyPEM,
		ExpiresAt:     pgtype.Timestamptz{Time: in.ExpiresAt, Valid: true},
	}))
}

func (s *PostgresStore) GetAccountDeletionSnapshot(ctx context.Context, id string) (*store.AccountDeletionSnapshot, error) {
	row, err := s.q.GetAccountDeletionSnapshot(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	return &store.AccountDeletionSnapshot{
		ID:            row.ID,
		APID:          row.ApID,
		PrivateKeyPEM: row.PrivateKeyPem,
		CreatedAt:     pgTime(row.CreatedAt),
		ExpiresAt:     pgTime(row.ExpiresAt),
	}, nil
}

func (s *PostgresStore) InsertAccountDeletionTargetsForAccount(ctx context.Context, deletionID, accountID string) error {
	return mapErr(s.q.InsertAccountDeletionTargetsForAccount(ctx, db.InsertAccountDeletionTargetsForAccountParams{
		DeletionID: deletionID,
		TargetID:   accountID,
	}))
}

func (s *PostgresStore) ListPendingAccountDeletionTargets(ctx context.Context, deletionID, cursor string, limit int) ([]string, error) {
	rows, err := s.q.ListPendingAccountDeletionTargets(ctx, db.ListPendingAccountDeletionTargetsParams{
		DeletionID: deletionID,
		Column2:    cursor,
		Limit:      int32(limit), //nolint:gosec // G115: limit bounded by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return rows, nil
}

func (s *PostgresStore) MarkAccountDeletionTargetDelivered(ctx context.Context, deletionID, inboxURL string) error {
	return mapErr(s.q.MarkAccountDeletionTargetDelivered(ctx, db.MarkAccountDeletionTargetDeliveredParams{
		DeletionID: deletionID,
		InboxUrl:   inboxURL,
	}))
}

func (s *PostgresStore) DeleteExpiredAccountDeletionSnapshots(ctx context.Context, before time.Time) (int64, error) {
	n, err := s.q.DeleteExpiredAccountDeletionSnapshots(ctx, pgtype.Timestamptz{Time: before, Valid: true})
	if err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}

func (s *PostgresStore) InsertAccountDeletionMediaTargetsForAccount(ctx context.Context, deletionID, accountID string) error {
	return mapErr(s.q.InsertAccountDeletionMediaTargetsForAccount(ctx, db.InsertAccountDeletionMediaTargetsForAccountParams{
		DeletionID: deletionID,
		AccountID:  accountID,
	}))
}

func (s *PostgresStore) ListPendingAccountDeletionMediaTargets(ctx context.Context, deletionID, cursor string, limit int) ([]string, error) {
	rows, err := s.q.ListPendingAccountDeletionMediaTargets(ctx, db.ListPendingAccountDeletionMediaTargetsParams{
		DeletionID: deletionID,
		Column2:    cursor,
		Limit:      int32(limit), //nolint:gosec // G115: limit bounded by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return rows, nil
}

func (s *PostgresStore) MarkAccountDeletionMediaTargetDelivered(ctx context.Context, deletionID, storageKey string) error {
	return mapErr(s.q.MarkAccountDeletionMediaTargetDelivered(ctx, db.MarkAccountDeletionMediaTargetDeliveredParams{
		DeletionID: deletionID,
		StorageKey: storageKey,
	}))
}
