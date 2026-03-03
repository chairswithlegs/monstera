//go:build integration

package activitypub

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/cache"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	natsutil "github.com/chairswithlegs/monstera-fed/internal/nats"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
)

// receivedRequest captures a request for assertions.
type receivedRequest struct {
	Method string
	URL    string
	Header http.Header
	Body   []byte
}

// TestDeliveryWorker_PullDeliverAndNoDuplicate validates that the worker:
// - pulls messages from the ACTIVITYPUB stream,
// - makes an outgoing HTTP POST to the target inbox (via httptest server),
// - delivers each message exactly once (no duplicate POSTs).
func TestDeliveryWorker_PullDeliverAndNoDuplicate(t *testing.T) {
	url := os.Getenv("NATS_URL")
	require.NotEmpty(t, url, "NATS_URL must be set for integration test")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cfg := &config.Config{
		NATSUrl:                     url,
		InstanceDomain:              "test.example",
		FederationWorkerConcurrency: 1,
	}
	client, err := natsutil.New(cfg)
	require.NoError(t, err)
	defer client.Close()

	require.NoError(t, CreateOrUpdateStreams(ctx, client.JS))
	stream, err := client.JS.Stream(ctx, streamDelivery)
	require.NoError(t, err)
	require.NoError(t, stream.Purge(ctx), "purge stream so test starts with no messages")

	var received []receivedRequest
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, receivedRequest{
			Method: r.Method,
			URL:    r.URL.String(),
			Header: r.Header.Clone(),
			Body:   body,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	privPEM, err := generateTestKeyPair()
	require.NoError(t, err)
	senderID := uid.New()
	apID := fmt.Sprintf("https://%s/users/alice", cfg.InstanceDomain)
	inboxURL := srv.URL + "/inbox"

	fake := testutil.NewFakeStore()
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:           senderID,
		Username:     "alice",
		Domain:       nil,
		DisplayName:  nil,
		Note:         nil,
		PublicKey:    "test-pubkey",
		PrivateKey:   &privPEM,
		InboxURL:     inboxURL,
		OutboxURL:    "",
		FollowersURL: "",
		FollowingURL: "",
		APID:         apID,
		ApRaw:        nil,
		Bot:          false,
		Locked:       false,
	})
	require.NoError(t, err)

	activityBody := json.RawMessage(`{"type":"Create","object":{"type":"Note","content":"hello"}}`)
	delivery := outboxDeliveryMessage{
		ActivityID:  "https://test.example/activity/1",
		Activity:    activityBody,
		TargetInbox: inboxURL,
		SenderID:    senderID,
	}

	cacheStore, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = cacheStore.Close() }()
	accountSvc := service.NewAccountService(fake, "https://"+cfg.InstanceDomain)
	signer := NewHTTPSignatureService(cfg, cacheStore, accountSvc)
	worker := newOutboxDeliveryWorker(client.JS, nil, signer, cfg)
	require.NoError(t, worker.publish(ctx, "create", delivery))
	workerCtx, workerCancel := context.WithCancel(context.Background())
	go func() {
		_ = worker.start(workerCtx)
	}()
	defer workerCancel()

	require.Eventually(t, func() bool {
		mu.Lock()
		n := len(received)
		mu.Unlock()
		return n >= 1
	}, 5*time.Second, 100*time.Millisecond, "worker should deliver one request to inbox")

	mu.Lock()
	require.Len(t, received, 1, "expected exactly one HTTP request (no duplicate delivery)")
	req := received[0]
	mu.Unlock()

	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "application/activity+json", req.Header.Get("Content-Type"))
	assert.NotEmpty(t, req.Header.Get("Signature"), "request must be signed with HTTP Signature")
	assert.True(t, bytes.Equal(activityBody, req.Body), "request body must match enqueued activity")
}

