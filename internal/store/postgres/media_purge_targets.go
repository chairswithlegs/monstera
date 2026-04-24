package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/chairswithlegs/monstera/internal/store/postgres/generated"
)

// Store surface for the shared media_purge_targets table. Used by both the
// account-deletion flow (see account_deletions.go) and the domain-block purge
// subscriber. purge_id is an opaque identifier owned by whichever flow
// emitted the rows.

func (s *PostgresStore) InsertMediaPurgeTargetsForAccount(ctx context.Context, purgeID, accountID string) error {
	return mapErr(s.q.InsertMediaPurgeTargetsForAccount(ctx, db.InsertMediaPurgeTargetsForAccountParams{
		PurgeID:   purgeID,
		AccountID: accountID,
	}))
}

func (s *PostgresStore) ListPendingMediaPurgeTargets(ctx context.Context, purgeID, cursor string, limit int) ([]string, error) {
	rows, err := s.q.ListPendingMediaPurgeTargets(ctx, db.ListPendingMediaPurgeTargetsParams{
		PurgeID: purgeID,
		Column2: cursor,
		Limit:   int32(limit), //nolint:gosec // G115: limit bounded by caller
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return rows, nil
}

func (s *PostgresStore) MarkMediaPurgeTargetDelivered(ctx context.Context, purgeID, storageKey string) error {
	return mapErr(s.q.MarkMediaPurgeTargetDelivered(ctx, db.MarkMediaPurgeTargetDeliveredParams{
		PurgeID:    purgeID,
		StorageKey: storageKey,
	}))
}

// DeleteDeliveredMediaPurgeTargets sweeps rows whose blobs have already been
// deleted and are older than the cutoff. Called by the
// purge-account-deletion-snapshots scheduler job to compensate for the
// removal of the FK to account_deletion_snapshots(id) in migration 000085.
func (s *PostgresStore) DeleteDeliveredMediaPurgeTargets(ctx context.Context, before time.Time) (int64, error) {
	n, err := s.q.DeleteDeliveredMediaPurgeTargets(ctx, pgtype.Timestamptz{Time: before, Valid: true})
	if err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}
