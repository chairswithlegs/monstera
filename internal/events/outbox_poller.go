package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/natsutil"
	"github.com/chairswithlegs/monstera/internal/store"
)

// PollerConfig controls the outbox poller behaviour.
type PollerConfig struct {
	PollInterval time.Duration
	BatchSize    int
}

// Poller reads unpublished events from the outbox table and publishes them to
// the DOMAIN_EVENTS NATS JetStream stream. It uses FOR UPDATE SKIP LOCKED to
// coordinate across multiple app instances, and NATS message-level dedup
// (Nats-Msg-Id) as a safety net.
type Poller struct {
	store store.Store
	js    jetstream.JetStream
	cfg   PollerConfig
}

// NewPoller creates a new outbox poller.
func NewPoller(s store.Store, js jetstream.JetStream, cfg PollerConfig) *Poller {
	return &Poller{store: s, js: js, cfg: cfg}
}

// Start polls the outbox table until ctx is cancelled.
func (p *Poller) Start(ctx context.Context) error {
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := p.poll(ctx); err != nil {
				slog.ErrorContext(ctx, "outbox poller: poll failed", slog.Any("error", err))
			}
		}
	}
}

func (p *Poller) poll(ctx context.Context) error {
	var events []domain.DomainEvent

	err := p.store.WithTx(ctx, func(tx store.Store) error {
		var txErr error
		events, txErr = tx.GetAndLockUnpublishedOutboxEvents(ctx, p.cfg.BatchSize)
		if txErr != nil {
			return fmt.Errorf("lock events: %w", txErr)
		}
		if len(events) == 0 {
			return nil
		}

		ids := make([]string, len(events))
		for i, e := range events {
			if err := p.publish(ctx, e); err != nil {
				return fmt.Errorf("publish event %s: %w", e.ID, err)
			}
			ids[i] = e.ID
		}

		if err := tx.MarkOutboxEventsPublished(ctx, ids); err != nil {
			return fmt.Errorf("mark published: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("outbox poller: %w", err)
	}

	if len(events) > 0 {
		slog.DebugContext(ctx, "outbox poller: published events", slog.Int("count", len(events)))
	}
	return nil
}

// publish sends a single domain event to NATS JetStream with dedup header.
func (p *Poller) publish(ctx context.Context, e domain.DomainEvent) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	subject := SubjectPrefix + e.EventType
	if err := natsutil.Publish(ctx, p.js, subject, raw, jetstream.WithMsgID(e.ID)); err != nil {
		return fmt.Errorf("publish event %s: %w", e.EventType, err)
	}
	return nil
}
