package events

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubjectPrefix(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "domain.events.", SubjectPrefix)
	assert.NotEmpty(t, SubjectPrefix)
}

func TestStreamAndConsumerConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "DOMAIN_EVENTS", StreamDomainEvents)
	assert.Equal(t, "domain-events-federation", ConsumerFederation)
	assert.Equal(t, "domain-events-sse", ConsumerSSE)
	assert.Equal(t, "domain-events-notifications", ConsumerNotifications)
	assert.Equal(t, "domain-events-push-delivery", ConsumerPushDelivery)
	assert.Equal(t, "domain-events-cards", ConsumerCards)

	consumers := []string{ConsumerFederation, ConsumerSSE, ConsumerNotifications, ConsumerPushDelivery, ConsumerCards}
	seen := make(map[string]bool, len(consumers))
	for _, c := range consumers {
		assert.False(t, seen[c], "duplicate consumer name: %s", c)
		seen[c] = true
	}
}

func TestStreamConfigs_SingleStreamDefined(t *testing.T) {
	t.Parallel()
	require.Len(t, StreamConfigs, 1)
}

func TestStreamConfigs_StreamProperties(t *testing.T) {
	t.Parallel()
	sc := StreamConfigs[0].Stream

	assert.Equal(t, StreamDomainEvents, sc.Name)
	require.Len(t, sc.Subjects, 1)
	assert.Equal(t, SubjectPrefix+">", sc.Subjects[0], "stream subject should be a wildcard under the prefix")
	assert.Equal(t, jetstream.InterestPolicy, sc.Retention)
	assert.Equal(t, jetstream.FileStorage, sc.Storage)
	assert.Equal(t, jetstream.DiscardOld, sc.Discard)
	assert.Greater(t, sc.MaxAge, time.Duration(0), "MaxAge must be positive")
	assert.Greater(t, sc.Duplicates, time.Duration(0), "Duplicates window must be positive")
}

func TestStreamConfigs_ConsumerCount(t *testing.T) {
	t.Parallel()
	consumers := StreamConfigs[0].Consumers
	require.Len(t, consumers, 5, "expected federation, SSE, notifications, push-delivery, and cards consumers")
}

func TestStreamConfigs_ConsumerProperties(t *testing.T) {
	t.Parallel()
	consumers := StreamConfigs[0].Consumers

	expectedNames := map[string]bool{
		ConsumerFederation:    false,
		ConsumerSSE:           false,
		ConsumerNotifications: false,
		ConsumerPushDelivery:  false,
		ConsumerCards:         false,
	}

	for _, c := range consumers {
		_, ok := expectedNames[c.Durable]
		assert.True(t, ok, "unexpected consumer: %s", c.Durable)
		expectedNames[c.Durable] = true

		assert.Equal(t, jetstream.AckExplicitPolicy, c.AckPolicy, "consumer %s should use explicit ack", c.Durable)
		assert.Positive(t, c.MaxAckPending, "consumer %s MaxAckPending must be positive", c.Durable)
		assert.Greater(t, c.AckWait, time.Duration(0), "consumer %s AckWait must be positive", c.Durable)
		assert.Contains(t, c.FilterSubject, SubjectPrefix, "consumer %s should filter under domain.events prefix", c.Durable)
	}

	for name, found := range expectedNames {
		assert.True(t, found, "consumer %s not found in StreamConfigs", name)
	}
}

func TestStreamConfigs_ConsumerNamesAreDurable(t *testing.T) {
	t.Parallel()
	for _, c := range StreamConfigs[0].Consumers {
		assert.NotEmpty(t, c.Durable, "all consumers must have a durable name")
	}
}
