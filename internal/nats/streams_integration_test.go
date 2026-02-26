//go:build integration

package nats

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera-fed/internal/ap"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/nats/federation"
)

// TestEnsureStreams_PublishConsume requires NATS with JetStream running (e.g. docker-compose up nats).
func TestEnsureStreams_PublishConsume(t *testing.T) {
	t.Helper()
	url := os.Getenv("NATS_URL")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{NATSUrl: url}

	client, err := New(cfg, logger)
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = EnsureStreams(ctx, client.JS)
	require.NoError(t, err)

	producer := federation.NewProducer(client.JS, nil)
	msg := ap.DeliveryMessage{
		ActivityID:  "https://test.example/activity/1",
		Activity:    json.RawMessage(`{"type":"Test"}`),
		TargetInbox: "https://remote.example/inbox",
		SenderID:    "01ABC",
	}
	err = producer.EnqueueDelivery(ctx, "test", msg)
	require.NoError(t, err)

	// Use the existing federation-worker consumer (WorkQueue streams allow only one consumer).
	cons, err := client.JS.Consumer(ctx, "FEDERATION", "federation-worker")
	require.NoError(t, err)

	msgs, err := cons.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
	require.NoError(t, err)
	var first jetstream.Msg
	for m := range msgs.Messages() {
		first = m
		break
	}
	require.NoError(t, msgs.Error())
	require.NotNil(t, first, "expected one message from stream")
	var decoded ap.DeliveryMessage
	err = json.Unmarshal(first.Data(), &decoded)
	require.NoError(t, err)
	assert.Equal(t, msg.ActivityID, decoded.ActivityID)
	assert.Equal(t, msg.TargetInbox, decoded.TargetInbox)
	require.NoError(t, first.Ack())
}
