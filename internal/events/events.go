// Package events provides the transactional outbox, event poller, and
// domain-event subscribers (notifications, SSE).
package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// EmitEvent writes a domain event to the transactional outbox within the
// current transaction. The event is published to NATS by the outbox poller.
func EmitEvent(ctx context.Context, tx store.Store, eventType, aggregateType, aggregateID string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s event payload: %w", eventType, err)
	}
	if err := tx.InsertOutboxEvent(ctx, store.InsertOutboxEventInput{
		ID:            uid.New(),
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       raw,
	}); err != nil {
		return fmt.Errorf("emit %s event: %w", eventType, err)
	}
	return nil
}
