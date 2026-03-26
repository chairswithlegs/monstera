package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
)

const (
	scheduledStatusesBatchSize = 100
	pendingCardsBatchSize      = 100
)

// ScheduledStatuses returns a handler that publishes all due scheduled statuses,
// processing in batches until no more are due.
func ScheduledStatuses(svc service.ScheduledStatusService) func(context.Context) error {
	return func(ctx context.Context) error {
		return drainBatches(ctx, scheduledStatusesBatchSize, func(ctx context.Context, limit int) (int, error) {
			return svc.PublishDueStatuses(ctx, limit)
		})
	}
}

// UpdateTrendingIndexes returns a job handler that refreshes the trending_statuses
// and trending_tag_history pre-computed indexes.
func UpdateTrendingIndexes(svc service.TrendsService) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return svc.RefreshIndexes(ctx)
	}
}

// ProcessPendingCards returns a job handler that fetches link preview cards for
// recent statuses, processing in batches until no more are pending.
func ProcessPendingCards(svc service.CardService) func(context.Context) error {
	return func(ctx context.Context) error {
		return drainBatches(ctx, pendingCardsBatchSize, func(ctx context.Context, limit int) (int, error) {
			return svc.ProcessPendingCards(ctx, limit)
		})
	}
}

// CleanupOutboxEvents returns a job handler that deletes published outbox events
// older than the given retention period (e.g. 24 hours).
func CleanupOutboxEvents(s store.Store, retention time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		return s.DeletePublishedOutboxEventsBefore(ctx, time.Now().Add(-retention))
	}
}

// CrawlRemoteAccounts returns a job handler that walks the social graph outward
// from a random local account, enqueuing backfill jobs for each hop.
func CrawlRemoteAccounts(s store.Store, backfill service.BackfillService) func(context.Context) error {
	return func(ctx context.Context) error {
		// Get a random local account to start from.
		localAccount, err := s.GetRandomLocalAccount(ctx)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("CrawlRemoteAccounts GetRandomLocalAccount: %w", err)
		}

		// Crawl up to n hops of remote follows, backfilling each hop.
		const maxHops = 2
		prevHop := localAccount
		var hop *domain.Account
		for i := 0; i < maxHops; i++ {
			hop, err = s.GetRandomFollowTarget(ctx, prevHop.ID)
			if err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					return nil
				}
				return fmt.Errorf("CrawlRemoteAccounts GetRandomFollowTarget(hop%d): %w", i, err)
			}

			if hop.IsLocal() {
				break
			}

			slog.InfoContext(ctx, "crawl-remote-accounts", slog.String("hop", hop.ID))

			if err := backfill.RequestBackfill(ctx, hop.ID); err != nil {
				return fmt.Errorf("CrawlRemoteAccounts RequestBackfill(hop%d): %w", i, err)
			}

			prevHop = hop
		}

		return nil
	}
}

// drainBatches calls processFn repeatedly with the given batch size until a
// batch returns fewer items than the limit (meaning the queue is drained) or
// the context is cancelled.
func drainBatches(ctx context.Context, batchSize int, processFn func(ctx context.Context, limit int) (int, error)) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("drainBatches: %w", err)
		}
		processed, err := processFn(ctx, batchSize)
		if err != nil {
			return err
		}
		if processed < batchSize {
			return nil
		}
	}
}
