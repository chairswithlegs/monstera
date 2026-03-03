package activitypub

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Subject prefixes for the activitypub outbound streams.
// DLQ subjects use a sibling *dlq. prefix to avoid overlap with parent stream wildcards.
const (
	subjectPrefixDeliver    = "activitypub.outbound.deliver."
	subjectPrefixDeliverDLQ = "activitypub.outbound.deliverdlq."
	subjectPrefixFanout     = "activitypub.outbound.fanout."
	subjectPrefixFanoutDLQ  = "activitypub.outbound.fanoutdlq."
)

// Stream and consumer names for activitypub outbound.
const (
	streamDelivery    = "ACTIVITYPUB_OUTBOUND_DELIVERY"
	streamDeliveryDLQ = "ACTIVITYPUB_OUTBOUND_DELIVERY_DLQ"
	streamFanout      = "ACTIVITYPUB_OUTBOUND_FANOUT"
	streamFanoutDLQ   = "ACTIVITYPUB_OUTBOUND_FANOUT_DLQ"

	consumerDelivery = "activitypub-outbound-delivery"
	consumerFanout   = "activitypub-outbound-fanout"
)

var (
	deliveryRetries = []time.Duration{30 * time.Second, 5 * time.Minute, time.Hour}
	fanoutRetries   = []time.Duration{5 * time.Minute}
)

// CreateOrUpdateStreams creates or updates the activitypub outbound streams and consumers.
func CreateOrUpdateStreams(ctx context.Context, js jetstream.JetStream) error {
	for _, d := range streams {
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

// streamConfig describes a JetStream stream and its optional durable consumer.
type streamConfig struct {
	Stream   jetstream.StreamConfig
	Consumer *jetstream.ConsumerConfig
}

var streams = []streamConfig{
	{
		Stream: jetstream.StreamConfig{
			Name:      streamDelivery,
			Subjects:  []string{subjectPrefixDeliver + ">"},
			Retention: jetstream.WorkQueuePolicy,
			Storage:   jetstream.FileStorage,
			MaxAge:    72 * time.Hour,
			MaxBytes:  4 * 1024 * 1024,
			Discard:   jetstream.DiscardOld,
		},
		Consumer: &jetstream.ConsumerConfig{
			Durable:       consumerDelivery,
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxAckPending: 50,
			AckWait:       60 * time.Second,
			MaxDeliver:    len(deliveryRetries),
			BackOff:       deliveryRetries,
		},
	},
	{
		Stream: jetstream.StreamConfig{
			Name:      streamDeliveryDLQ,
			Subjects:  []string{subjectPrefixDeliverDLQ + ">"},
			Retention: jetstream.LimitsPolicy,
			Storage:   jetstream.FileStorage,
			MaxAge:    30 * 24 * time.Hour,
		},
	},
	{
		Stream: jetstream.StreamConfig{
			Name:      streamFanout,
			Subjects:  []string{subjectPrefixFanout + ">"},
			Retention: jetstream.WorkQueuePolicy,
			Storage:   jetstream.FileStorage,
			MaxAge:    72 * time.Hour,
			MaxBytes:  4 * 1024 * 1024,
			Discard:   jetstream.DiscardOld,
		},
		Consumer: &jetstream.ConsumerConfig{
			Durable:       consumerFanout,
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxAckPending: 20,
			AckWait:       120 * time.Second,
			MaxDeliver:    len(fanoutRetries),
			BackOff:       fanoutRetries,
		},
	},
	{
		Stream: jetstream.StreamConfig{
			Name:      streamFanoutDLQ,
			Subjects:  []string{subjectPrefixFanoutDLQ + ">"},
			Retention: jetstream.LimitsPolicy,
			Storage:   jetstream.FileStorage,
			MaxAge:    30 * 24 * time.Hour,
		},
	},
}
