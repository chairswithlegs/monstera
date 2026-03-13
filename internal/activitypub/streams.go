package activitypub

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/activitypub/internal"
	"github.com/nats-io/nats.go/jetstream"
)

// CreateOrUpdateStreams creates or updates the activitypub outbound streams and consumers.
func CreateOrUpdateStreams(ctx context.Context, js jetstream.JetStream) error {
	for _, d := range internal.Streams {
		if _, err := js.CreateOrUpdateStream(ctx, d.Stream); err != nil {
			return fmt.Errorf("nats: create stream %s: %w", d.Stream.Name, err)
		}
		if d.Consumer != nil {
			if _, err := js.CreateOrUpdateConsumer(ctx, d.Stream.Name, *d.Consumer); err != nil {
				return fmt.Errorf("nats: create consumer %s on %s: %w", d.Consumer.Durable, d.Stream.Name, err)
			}
		}
	}
	return nil
}
