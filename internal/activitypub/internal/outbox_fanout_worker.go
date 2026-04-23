package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/natsutil"
	"github.com/chairswithlegs/monstera/internal/service"
)

const (
	activityTypeUnknown = "unknown"
	fanoutPageSize      = 500
)

// OutboxFanoutMessage is the payload for async fan-out: one message per activity to be delivered to all followers.
//
// Exactly one of SenderID and DeletionID is set:
//   - SenderID — the normal path. The worker reads follower inbox URLs from
//     the live follows/accounts tables via RemoteFollowService.
//   - DeletionID — a Delete{Actor} emitted by deleteLocalAccount. The accounts
//     row and its follows are gone by the time this message is processed;
//     the worker reads pre-captured inbox URLs from
//     account_deletion_targets via AccountDeletionService, and the delivery
//     worker signs with the snapshot's private key via AccountDeletionService.
type OutboxFanoutMessage struct {
	ActivityID string          `json:"activity_id"`
	Activity   json.RawMessage `json:"activity"`

	SenderID   string `json:"sender_id,omitempty"`
	DeletionID string `json:"deletion_id,omitempty"`
}

// OutboxFanoutWorker consumes from the ACTIVITYPUB_FANOUT stream and fans out for delivery.
type OutboxFanoutWorker interface {
	Publish(ctx context.Context, activityType string, msg OutboxFanoutMessage) error
	Start(ctx context.Context) error
}

type outboxFanoutWorker struct {
	js                jetstream.JetStream
	followers         service.RemoteFollowService
	deletions         service.AccountDeletionService
	delivery          OutboxDeliveryWorker
	workerConcurrency int
}

// NewOutboxFanoutWorker constructs an outbox fan-out worker.
func NewOutboxFanoutWorker(
	js jetstream.JetStream,
	followers service.RemoteFollowService,
	deletions service.AccountDeletionService,
	delivery OutboxDeliveryWorker,
	workerConcurrency int,
) OutboxFanoutWorker {
	return &outboxFanoutWorker{
		js:                js,
		followers:         followers,
		deletions:         deletions,
		delivery:          delivery,
		workerConcurrency: workerConcurrency,
	}
}

// Start consumes from the ACTIVITYPUB_FANOUT stream and processes each message with paginated fan-out.
func (w *outboxFanoutWorker) Start(ctx context.Context) error {
	concurrency := 3
	if w.workerConcurrency > 0 {
		concurrency = w.workerConcurrency
	}
	if concurrency > 5 {
		concurrency = 5
	}
	if err := natsutil.RunConsumer(ctx, w.js, StreamOutboxFanout, consumerFanout,
		func(msg jetstream.Msg) { go w.processMessage(ctx, msg) },
		natsutil.WithMaxMessages(concurrency),
		natsutil.WithLabel("activitypub fanout worker"),
	); err != nil {
		return fmt.Errorf("fanout worker: %w", err)
	}
	return nil
}

// Publish publishes a fan-out message to the stream. The worker will later consume it and fan out to follower inboxes.
func (w *outboxFanoutWorker) Publish(ctx context.Context, activityType string, msg OutboxFanoutMessage) error {
	slog.DebugContext(ctx, "outbox fanout worker: publishing message", slog.String("activity_type", activityType), slog.String("activity_id", msg.ActivityID))

	subject := subjectPrefixFanout + strings.ToLower(activityType)
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("activitypub: marshal fanout message: %w", err)
	}
	if err := natsutil.Publish(ctx, w.js, subject, data); err != nil {
		return fmt.Errorf("fanout publish: %w", err)
	}
	return nil
}

