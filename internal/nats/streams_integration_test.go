//go:build integration

package nats

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera-fed/internal/config"
)

// Test-only stream/consumer and subject prefix.
const (
	streamTest         = "NATS_INTEGRATION_TEST"
	consumerTest       = "nats-integration-test-worker"
	subjectPrefixTest  = "nats.integration.test.deliver."
	subjectTestDeliver = subjectPrefixTest + ">"
)

// testMessage is a small payload used only to verify stream publish/consume round-trip.
type testMessage struct {
	ID      string `json:"id"`
	Payload string `json:"payload"`
}

// TestEnsureStreams_StreamAndConsumerConfig validates the EnsureStreams function
// correctly configures NATS
func TestEnsureStreams_StreamAndConsumerConfig(t *testing.T) {
	url := os.Getenv("NATS_URL")
	require.NotEmpty(t, url, "NATS_URL must be set for integration test")
	ctx := context.Background()
	cfg := &config.Config{NATSUrl: url}
	client, err := New(cfg)
	require.NoError(t, err)
	require.NoError(t, EnsureStreams(ctx, client.JS))

	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.JS.Stream(ctx, StreamActivityPub)
	require.NoError(t, err)
	streamInfo, err := stream.Info(ctx)
	require.NoError(t, err)

	assert.Equal(t, StreamActivityPub, streamInfo.Config.Name, "stream name")
	assert.Equal(t, jetstream.WorkQueuePolicy, streamInfo.Config.Retention,
		"stream retention must be WorkQueue so each message is delivered to only one consumer")
	assert.Equal(t, []string{SubjectPrefixActivityPubDeliver + ">"}, streamInfo.Config.Subjects, "stream subjects")

	cons, err := client.JS.Consumer(ctx, StreamActivityPub, ConsumerActivityPubWorker)
	require.NoError(t, err)
	consInfo, err := cons.Info(ctx)
	require.NoError(t, err)

	assert.Equal(t, ConsumerActivityPubWorker, consInfo.Config.Durable, "consumer durable name")
	assert.Equal(t, jetstream.AckExplicitPolicy, consInfo.Config.AckPolicy, "consumer ack policy")
	assert.Empty(t, consInfo.Config.DeliverSubject,
		"consumer must be pull (no DeliverSubject); DeliverSubject is for push only")
	assert.Equal(t, MaxDeliverActivityPub, consInfo.Config.MaxDeliver, "consumer max deliver")
}

// TestEnsureStreams_PublishConsume requires NATS with JetStream running (e.g. docker-compose up nats).
// It ensures the test stream and consumer exist, then publishes a message and consumes it.
func TestEnsureStreams_PublishConsume(t *testing.T) {
	client := setupNATSTest(t)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msg := testMessage{ID: "test-1", Payload: "hello"}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	_, err = client.JS.Publish(ctx, subjectPrefixTest+"test", data)
	require.NoError(t, err)

	cons, err := client.JS.Consumer(ctx, streamTest, consumerTest)
	require.NoError(t, err)

	msgCh := make(chan jetstream.Msg, 1)
	consCtx, err := cons.Consume(
		func(m jetstream.Msg) {
			select {
			case msgCh <- m:
			default:
			}
		},
		jetstream.PullMaxMessages(1),
		jetstream.PullExpiry(2*time.Second),
	)
	require.NoError(t, err)
	defer consCtx.Stop()

	first := <-msgCh
	require.NotNil(t, first, "expected one message from stream")
	var decoded testMessage
	err = json.Unmarshal(first.Data(), &decoded)
	require.NoError(t, err)
	assert.Equal(t, msg.ID, decoded.ID)
	assert.Equal(t, msg.Payload, decoded.Payload)
	require.NoError(t, first.Ack())
}

// TestEnsureStreams_SingleMessageNotPulledTwice verifies that with WorkQueue retention,
// a single message is delivered to only one consumer when multiple processes pull.
func TestEnsureStreams_SingleMessageNotPulledTwice(t *testing.T) {
	client := setupNATSTest(t)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	msg := testMessage{ID: "unique", Payload: "test"}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	// Channel to receive messages
	msgCh := make(chan jetstream.Msg, 2)

	// Create two consumers on the test stream
	runConsumer := func() jetstream.ConsumeContext {
		cons, err := client.JS.Consumer(ctx, streamTest, consumerTest)
		require.NoError(t, err)

		consCtx, err := cons.Consume(
			func(m jetstream.Msg) {
				msgCh <- m
			},
		)
		require.NoError(t, err)
		return consCtx
	}

	consCtx1 := runConsumer()
	defer consCtx1.Stop()

	consCtx2 := runConsumer()
	defer consCtx2.Stop()

	// Publish the message
	_, err = client.JS.Publish(ctx, subjectPrefixTest+"test", data)
	require.NoError(t, err)

	// Wait for the message
	select {
	case m := <-msgCh:
		require.NotNil(t, m)
		_ = m.Ack()
	case <-time.After(6 * time.Second):
		t.Fatal("timeout waiting for message")
	}

	// Assert no duplicate message is received
	select {
	case m := <-msgCh:
		_ = m.Ack()
		t.Fatal("expected exactly one message; got duplicate delivery")
	case <-time.After(1 * time.Second):
		// No second message — pass
	}
}

func setupNATSTest(t *testing.T) *Client {
	t.Helper()
	url := os.Getenv("NATS_URL")
	require.NotEmpty(t, url, "NATS_URL must be set for integration test")
	ctx := context.Background()
	cfg := &config.Config{NATSUrl: url}
	client, err := New(cfg)
	require.NoError(t, err)

	require.NoError(t, ensureTestStreams(ctx, client.JS))

	stream, err := client.JS.Stream(ctx, streamTest)
	require.NoError(t, err)
	err = stream.Purge(ctx)
	require.NoError(t, err)

	return client
}

// ensureTestStreams creates the package test stream and consumer (same shape as FEDERATION).
// Does not touch the production FEDERATION stream.
func ensureTestStreams(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      streamTest,
		Subjects:  []string{subjectTestDeliver},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    72 * time.Hour,
		MaxBytes:  4 * 1024 * 1024,
		Discard:   jetstream.DiscardOld,
	})
	if err != nil {
		return err
	}
	_, err = js.CreateOrUpdateConsumer(ctx, streamTest, jetstream.ConsumerConfig{
		Durable:       consumerTest,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxAckPending: 50,
		AckWait:       60 * time.Second,
		MaxDeliver:    3,
		BackOff:       []time.Duration{30 * time.Second, 5 * time.Minute, 30 * time.Minute},
	})
	return err
}
