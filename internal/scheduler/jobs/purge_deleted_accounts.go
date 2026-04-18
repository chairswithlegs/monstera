package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
)

const purgeDeletedAccountsBatchSize = 20

// PurgeDeletedAccounts returns a handler that finds local accounts whose
// soft-delete grace period has elapsed and hard-deletes them via the account
// service. Individual failures are logged and skipped so one bad row doesn't
// block the batch.
func PurgeDeletedAccounts(s store.Store, svc service.AccountService, grace time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		return drainBatches(ctx, purgeDeletedAccountsBatchSize, func(ctx context.Context, limit int) (int, error) {
			ids, err := s.ListLocalAccountsPastDeletionGrace(ctx, time.Now().Add(-grace), limit)
			if err != nil {
				return 0, fmt.Errorf("PurgeDeletedAccounts: %w", err)
			}
			for _, id := range ids {
				if purgeErr := svc.PurgeAccount(ctx, id); purgeErr != nil {
					slog.WarnContext(ctx, "PurgeDeletedAccounts: purge failed",
						slog.String("account_id", id),
						slog.Any("error", purgeErr),
					)
				}
			}
			return len(ids), nil
		})
	}
}
