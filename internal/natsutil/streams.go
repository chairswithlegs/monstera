package natsutil

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

// StreamConfig describes a JetStream stream and its associated consumers.
// Each subsystem (events, activitypub, scheduler) defines its own configs;
// EnsureStreams handles the registration.
type StreamConfig struct {
	Stream    jetstream.StreamConfig
	Consumers []jetstream.ConsumerConfig
}

// EnsureStreams creates or updates JetStream streams and their consumers.
func EnsureStreams(ctx context.Context, js jetstream.JetStream, configs ...StreamConfig) error {
	for _, cfg := range configs {
		if _, err := js.CreateOrUpdateStream(ctx, cfg.Stream); err != nil {
			return fmt.Errorf("nats: create stream %s: %w", cfg.Stream.Name, err)
		}
		for _, c := range cfg.Consumers {
			if _, err := js.CreateOrUpdateConsumer(ctx, cfg.Stream.Name, c); err != nil {
				return fmt.Errorf("nats: create consumer %s on %s: %w", c.Durable, cfg.Stream.Name, err)
			}
		}
	}
	return nil
}
