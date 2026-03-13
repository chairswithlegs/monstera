package internal

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

type outboxDLQPublisher interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}

// jsDLQPublisher adapts jetstream.JetStream to the outboxDLQPublisher interface.
type jsDLQPublisher struct {
	js jetstream.JetStream
}

func (p *jsDLQPublisher) Publish(ctx context.Context, subject string, payload []byte) error {
	_, err := p.js.Publish(ctx, subject, payload)
	if err != nil {
		return fmt.Errorf("Publish: %w", err)
	}
	return nil
}
