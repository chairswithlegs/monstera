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
	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/config"
)

func TestInboxHandler_POSTInbox_badContentType_returns400(t *testing.T) {
	t.Parallel()
	cacheStore, err := cache.New(cache.Config{Driver: "memory"})
	require.NoError(t, err)
	proc := &mockInboxProcessor{}
	cfg := &config.Config{InstanceDomain: "example.com"}
	h := NewInboxHandler(proc, cacheStore, cfg, nil)
	r := httptest.NewRequest(http.MethodPost, "/inbox", strings.NewReader(`{"type":"Create"}`))
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.POSTInbox(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

type mockInboxProcessor struct{}

func (p *mockInboxProcessor) Process(ctx context.Context, activity *ap.Activity) error {
	return nil
}
