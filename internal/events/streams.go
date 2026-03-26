package events

import (
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/natsutil"
)

// Subject prefix for domain events published by the outbox poller.
const SubjectPrefix = "domain.events."

// Stream and consumer names.
const (
	StreamDomainEvents    = "DOMAIN_EVENTS"
	ConsumerFederation    = "domain-events-federation"
	ConsumerSSE           = "domain-events-sse"
	ConsumerNotifications = "domain-events-notifications"
	ConsumerPushDelivery  = "domain-events-push-delivery"
	ConsumerCards         = "domain-events-cards"
)

// StreamConfigs returns the DOMAIN_EVENTS stream and consumer configurations.
var StreamConfigs = []natsutil.StreamConfig{
	{
		Stream: jetstream.StreamConfig{
			Name:       StreamDomainEvents,
			Subjects:   []string{SubjectPrefix + ">"},
			Retention:  jetstream.InterestPolicy,
			Storage:    jetstream.FileStorage,
			MaxAge:     72 * time.Hour,
			Discard:    jetstream.DiscardOld,
			Duplicates: 5 * time.Minute,
		},
		Consumers: []jetstream.ConsumerConfig{
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
			{
				Durable:       ConsumerNotifications,
				AckPolicy:     jetstream.AckExplicitPolicy,
				MaxAckPending: 50,
				AckWait:       30 * time.Second,
				FilterSubject: SubjectPrefix + ">",
			},
			{
				Durable:       ConsumerPushDelivery,
				AckPolicy:     jetstream.AckExplicitPolicy,
				MaxAckPending: 50,
				AckWait:       30 * time.Second,
				FilterSubject: SubjectPrefix + "notification.>",
			},
			{
				Durable:       ConsumerCards,
				AckPolicy:     jetstream.AckExplicitPolicy,
				MaxAckPending: 50,
				AckWait:       30 * time.Second,
				FilterSubject: SubjectPrefix + "status.created",
			},
		},
	},
}
