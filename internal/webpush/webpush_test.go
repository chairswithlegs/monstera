package webpush

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	wp "github.com/SherClockHolmes/webpush-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// testSubscription returns a domain.PushSubscription with valid keys from webpush-go's test suite.
func testSubscription(t *testing.T, endpoint string) *domain.PushSubscription {
	t.Helper()
	return &domain.PushSubscription{
		Endpoint:  endpoint,
		KeyP256DH: "BNNL5ZaTfK81qhXOx23-wewhigUeFb632jN6LvRWCFH1ubQr77FE_9qV1FuojuRmHP42zmf34rXgW80OvUVDgTk",
		KeyAuth:   "zqbxT6JKstKSY9JKibZLSQ",
	}
}

// newTestSender creates a Sender backed by http.DefaultClient for use in tests.
// Tests use httptest servers on localhost which the SSRF-hardened client blocks by design.
func newTestSender(t *testing.T, pubKey, privKey string) Sender {
	t.Helper()
	return newSenderWithClient(pubKey, privKey, "mailto:test@example.com", http.DefaultClient)
}

func TestSend_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	privKey, pubKey, err := wp.GenerateVAPIDKeys()
	require.NoError(t, err)

	s := newTestSender(t, pubKey, privKey)
	sub := testSubscription(t, server.URL)

	err = s.Send(context.Background(), sub, []byte("test payload"))
	assert.NoError(t, err)
}

func TestSend_Gone(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	t.Cleanup(server.Close)

	privKey, pubKey, err := wp.GenerateVAPIDKeys()
	require.NoError(t, err)

	s := newTestSender(t, pubKey, privKey)
	sub := testSubscription(t, server.URL)

	err = s.Send(context.Background(), sub, []byte("test payload"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSubscriptionGone, "expected ErrSubscriptionGone, got %v", err)
}

func TestSend_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	privKey, pubKey, err := wp.GenerateVAPIDKeys()
	require.NoError(t, err)

	s := newTestSender(t, pubKey, privKey)
	sub := testSubscription(t, server.URL)

	err = s.Send(context.Background(), sub, []byte("test payload"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
