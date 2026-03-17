package activitypub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	ap "github.com/chairswithlegs/monstera/internal/activitypub"
	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
)

func TestInboxHandler_POSTInbox_badContentType_returns400(t *testing.T) {
	t.Parallel()
	proc := &mockInboxProcessor{}
	verifier := &mockHTTPSignatureService{keyID: "https://example.com/users/alice#main-key"}
	h := NewInboxHandler(proc, verifier, "example.com")
	r := httptest.NewRequest(http.MethodPost, "/inbox", strings.NewReader(`{"type":"Create"}`))
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.POSTInbox(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInboxHandler_POSTInbox_invalidJSON_returns400(t *testing.T) {
	t.Parallel()
	proc := &mockInboxProcessor{}
	verifier := &mockHTTPSignatureService{keyID: "https://example.com/users/alice#main-key"}
	h := NewInboxHandler(proc, verifier, "example.com")
	r := httptest.NewRequest(http.MethodPost, "/inbox", strings.NewReader(`not json`))
	r.Header.Set("Content-Type", "application/activity+json")
	w := httptest.NewRecorder()
	h.POSTInbox(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInboxHandler_POSTInbox_happyPath_returns202(t *testing.T) {
	t.Parallel()
	proc := &mockInboxProcessor{}
	verifier := &mockHTTPSignatureService{keyID: "https://remote.example/users/alice#main-key"}
	h := NewInboxHandler(proc, verifier, "example.com")
	body := `{"@context":"https://www.w3.org/ns/activitystreams","type":"Follow","actor":"https://remote.example/users/alice","object":"https://example.com/users/bob"}`
	r := httptest.NewRequest(http.MethodPost, "/inbox", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/activity+json")
	w := httptest.NewRecorder()
	h.POSTInbox(w, r)
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.True(t, proc.called, "Process should have been called")
}

func TestInboxHandler_POSTInbox_ownDomain_returns400(t *testing.T) {
	t.Parallel()
	proc := &mockInboxProcessor{}
	verifier := &mockHTTPSignatureService{keyID: "https://example.com/users/alice#main-key"}
	h := NewInboxHandler(proc, verifier, "example.com")
	body := `{"@context":"https://www.w3.org/ns/activitystreams","type":"Follow","actor":"https://example.com/users/alice","object":"https://other.example/users/bob"}`
	r := httptest.NewRequest(http.MethodPost, "/inbox", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/activity+json")
	w := httptest.NewRecorder()
	h.POSTInbox(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, proc.called, "Process should not have been called for own-domain activity")
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