func (w *outboxFanoutWorker) processMessage(ctx context.Context, msg jetstream.Msg) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "outbox fanout worker: panic in processMessage", slog.Any("panic", r), slog.String("subject", msg.Subject()))
			_ = msg.Nak()
		}
	}()
	slog.DebugContext(ctx, "outbox fanout worker: processing message", slog.String("subject", msg.Subject()))

	var fanout OutboxFanoutMessage
	if err := json.Unmarshal(msg.Data(), &fanout); err != nil {
		slog.WarnContext(ctx, "fanout worker: invalid payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}

	activityType := w.getActivityType(msg.Subject())

	cursor := ""
	delivered := 0
	for {
		page, err := w.listTargets(ctx, &fanout, cursor)
		if err != nil {
			slog.WarnContext(ctx, "fanout worker: list targets failed",
				slog.String("sender_id", fanout.SenderID),
				slog.String("deletion_id", fanout.DeletionID),
				slog.String("activity_id", fanout.ActivityID),
				slog.Any("error", err),
			)
			w.handleFanoutFailure(ctx, msg, &fanout, activityType, nil)
			return
		}
		if len(page) == 0 {
			break
		}
		for _, inbox := range page {
			if inbox == "" {
				continue
			}
			dm := OutboxDeliveryMessage{
				ActivityID:  fanout.ActivityID,
				Activity:    fanout.Activity,
				TargetInbox: inbox,
				SenderID:    fanout.SenderID,
				DeletionID:  fanout.DeletionID,
			}
			if err := w.delivery.Publish(ctx, activityType, dm); err != nil {
				slog.WarnContext(ctx, "fanout worker: enqueue delivery failed",
					slog.String("activity_id", fanout.ActivityID),
					slog.String("target_inbox", inbox),
					slog.String("deletion_id", fanout.DeletionID),
					slog.Any("error", err),
				)
				w.handleFanoutFailure(ctx, msg, &fanout, activityType, nil)
				return
			}
			if fanout.DeletionID != "" {
				// Mark target as enqueued so a redelivery of this fanout
				// message (e.g. after a worker crash + NATS resend) doesn't
				// re-enqueue the same inbox. Failure to mark is non-fatal:
				// the per-inbox HTTP delivery has its own idempotency at the
				// receiving server, and the TTL-based snapshot GC bounds the
				// window.
				if mErr := w.deletions.MarkTargetDelivered(ctx, fanout.DeletionID, inbox); mErr != nil {
					slog.WarnContext(ctx, "fanout worker: mark deletion target delivered failed",
						slog.String("deletion_id", fanout.DeletionID),
						slog.String("target_inbox", inbox),
						slog.Any("error", mErr),
					)
				}
			}
			delivered++
		}
		cursor = page[len(page)-1]
		if len(page) < fanoutPageSize {
			break
		}
	}

	_ = msg.Ack()
}

// listTargets returns the next page of follower inbox URLs to deliver to.
// For a normal fanout (SenderID set) it reads from the live follows+accounts
// tables. For an account-deletion fanout (DeletionID set) it reads from the
// account_deletion_targets snapshot that was populated in the delete tx.
func (w *outboxFanoutWorker) listTargets(ctx context.Context, fanout *OutboxFanoutMessage, cursor string) ([]string, error) {
	if fanout.DeletionID != "" {
		return w.deletions.ListPendingTargets(ctx, fanout.DeletionID, cursor, fanoutPageSize)
	}
	return w.followers.GetFollowerInboxURLsPaginated(ctx, fanout.SenderID, cursor, fanoutPageSize)
}

// handleFanoutFailure NAKs with backoff for retry, or sends to fanout DLQ and Acks if max retries exhausted.
func (w *outboxFanoutWorker) handleFanoutFailure(ctx context.Context, msg jetstream.Msg, fanout *OutboxFanoutMessage, activityType string, meta *jetstream.MsgMetadata) {
	if meta == nil {
		var err error
		meta, err = msg.Metadata()
		if err != nil {
			natsutil.NAKWithBackoff(msg, nil, fanoutRetries)
			return
		}
	}
	if meta.NumDelivered >= uint64(len(fanoutRetries)) {
		if err := w.sendToFanoutDLQ(ctx, activityType, fanout); err != nil {
			slog.WarnContext(ctx, "fanout worker: publish DLQ failed", slog.String("activity_id", fanout.ActivityID), slog.Any("error", err))
		}
		_ = msg.Ack()
		return
	}
	natsutil.NAKWithBackoff(msg, meta, fanoutRetries)
}

// sendToFanoutDLQ copies a failed fanout message to the fanout DLQ stream.
func (w *outboxFanoutWorker) sendToFanoutDLQ(ctx context.Context, activityType string, fanout *OutboxFanoutMessage) error {
	subject := subjectPrefixFanoutDLQ + strings.ToLower(activityType)
	data, err := json.Marshal(fanout)
	if err != nil {
		return fmt.Errorf("activitypub: marshal fanout DLQ message: %w", err)
	}
	if err := natsutil.Publish(ctx, w.js, subject, data); err != nil {
		return fmt.Errorf("fanout DLQ publish: %w", err)
	}
	return nil
}

func (w *outboxFanoutWorker) getActivityType(subject string) string {
	if strings.HasPrefix(subject, subjectPrefixFanout) {
		return strings.TrimPrefix(subject, subjectPrefixFanout)
	}
	return activityTypeUnknown
}
