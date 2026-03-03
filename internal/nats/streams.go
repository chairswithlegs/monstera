package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Subject prefixes for the activitypub streams. Used by streams (with ">").
const (
	SubjectPrefixActivityPubDeliver = "activitypub.deliver."
	SubjectPrefixActivityPubDLQ     = "activitypub.dlq."
)

// ActivityPub stream/consumer names and limits for use by the activitypub worker and tests.
const (
	StreamActivityPub         = "ACTIVITYPUB"
	StreamActivityPubDLQ      = "ACTIVITYPUB_DLQ"
	ConsumerActivityPubWorker = "activitypub-worker"
	MaxDeliverActivityPub     = 5
)

// EnsureStreams creates or updates the JetStream streams and durable consumers.
//
// The purpose of this function is to ensure NATS is properly configured at application start.
func EnsureStreams(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      StreamActivityPub,
		Subjects:  []string{SubjectPrefixActivityPubDeliver + ">"},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    72 * time.Hour,
		MaxBytes:  4 * 1024 * 1024,
		Discard:   jetstream.DiscardOld,
	})
	if err != nil {
		return fmt.Errorf("nats: create stream %s: %w", StreamActivityPub, err)
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      StreamActivityPubDLQ,
		Subjects:  []string{SubjectPrefixActivityPubDLQ + ">"},
		Retention: jetstream.LimitsPolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    30 * 24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("nats: create stream %s: %w", StreamActivityPubDLQ, err)
	}

	// Scaling: WorkQueue allows only one consumer *definition* per stream (hence one
	// durable name, "activitypub-worker"). Each process runs a single pull loop that
	// does Fetch(batch) so only one Fetch is in flight at a time; messages are
	// processed concurrently within the batch. Multiple processes can run the same
	// consumer; NATS distributes work. Each delivery job is processed exactly once.
	_, err = js.CreateOrUpdateConsumer(ctx, StreamActivityPub, jetstream.ConsumerConfig{
		Durable:       ConsumerActivityPubWorker,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxAckPending: 50,
		AckWait:       60 * time.Second,
		MaxDeliver:    MaxDeliverActivityPub,
		// These can be overridden by the worker.
		BackOff: []time.Duration{30 * time.Second, 5 * time.Minute, 30 * time.Minute},
	})
	if err != nil {
		return fmt.Errorf("nats: create consumer %s: %w", ConsumerActivityPubWorker, err)
	}

	return nil
}
