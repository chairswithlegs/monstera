package activitypub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/testutil"
)

func TestInboxHandler_POSTInbox_noProcessor_returns202(t *testing.T) {
	t.Parallel()
	deps := testDeps(testutil.NewFakeStore(), &config.Config{InstanceDomain: "example.com"})
	deps.Cache = nil
	deps.Inbox = nil
	h := NewInboxHandler(deps)
	r := httptest.NewRequest(http.MethodPost, "/inbox", strings.NewReader(`{"type":"Create"}`))
	r.Header.Set("Content-Type", "application/activity+json")
	w := httptest.NewRecorder()
	h.POSTInbox(w, r)
	assert.Equal(t, http.StatusAccepted, w.Code)
}
