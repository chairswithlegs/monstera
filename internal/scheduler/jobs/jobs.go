package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/store"
)

// TODO: the scheduled statuses and pending cards jobs run at a fixed batch size and cadence,
// This means there is a potential for these jobs to fall behind if there is too much work to do.
// We should consider adding a scaling mechanism (e.g. a fanout) to handle this.

const (
	// maxScheduledStatusesToProcess is the maximum number of scheduled statuses to process in a single batch.
	scheduledStatusesBatchSize = 100
	// maxPendingCardsToProcess is the maximum number of pending cards to process in a single batch.
	pendingCardsBatchSize = 100
)

// ScheduledStatuses returns a handler that publishes all due scheduled statuses.
func ScheduledStatuses(svc service.StatusWriteService) func(context.Context) error {
	return func(ctx context.Context) error {
		return svc.PublishDueStatuses(ctx, scheduledStatusesBatchSize)
	}
}

// UpdateTrendingIndexes returns a job handler that refreshes the trending_statuses
// and trending_tag_history pre-computed indexes.
func UpdateTrendingIndexes(svc service.TrendsService) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return svc.RefreshIndexes(ctx)
	}
}

// ProcessPendingCards returns a job handler that fetches link preview cards for recent statuses.
func ProcessPendingCards(svc service.CardService) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := svc.ProcessPendingCards(ctx, pendingCardsBatchSize)
		if err != nil {
			return fmt.Errorf("ProcessPendingCards: %w", err)
		}
		return nil
	}
}

// CleanupOutboxEvents returns a job handler that deletes published outbox events
// older than the given retention period (e.g. 24 hours).
func CleanupOutboxEvents(s store.Store, retention time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		return s.DeletePublishedOutboxEventsBefore(ctx, time.Now().Add(-retention))
	}
}
