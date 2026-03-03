package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	natsutil "github.com/chairswithlegs/monstera-fed/internal/nats"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
	"github.com/chairswithlegs/monstera-fed/internal/store"
)

// jsDLQPublisher adapts jetstream.JetStream to the outboxFanoutDLQPublisher interface.
type jsDLQPublisher struct {
	js jetstream.JetStream
}

func (p *jsDLQPublisher) Publish(ctx context.Context, subject string, payload []byte) error {
	_, err := p.js.Publish(ctx, subject, payload)
	return err
}

const fanoutPageSize = 500

// outboxFanoutMessage is the payload for async fan-out: one message per activity to be delivered to all followers.
type outboxFanoutMessage struct {
	ActivityID string          `json:"activity_id"`
	Activity   json.RawMessage `json:"activity"`

	SenderID string `json:"sender_id"`
}

// outboxFanoutPublisher enqueues fan-out messages; the worker consumes and fans out to follower inboxes.
type outboxFanoutPublisher interface {
	publish(ctx context.Context, activityType string, msg outboxFanoutMessage) error
}

// outboxFanoutDLQPublisher publishes messages to the fanout DLQ stream.
type outboxFanoutDLQPublisher interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}

// newOutboxFanoutWorker constructs an outbox fan-out worker. Call start to begin consuming from ACTIVITYPUB_FANOUT.
func newOutboxFanoutWorker(
	js jetstream.JetStream,
	s store.Store,
	delivery outboxDeliveryPublisher,
	cfg *config.Config,
) *outboxFanoutWorker {
	return &outboxFanoutWorker{
		js:           js,
		store:        s,
		delivery:     delivery,
		dlqPublisher: &jsDLQPublisher{js: js},
		cfg:          cfg,
	}
}

type outboxFanoutWorker struct {
	js           jetstream.JetStream
	store        store.Store
	delivery     outboxDeliveryPublisher
	dlqPublisher outboxFanoutDLQPublisher
	cfg          *config.Config
}

// start consumes from the ACTIVITYPUB_FANOUT stream and processes each message with paginated fan-out.
func (w *outboxFanoutWorker) start(ctx context.Context) error {
	consumer, err := w.js.Consumer(ctx, streamFanout, consumerFanout)
	if err != nil {
		return fmt.Errorf("activitypub fanout worker: get consumer: %w", err)
	}

	concurrency := 3
	if w.cfg != nil && w.cfg.FederationWorkerConcurrency > 0 {
		concurrency = w.cfg.FederationWorkerConcurrency
	}
	if concurrency > 5 {
		concurrency = 5
	}

	slog.Info("activitypub fanout worker started",
		slog.Int("concurrency", concurrency),
		slog.String("consumer", consumerFanout),
	)

	consCtx, err := consumer.Consume(
		func(msg jetstream.Msg) {
			go w.processMessage(ctx, msg)
		},
		jetstream.PullMaxMessages(concurrency),
		jetstream.PullExpiry(5*time.Second),
		jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
			if ctx.Err() == nil {
				slog.Warn("fanout worker consume error", slog.Any("error", err))
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("activitypub fanout worker: consume: %w", err)
	}

	<-ctx.Done()
	slog.Info("activitypub fanout worker stopping")
	consCtx.Stop()
	<-consCtx.Closed()
	return nil
}

// publish publishes a fan-out message to the stream. The worker will later consume it and fan out to follower inboxes.
func (w *outboxFanoutWorker) publish(ctx context.Context, activityType string, msg outboxFanoutMessage) (err error) {
	subject := subjectPrefixFanout + strings.ToLower(activityType)
	defer func() {
		if err != nil {
			observability.IncNATSPublish(subject, "error")
		} else {
			observability.IncNATSPublish(subject, "ok")
		}
	}()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("activitypub: marshal fanout message: %w", err)
	}

	_, err = w.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("activitypub: publish fanout to %s: %w", subject, err)
	}
	return nil
}

func (w *outboxFanoutWorker) processMessage(ctx context.Context, msg jetstream.Msg) {
	var fanout outboxFanoutMessage
	if err := json.Unmarshal(msg.Data(), &fanout); err != nil {
		slog.Warn("fanout worker: invalid payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}

	activityType := w.getActivityType(msg.Subject())

	cursor := ""
	delivered := 0
	for {
		page, err := w.store.GetDistinctFollowerInboxURLsPaginated(ctx, fanout.SenderID, cursor, fanoutPageSize)
		if err != nil {
			slog.Warn("fanout worker: get follower inboxes failed",
				slog.String("sender_id", fanout.SenderID),
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
			dm := outboxDeliveryMessage{
				ActivityID:  fanout.ActivityID,
				Activity:    fanout.Activity,
				TargetInbox: inbox,
				SenderID:    fanout.SenderID,
			}
			if err := w.delivery.publish(ctx, activityType, dm); err != nil {
				slog.Warn("fanout worker: enqueue delivery failed",
					slog.String("activity_id", fanout.ActivityID),
					slog.String("target_inbox", inbox),
					slog.Any("error", err),
				)
				w.handleFanoutFailure(ctx, msg, &fanout, activityType, nil)
				return
			}
			delivered++
		}
		cursor = page[len(page)-1]
		if len(page) < fanoutPageSize {
			break
		}
	}

	_ = msg.Ack()
	slog.DebugContext(ctx, "fanout worker: completed",
		slog.String("activity_id", fanout.ActivityID),
		slog.String("sender_id", fanout.SenderID),
		slog.Int("delivered", delivered),
	)
}

// handleFanoutFailure NAKs with backoff for retry, or sends to fanout DLQ and Acks if max retries exhausted.
func (w *outboxFanoutWorker) handleFanoutFailure(ctx context.Context, msg jetstream.Msg, fanout *outboxFanoutMessage, activityType string, meta *jetstream.MsgMetadata) {
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
			slog.Warn("fanout worker: publish DLQ failed", slog.String("activity_id", fanout.ActivityID), slog.Any("error", err))
		}
		_ = msg.Ack()
		return
	}
	natsutil.NAKWithBackoff(msg, meta, fanoutRetries)
}

// sendToFanoutDLQ copies a failed fanout message to the fanout DLQ stream.
func (w *outboxFanoutWorker) sendToFanoutDLQ(ctx context.Context, activityType string, fanout *outboxFanoutMessage) (err error) {
	subject := subjectPrefixFanoutDLQ + strings.ToLower(activityType)
	defer func() {
		if err != nil {
			observability.IncNATSPublish(subject, "error")
		} else {
			observability.IncNATSPublish(subject, "ok")
		}
	}()

	data, err := json.Marshal(fanout)
	if err != nil {
		return fmt.Errorf("activitypub: marshal fanout DLQ message: %w", err)
	}

	if err = w.dlqPublisher.Publish(ctx, subject, data); err != nil {
		return fmt.Errorf("activitypub: publish fanout DLQ to %s: %w", subject, err)
	}
	return nil
}

func (w *outboxFanoutWorker) getActivityType(subject string) string {
	if strings.HasPrefix(subject, subjectPrefixFanout) {
		return strings.TrimPrefix(subject, subjectPrefixFanout)
	}
	return "unknown"
}
