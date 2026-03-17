package natsutil

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/observability"
)

// Publish sends data to a JetStream subject and records metrics via
// observability.IncNATSPublish. opts are passed through to
// jetstream.JetStream.Publish (e.g. jetstream.WithMsgID for dedup).
func Publish(ctx context.Context, js jetstream.JetStream, subject string, data []byte, opts ...jetstream.PublishOpt) error {
	_, err := js.Publish(ctx, subject, data, opts...)
	if err != nil {
		observability.IncNATSPublish(subject, "error")
		return fmt.Errorf("nats publish to %s: %w", subject, err)
	}
	observability.IncNATSPublish(subject, "ok")
	return nil
}
