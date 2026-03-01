package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Subject prefixes for federation streams. Used by streams (with ">"), OutboxWorker (publish/consume), and tests.
const (
	SubjectPrefixFederationDeliver = "federation.deliver."
	SubjectPrefixFederationDLQ     = "federation.dlq."
)

const (
	subjectDeliver = SubjectPrefixFederationDeliver + ">"
	subjectDLQ     = SubjectPrefixFederationDLQ + ">"
)

// Federation stream/consumer names and limits for use by federation worker and tests.
const (
	StreamFederation         = "FEDERATION"
	StreamFederationDLQ      = "FEDERATION_DLQ"
	ConsumerFederationWorker = "federation-worker"
	MaxDeliverFederation     = 5
)

// EnsureStreams creates or updates the FEDERATION and FEDERATION_DLQ JetStream streams
// and the federation-worker durable pull consumer. Idempotent; call once at startup.
//
// The purpose of this function is to ensure NATS is properly configured at application start.
//
// Scaling: WorkQueue allows only one consumer *definition* per stream (hence one
// durable name, "federation-worker"). Each process runs a single pull loop that
// does Fetch(batch) so only one Fetch is in flight at a time; messages are
// processed concurrently within the batch. Multiple processes can run the same
// consumer; NATS distributes work. Each delivery job is processed exactly once
// (no duplicate POSTs to remote inboxes).
func EnsureStreams(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      StreamFederation,
		Subjects:  []string{subjectDeliver},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    72 * time.Hour,
		MaxBytes:  4 * 1024 * 1024,
		Discard:   jetstream.DiscardOld,
	})
	if err != nil {
		return fmt.Errorf("nats: create stream %s: %w", StreamFederation, err)
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      StreamFederationDLQ,
		Subjects:  []string{subjectDLQ},
		Retention: jetstream.LimitsPolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    30 * 24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("nats: create stream %s: %w", StreamFederationDLQ, err)
	}

	_, err = js.CreateOrUpdateConsumer(ctx, StreamFederation, jetstream.ConsumerConfig{
		Durable:       ConsumerFederationWorker,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxAckPending: 50,
		AckWait:       60 * time.Second,
		MaxDeliver:    MaxDeliverFederation,
		// These can be overridden by the worker.
		BackOff: []time.Duration{30 * time.Second, 5 * time.Minute, 30 * time.Minute},
	})
	if err != nil {
		return fmt.Errorf("nats: create consumer %s: %w", ConsumerFederationWorker, err)
	}

	return nil
}
