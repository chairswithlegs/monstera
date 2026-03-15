package activitypub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ap "github.com/chairswithlegs/monstera/internal/activitypub"
	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/config"
)

func TestInboxHandler_POSTInbox_badContentType_returns400(t *testing.T) {
	t.Parallel()
	cacheStore, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	proc := &mockInboxProcessor{}
	cfg := &config.Config{InstanceDomain: "example.com"}
	verifier := &mockHTTPSignatureService{keyID: "https://example.com/users/alice#main-key"}
	h := NewInboxHandler(proc, cacheStore, cfg, verifier)
	r := httptest.NewRequest(http.MethodPost, "/inbox", strings.NewReader(`{"type":"Create"}`))
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.POSTInbox(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInboxHandler_POSTInbox_invalidJSON_returns400(t *testing.T) {
	t.Parallel()
	cacheStore, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	proc := &mockInboxProcessor{}
	cfg := &config.Config{InstanceDomain: "example.com"}
	verifier := &mockHTTPSignatureService{keyID: "https://example.com/users/alice#main-key"}
	h := NewInboxHandler(proc, cacheStore, cfg, verifier)
	r := httptest.NewRequest(http.MethodPost, "/inbox", strings.NewReader(`not json`))
	r.Header.Set("Content-Type", "application/activity+json")
	w := httptest.NewRecorder()
	h.POSTInbox(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInboxHandler_POSTInbox_happyPath_returns202(t *testing.T) {
	t.Parallel()
	cacheStore, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	proc := &mockInboxProcessor{}
	cfg := &config.Config{InstanceDomain: "example.com"}
	verifier := &mockHTTPSignatureService{keyID: "https://example.com/users/alice#main-key"}
	h := NewInboxHandler(proc, cacheStore, cfg, verifier)
	body := `{"@context":"https://www.w3.org/ns/activitystreams","type":"Follow","actor":"https://example.com/users/alice","object":"https://other.example/users/bob"}`
	r := httptest.NewRequest(http.MethodPost, "/inbox", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/activity+json")
	w := httptest.NewRecorder()
	h.POSTInbox(w, r)
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.True(t, proc.called, "Process should have been called")
}

type mockInboxProcessor struct {
	called bool
}

func (p *mockInboxProcessor) Process(ctx context.Context, activity *vocab.Activity) error {
	p.called = true
	return nil
}

type mockHTTPSignatureService struct {
	ap.HTTPSignatureService
	keyID string
	err   error
}

func (m *mockHTTPSignatureService) Verify(ctx context.Context, r *http.Request) (keyID string, err error) {
	if m.err != nil {
		return "", m.err
	}
	return m.keyID, nil
}
