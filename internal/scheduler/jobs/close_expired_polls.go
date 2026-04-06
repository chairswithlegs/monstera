package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

const expiredPollBatchSize = 50

// CloseExpiredPolls returns a handler that finds polls past their expiry time
// and emits EventPollExpired for each, triggering federation updates and
// notifications. It processes in batches until no more expired polls are found.
func CloseExpiredPolls(s store.Store) func(context.Context) error {
	return func(ctx context.Context) error {
		return drainBatches(ctx, expiredPollBatchSize, func(ctx context.Context, limit int) (int, error) {
			statusIDs, err := s.ListExpiredOpenPollStatusIDs(ctx, limit)
			if err != nil {
				return 0, fmt.Errorf("CloseExpiredPolls: %w", err)
			}
			for _, statusID := range statusIDs {
				if emitErr := emitPollExpired(ctx, s, statusID); emitErr != nil {
					slog.WarnContext(ctx, "CloseExpiredPolls: emit failed", slog.String("status_id", statusID), slog.Any("error", emitErr))
				}
			}
			return len(statusIDs), nil
		})
	}
}

func emitPollExpired(ctx context.Context, s store.Store, statusID string) error {
	st, err := s.GetStatusByID(ctx, statusID)
	if err != nil {
		return fmt.Errorf("GetStatusByID: %w", err)
	}
	author, err := s.GetAccountByID(ctx, st.AccountID)
	if err != nil {
		return fmt.Errorf("GetAccountByID: %w", err)
	}
	poll, err := s.GetPollByStatusID(ctx, statusID)
	if err != nil {
		return fmt.Errorf("GetPollByStatusID: %w", err)
	}
	opts, err := s.ListPollOptions(ctx, poll.ID)
	if err != nil {
		return fmt.Errorf("ListPollOptions: %w", err)
	}
	votersCount, err := s.CountDistinctVoters(ctx, poll.ID)
	if err != nil {
		return fmt.Errorf("CountDistinctVoters: %w", err)
	}
	mentions, err := s.GetStatusMentions(ctx, statusID)
	if err != nil {
		return fmt.Errorf("GetStatusMentions: %w", err)
	}
	tags, err := s.GetStatusHashtags(ctx, statusID)
	if err != nil {
		return fmt.Errorf("GetStatusHashtags: %w", err)
	}
	media, err := s.GetStatusAttachments(ctx, statusID)
	if err != nil {
		return fmt.Errorf("GetStatusAttachments: %w", err)
	}
	// Close the poll and emit the event in one transaction so the payload
	// reflects the closed state and the poll can't be re-processed.
	return s.WithTx(ctx, func(tx store.Store) error {
		if err := tx.ClosePoll(ctx, poll.ID); err != nil {
			return fmt.Errorf("ClosePoll: %w", err)
		}
		// Set ClosedAt on the struct so the outbound AP Question includes the closed timestamp.
		now := time.Now()
		poll.ClosedAt = &now

		payload := domain.PollUpdatedPayload{
			Status:      st,
			Author:      author,
			Poll:        poll,
			PollOptions: opts,
			VotersCount: votersCount,
			Mentions:    mentions,
			Tags:        tags,
			Media:       media,
			Local:       st.IsLocal(),
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		return tx.InsertOutboxEvent(ctx, store.InsertOutboxEventInput{
			ID:            uid.New(),
			EventType:     domain.EventPollExpired,
			AggregateType: "poll",
			AggregateID:   poll.ID,
			Payload:       raw,
		})
	})
}
