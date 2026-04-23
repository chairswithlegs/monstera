package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/chairswithlegs/monstera/internal/service"
)

// PurgeAccountDeletionSnapshots returns a handler that drops expired rows from
// account_deletion_snapshots. CASCADE wipes the associated
// account_deletion_targets rows along with each snapshot, reclaiming the
// private-key material once the Delete{Actor} fanout/delivery window has
// closed.
//
// The TTL itself is set at delete time (see
// service.AccountDeletionSnapshotTTL) — the scheduler merely sweeps anything
// past it.
func PurgeAccountDeletionSnapshots(svc service.AccountDeletionService) func(context.Context) error {
	return func(ctx context.Context) error {
		n, err := svc.PurgeExpiredSnapshots(ctx, time.Now())
		if err != nil {
			return fmt.Errorf("PurgeAccountDeletionSnapshots: %w", err)
		}
		if n > 0 {
			slog.InfoContext(ctx, "purged expired account deletion snapshots", slog.Int64("count", n))
		}
		return nil
	}
}
