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

// testMessage is a small payload used only to verify stream publish/consume round-trip.
type testMessage struct {
	ID      string `json:"id"`
	Payload string `json:"payload"`
}

// TestEnsureStreams_StreamAndConsumerConfig validates that the FEDERATION stream and
// federation-worker consumer have the expected config.
// This is used to ensure that EnsureStreams is working correctly.
func TestEnsureStreams_StreamAndConsumerConfig(t *testing.T) {
	client := setupNATSTest(t)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.JS.Stream(ctx, StreamFederation)
	require.NoError(t, err)
	streamInfo, err := stream.Info(ctx)
	require.NoError(t, err)

	assert.Equal(t, StreamFederation, streamInfo.Config.Name, "stream name")
	assert.Equal(t, jetstream.WorkQueuePolicy, streamInfo.Config.Retention,
		"stream retention must be WorkQueue so each message is delivered to only one consumer")
	assert.Equal(t, []string{subjectDeliver}, streamInfo.Config.Subjects, "stream subjects")

	cons, err := client.JS.Consumer(ctx, StreamFederation, ConsumerFederationWorker)
	require.NoError(t, err)
	consInfo, err := cons.Info(ctx)
	require.NoError(t, err)

	assert.Equal(t, ConsumerFederationWorker, consInfo.Config.Durable, "consumer durable name")
	assert.Equal(t, jetstream.AckExplicitPolicy, consInfo.Config.AckPolicy, "consumer ack policy")
	assert.Empty(t, consInfo.Config.DeliverSubject,
		"consumer must be pull (no DeliverSubject); DeliverSubject is for push only")
	assert.Equal(t, MaxDeliverFederation, consInfo.Config.MaxDeliver, "consumer max deliver")
}

// TestEnsureStreams_PublishConsume requires NATS with JetStream running (e.g. docker-compose up nats).
// It ensures streams and consumer exist, then publishes a message and consumes it.
func TestEnsureStreams_PublishConsume(t *testing.T) {
	client := setupNATSTest(t)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msg := testMessage{ID: "test-1", Payload: "hello"}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	_, err = client.JS.Publish(ctx, SubjectPrefixFederationDeliver+"test", data)
	require.NoError(t, err)

	// Use the existing federation-worker consumer (WorkQueue streams allow only one consumer).
	cons, err := client.JS.Consumer(ctx, StreamFederation, ConsumerFederationWorker)
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

	// Create two consumers
	runConsumer := func() jetstream.ConsumeContext {
		cons, err := client.JS.Consumer(ctx, StreamFederation, ConsumerFederationWorker)
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
	_, err = client.JS.Publish(ctx, SubjectPrefixFederationDeliver+"test", data)
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

	require.NoError(t, EnsureStreams(ctx, client.JS))

	stream, err := client.JS.Stream(ctx, StreamFederation)
	require.NoError(t, err)
	err = stream.Purge(ctx)
	require.NoError(t, err)

	return client
}
