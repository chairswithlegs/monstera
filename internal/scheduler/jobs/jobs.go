package jobs

import (
	"context"
	"fmt"
	"time"

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