// TestOutboxFanoutWorker_Integration validates the full path: publish outboxFanoutMessage to ACTIVITYPUB_FANOUT,
// fanout worker consumes it, fetches follower inboxes from store, publishes deliveryMessages to
// ACTIVITYPUB stream, delivery worker consumes and POSTs to the target inbox.
func TestOutboxFanoutWorker_Integration(t *testing.T) {
	url := os.Getenv("NATS_URL")
	require.NotEmpty(t, url, "NATS_URL must be set for integration test")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cfg := &config.Config{
		NATSUrl:                     url,
		InstanceDomain:              "test.example",
		FederationWorkerConcurrency: 1,
	}
	client, err := natsutil.New(cfg)
	require.NoError(t, err)
	defer client.Close()

	require.NoError(t, CreateOrUpdateStreams(ctx, client.JS))
	for _, streamName := range []string{streamDelivery, streamFanout} {
		stream, err := client.JS.Stream(ctx, streamName)
		require.NoError(t, err)
		require.NoError(t, stream.Purge(ctx), "purge stream %s", streamName)
	}

	var received []receivedRequest
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, receivedRequest{
			Method: r.Method,
			URL:    r.URL.String(),
			Header: r.Header.Clone(),
			Body:   body,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	privPEM, err := generateTestKeyPair()
	require.NoError(t, err)
	senderID := uid.New()
	apID := fmt.Sprintf("https://%s/users/alice", cfg.InstanceDomain)
	inboxURL := srv.URL + "/inbox"

	fake := testutil.NewFakeStore()
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:         senderID,
		Username:   "alice",
		Domain:     nil,
		PublicKey:  "test-pubkey",
		PrivateKey: &privPEM,
		InboxURL:   inboxURL,
		APID:       apID,
		Bot:        false,
		Locked:     false,
	})
	require.NoError(t, err)
	remoteDomain := "remote.example"
	followerID := uid.New()
	_, err = fake.CreateAccount(ctx, store.CreateAccountInput{
		ID:       followerID,
		Username: "bob",
		Domain:   &remoteDomain,
		InboxURL: inboxURL,
		APID:     "https://remote.example/users/bob",
	})
	require.NoError(t, err)
	_, err = fake.CreateFollow(ctx, store.CreateFollowInput{
		ID:        uid.New(),
		AccountID: followerID,
		TargetID:  senderID,
		State:     "accepted",
	})
	require.NoError(t, err)

	cacheStore, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	defer func() { _ = cacheStore.Close() }()
	accountSvc := service.NewAccountService(fake, "https://"+cfg.InstanceDomain)
	signer := NewHTTPSignatureService(cfg, cacheStore, accountSvc)
	outboxDeliveryWorker := newOutboxDeliveryWorker(client.JS, nil, signer, cfg)
	fanoutWorker := newOutboxFanoutWorker(client.JS, fake, outboxDeliveryWorker, cfg)

	fanoutMsg := outboxFanoutMessage{
		ActivityID: "https://test.example/activities/01act",
		Activity:   json.RawMessage(`{"type":"Create","object":{"type":"Note","content":"hello"}}`),
		SenderID:   senderID,
	}
	require.NoError(t, fanoutWorker.publish(ctx, "create", fanoutMsg))

	workerCtx, workerCancel := context.WithCancel(context.Background())
	go func() { _ = outboxDeliveryWorker.start(workerCtx) }()
	go func() { _ = fanoutWorker.start(workerCtx) }()
	defer workerCancel()

	require.Eventually(t, func() bool {
		mu.Lock()
		n := len(received)
		mu.Unlock()
		return n >= 1
	}, 5*time.Second, 100*time.Millisecond, "fanout worker should enqueue delivery and worker should POST to inbox")

	mu.Lock()
	require.Len(t, received, 1)
	req := received[0]
	mu.Unlock()

	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "application/activity+json", req.Header.Get("Content-Type"))
	assert.NotEmpty(t, req.Header.Get("Signature"))
	assert.Equal(t, `{"type":"Create","object":{"type":"Note","content":"hello"}}`, string(req.Body))
}

func generateTestKeyPair() (privPEM string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return string(pem.EncodeToMemory(privBlock)), nil
}
