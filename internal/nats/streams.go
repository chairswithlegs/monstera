package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const (
	streamFederation   = "FEDERATION"
	streamFederationDLQ = "FEDERATION_DLQ"
	subjectDeliver     = "federation.deliver.>"
	subjectDLQ         = "federation.dlq.>"
	consumerFederationWorker = "federation-worker"
)

// EnsureStreams creates or updates the FEDERATION and FEDERATION_DLQ JetStream streams
// and the federation-worker durable pull consumer. Idempotent; call once at startup.
//
// Scaling: WorkQueue allows only one consumer *definition* per stream (hence one
// durable name, "federation-worker"). Many processes can share that consumer:
// each replica calls Consumer(ctx, "FEDERATION", "federation-worker") and Fetch();
// NATS delivers each message to exactly one in-flight Fetch, so replicas compete
// for work and throughput scales horizontally. Each delivery job is still
// processed exactly once (no duplicate POSTs to remote inboxes).
func EnsureStreams(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      streamFederation,
		Subjects:  []string{subjectDeliver},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    72 * time.Hour,
		MaxBytes:  4 * 1024 * 1024,
		Discard:   jetstream.DiscardOld,
	})
	if err != nil {
		return fmt.Errorf("nats: create stream %s: %w", streamFederation, err)
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      streamFederationDLQ,
		Subjects:  []string{subjectDLQ},
		Retention: jetstream.LimitsPolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    30 * 24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("nats: create stream %s: %w", streamFederationDLQ, err)
	}

	_, err = js.CreateOrUpdateConsumer(ctx, streamFederation, jetstream.ConsumerConfig{
		Durable:       consumerFederationWorker,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxAckPending: 50,
		AckWait:       60 * time.Second,
		MaxDeliver:    5,
		BackOff:       []time.Duration{0, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 12 * time.Hour},
	})
	if err != nil {
		return fmt.Errorf("nats: create consumer %s: %w", consumerFederationWorker, err)
	}

	return nil
}
