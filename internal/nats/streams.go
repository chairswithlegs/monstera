package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Subject prefixes for the activitypub streams. Used by streams (with ">").
// DLQ subjects use sibling *dlq. prefix to avoid overlap with parent stream wildcards.
const (
	SubjectPrefixActivityPubOutboundDeliver    = "activitypub.outbound.deliver."
	SubjectPrefixActivityPubOutboundDeliverDLQ = "activitypub.outbound.deliverdlq."
	SubjectPrefixActivityPubOutboundFanout     = "activitypub.outbound.fanout."
	SubjectPrefixActivityPubOutboundFanoutDLQ  = "activitypub.outbound.fanoutdlq."
)

// Outbound ActivityPub stream/consumer names and limits (delivery to remote inboxes, fan-out, DLQ).
const (
	StreamActivityPubOutboundDelivery     = "ACTIVITYPUB_OUTBOUND_DELIVERY"
	StreamActivityPubOutboundDeliveryDLQ  = "ACTIVITYPUB_OUTBOUND_DELIVERY_DLQ"
	StreamActivityPubOutboundFanout       = "ACTIVITYPUB_OUTBOUND_FANOUT"
	StreamActivityPubOutboundFanoutDLQ    = "ACTIVITYPUB_OUTBOUND_FANOUT_DLQ"
	ConsumerActivityPubOutboundDelivery   = "activitypub-outbound-delivery"
	ConsumerActivityPubOutboundFanout     = "activitypub-outbound-fanout"
	MaxDeliverActivityPubOutboundDelivery = 5
	MaxDeliverActivityPubOutboundFanout   = 2
)

// EnsureStreams creates or updates the JetStream streams and durable consumers.
//
// The purpose of this function is to ensure NATS is properly configured at application start.
func EnsureStreams(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      StreamActivityPubOutboundDelivery,
		Subjects:  []string{SubjectPrefixActivityPubOutboundDeliver + ">"},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    72 * time.Hour,
		MaxBytes:  4 * 1024 * 1024,
		Discard:   jetstream.DiscardOld,
	})
	if err != nil {
		return fmt.Errorf("nats: create stream %s: %w", StreamActivityPubOutboundDelivery, err)
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      StreamActivityPubOutboundDeliveryDLQ,
		Subjects:  []string{SubjectPrefixActivityPubOutboundDeliverDLQ + ">"},
		Retention: jetstream.LimitsPolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    30 * 24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("nats: create stream %s: %w", StreamActivityPubOutboundDeliveryDLQ, err)
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      StreamActivityPubOutboundFanout,
		Subjects:  []string{SubjectPrefixActivityPubOutboundFanout + ">"},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    72 * time.Hour,
		MaxBytes:  4 * 1024 * 1024,
		Discard:   jetstream.DiscardOld,
	})
	if err != nil {
		return fmt.Errorf("nats: create stream %s: %w", StreamActivityPubOutboundFanout, err)
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      StreamActivityPubOutboundFanoutDLQ,
		Subjects:  []string{SubjectPrefixActivityPubOutboundFanoutDLQ + ">"},
		Retention: jetstream.LimitsPolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    30 * 24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("nats: create stream %s: %w", StreamActivityPubOutboundFanoutDLQ, err)
	}

	_, err = js.CreateOrUpdateConsumer(ctx, StreamActivityPubOutboundDelivery, jetstream.ConsumerConfig{
		Durable:       ConsumerActivityPubOutboundDelivery,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxAckPending: 50,
		AckWait:       60 * time.Second,
		MaxDeliver:    MaxDeliverActivityPubOutboundDelivery,
	})
	if err != nil {
		return fmt.Errorf("nats: create consumer %s: %w", ConsumerActivityPubOutboundDelivery, err)
	}

	_, err = js.CreateOrUpdateConsumer(ctx, StreamActivityPubOutboundFanout, jetstream.ConsumerConfig{
		Durable:       ConsumerActivityPubOutboundFanout,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxAckPending: 20,
		AckWait:       120 * time.Second,
		MaxDeliver:    MaxDeliverActivityPubOutboundFanout,
	})
	if err != nil {
		return fmt.Errorf("nats: create consumer %s: %w", ConsumerActivityPubOutboundFanout, err)
	}

	return nil
}
