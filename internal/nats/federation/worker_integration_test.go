//go:build integration

package federation

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ap "github.com/chairswithlegs/monstera-fed/internal/activitypub"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/nats"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
)

// TestFederationWorker_Delivery requires NATS with JetStream running.
// It enqueues a delivery, starts the worker with a fake store (account with valid key)
// and a test HTTP server, and asserts the server receives a signed POST.
func TestFederationWorker_Delivery(t *testing.T) {
	t.Helper()
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		t.Fatal("NATS_URL must be set for integration test")
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))

	fake := testutil.NewFakeStore()
	ctx := context.Background()
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:         "01sender",
		Username:   "alice",
		APID:       "https://example.com/users/alice",
		PrivateKey: &privPEM,
	})
	require.NoError(t, err)

	var received sync.WaitGroup
	received.Add(1)
	var (
		req     *http.Request
		bodyBuf []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = r
		bodyBuf, _ = io.ReadAll(r.Body)
		received.Done()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{
		NATSUrl:                     natsURL,
		InstanceDomain:              "example.com",
		FederationWorkerConcurrency: 1,
	}
	client, err := nats.New(cfg, logger)
	require.NoError(t, err)
	defer client.Close()

	streamCtx, streamCancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = nats.EnsureStreams(streamCtx, client.JS)
	streamCancel()
	require.NoError(t, err)

	producer := NewProducer(client.JS, nil)
	blocklist := ap.NewBlocklistCache(fake, logger)
	_ = blocklist.Refresh(ctx)

	worker := NewFederationWorker(client.JS, producer, fake, blocklist, cfg, logger, nil)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	go func() {
		_ = worker.Start(workerCtx)
	}()
	defer workerCancel()

	delivery := ap.DeliveryMessage{
		ActivityID:  "https://example.com/activities/1",
		Activity:    json.RawMessage(`{"type":"Create","actor":"https://example.com/users/alice"}`),
		TargetInbox: srv.URL,
		SenderID:    "01sender",
	}
	err = producer.EnqueueDelivery(ctx, "create", delivery)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		received.Wait()
		close(done)
	}()
	select {
	case <-done:
		break
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for worker to POST")
	}

	require.NotNil(t, req)
	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "application/activity+json", req.Header.Get("Content-Type"))
	assert.NotEmpty(t, req.Header.Get("Signature"))
	assert.NotEmpty(t, req.Header.Get("Date"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(bodyBuf, &body))
	assert.Equal(t, "Create", body["type"])
}
