package activitypub

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ap "github.com/chairswithlegs/monstera-fed/internal/activitypub"
	"github.com/chairswithlegs/monstera-fed/internal/cache"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
)

func TestInboxHandler_POSTInbox_badContentType_returns400(t *testing.T) {
	t.Parallel()
	fake := testutil.NewFakeStore()
	cacheStore, err := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	require.NoError(t, err)
	bl := ap.NewBlocklistCache(fake, slog.Default())
	_ = bl.Refresh(context.Background())
	proc := ap.NewInboxProcessor(fake, cacheStore, bl, nil, nil, &config.Config{InstanceDomain: "example.com"}, nil)
	cfg := &config.Config{InstanceDomain: "example.com"}
	h := NewInboxHandler(proc, cacheStore, cfg, nil)
	r := httptest.NewRequest(http.MethodPost, "/inbox", strings.NewReader(`{"type":"Create"}`))
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.POSTInbox(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
