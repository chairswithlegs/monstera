package events

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Subject prefix for domain events published by the outbox poller.
const SubjectPrefix = "domain.events."

// Stream and consumer names.
const (
	StreamDomainEvents = "DOMAIN_EVENTS"
	ConsumerFederation = "domain-events-federation"
	ConsumerSSE        = "domain-events-sse"
)

// CreateOrUpdateStreams creates or updates the DOMAIN_EVENTS stream and its consumers.
func CreateOrUpdateStreams(ctx context.Context, js jetstream.JetStream) error {
	if _, err := js.CreateOrUpdateStream(ctx, streamConfig); err != nil {
		return fmt.Errorf("nats: create stream %s: %w", StreamDomainEvents, err)
	}
	for _, c := range consumers {
		if _, err := js.CreateOrUpdateConsumer(ctx, StreamDomainEvents, c); err != nil {
			return fmt.Errorf("nats: create consumer %s on %s: %w", c.Durable, StreamDomainEvents, err)
		}
	}
	return nil
}

var streamConfig = jetstream.StreamConfig{
	Name:       StreamDomainEvents,
	Subjects:   []string{SubjectPrefix + ">"},
	Retention:  jetstream.InterestPolicy,
	Storage:    jetstream.FileStorage,
	MaxAge:     72 * time.Hour,
	Discard:    jetstream.DiscardOld,
	Duplicates: 5 * time.Minute,
}

var consumers = []jetstream.ConsumerConfig{
	{
		Durable:       ConsumerFederation,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxAckPending: 50,
		AckWait:       30 * time.Second,
		FilterSubject: SubjectPrefix + ">",
	},
	{
		Durable:       ConsumerSSE,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxAckPending: 100,
		AckWait:       10 * time.Second,
		FilterSubject: SubjectPrefix + ">",
	},
}
