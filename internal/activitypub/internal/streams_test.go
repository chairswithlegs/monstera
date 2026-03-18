package internal

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamConfigs_Count(t *testing.T) {
	t.Parallel()
	require.Len(t, StreamConfigs, 4, "expected delivery, delivery-DLQ, fanout, fanout-DLQ streams")
}

func TestStreamConfigs_NoDuplicateStreamNames(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool, len(StreamConfigs))
	for _, sc := range StreamConfigs {
		assert.False(t, seen[sc.Stream.Name], "duplicate stream name: %s", sc.Stream.Name)
		seen[sc.Stream.Name] = true
	}
}

func TestStreamConfigs_NoDuplicateConsumerNames(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for _, sc := range StreamConfigs {
		for _, c := range sc.Consumers {
			assert.False(t, seen[c.Durable], "duplicate consumer name: %s", c.Durable)
			seen[c.Durable] = true
		}
	}
}

func TestStreamConfigs_AllStreamsHaveValidProperties(t *testing.T) {
	t.Parallel()
	validRetention := map[jetstream.RetentionPolicy]bool{
		jetstream.LimitsPolicy:    true,
		jetstream.InterestPolicy:  true,
		jetstream.WorkQueuePolicy: true,
	}
	for _, sc := range StreamConfigs {
		assert.NotEmpty(t, sc.Stream.Name, "stream name must not be empty")
		require.NotEmpty(t, sc.Stream.Subjects, "stream %s must have at least one subject", sc.Stream.Name)
		for _, subj := range sc.Stream.Subjects {
			assert.NotEmpty(t, subj, "stream %s has an empty subject", sc.Stream.Name)
		}
		assert.True(t, validRetention[sc.Stream.Retention],
			"stream %s has unrecognised retention policy: %v", sc.Stream.Name, sc.Stream.Retention)
		assert.Equal(t, jetstream.FileStorage, sc.Stream.Storage, "stream %s should use file storage", sc.Stream.Name)
		assert.Greater(t, sc.Stream.MaxAge, time.Duration(0), "stream %s MaxAge must be positive", sc.Stream.Name)
	}
}

func TestStreamConfigs_AllConsumersHaveRequiredFields(t *testing.T) {
	t.Parallel()
	for _, sc := range StreamConfigs {
		for _, c := range sc.Consumers {
			assert.NotEmpty(t, c.Durable, "consumer on stream %s must have a durable name", sc.Stream.Name)
			assert.Equal(t, jetstream.AckExplicitPolicy, c.AckPolicy,
				"consumer %s on stream %s should use explicit ack", c.Durable, sc.Stream.Name)
			assert.Positive(t, c.MaxAckPending,
				"consumer %s MaxAckPending must be positive", c.Durable)
			assert.Greater(t, c.AckWait, time.Duration(0),
				"consumer %s AckWait must be positive", c.Durable)
		}
	}
}

func TestStreamConfigs_DeliveryStream(t *testing.T) {
	t.Parallel()
	sc := StreamConfigs[0]

	assert.Equal(t, StreamOutboxDelivery, sc.Stream.Name)
	assert.Equal(t, jetstream.WorkQueuePolicy, sc.Stream.Retention)
	assert.Equal(t, jetstream.DiscardOld, sc.Stream.Discard)
	assert.Positive(t, sc.Stream.MaxBytes, "MaxBytes must be positive")

	require.Len(t, sc.Consumers, 1)
	c := sc.Consumers[0]
	assert.Equal(t, consumerDelivery, c.Durable)
	assert.Equal(t, len(deliveryRetries), c.MaxDeliver)
	assert.Equal(t, deliveryRetries, c.BackOff)
}

func TestStreamConfigs_DeliveryDLQStream(t *testing.T) {
	t.Parallel()
	sc := StreamConfigs[1]

	assert.Equal(t, StreamOutboxDeliveryDLQ, sc.Stream.Name)
	assert.Equal(t, jetstream.LimitsPolicy, sc.Stream.Retention)
	assert.Empty(t, sc.Consumers, "DLQ stream should have no consumers")
}

func TestStreamConfigs_FanoutStream(t *testing.T) {
	t.Parallel()
	sc := StreamConfigs[2]

	assert.Equal(t, StreamOutboxFanout, sc.Stream.Name)
	assert.Equal(t, jetstream.WorkQueuePolicy, sc.Stream.Retention)
	assert.Equal(t, jetstream.DiscardOld, sc.Stream.Discard)
	assert.Positive(t, sc.Stream.MaxBytes, "MaxBytes must be positive")

	require.Len(t, sc.Consumers, 1)
	c := sc.Consumers[0]
	assert.Equal(t, consumerFanout, c.Durable)
	assert.Equal(t, len(fanoutRetries), c.MaxDeliver)
	assert.Equal(t, fanoutRetries, c.BackOff)
}

func TestStreamConfigs_FanoutDLQStream(t *testing.T) {
	t.Parallel()
	sc := StreamConfigs[3]

	assert.Equal(t, StreamOutboxFanoutDLQ, sc.Stream.Name)
	assert.Equal(t, jetstream.LimitsPolicy, sc.Stream.Retention)
	assert.Empty(t, sc.Consumers, "DLQ stream should have no consumers")
}

func TestSubjectPrefixes_NonEmpty(t *testing.T) {
	t.Parallel()
	prefixes := []string{
		subjectPrefixDeliver,
		subjectPrefixDeliverDLQ,
		subjectPrefixFanout,
		subjectPrefixFanoutDLQ,
	}
	seen := make(map[string]bool, len(prefixes))
	for _, p := range prefixes {
		assert.NotEmpty(t, p)
		assert.False(t, seen[p], "duplicate subject prefix: %s", p)
		seen[p] = true
	}
}

func TestStreamConstants_Unique(t *testing.T) {
	t.Parallel()
	names := []string{
		StreamOutboxDelivery,
		StreamOutboxDeliveryDLQ,
		StreamOutboxFanout,
		StreamOutboxFanoutDLQ,
	}
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		assert.NotEmpty(t, n)
		assert.False(t, seen[n], "duplicate stream constant: %s", n)
		seen[n] = true
	}
}

func TestRetryDurations_Positive(t *testing.T) {
	t.Parallel()
	require.NotEmpty(t, deliveryRetries)
	for i, d := range deliveryRetries {
		assert.Greater(t, d, time.Duration(0), "deliveryRetries[%d] must be positive", i)
	}
	require.NotEmpty(t, fanoutRetries)
	for i, d := range fanoutRetries {
		assert.Greater(t, d, time.Duration(0), "fanoutRetries[%d] must be positive", i)
	}
}
