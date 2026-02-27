package activitypub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInboxHandler_POSTInbox_noProcessor_returns202(t *testing.T) {
	t.Parallel()
	h := NewInboxHandler(nil, nil)
	r := httptest.NewRequest(http.MethodPost, "/inbox", strings.NewReader(`{"type":"Create"}`))
	r.Header.Set("Content-Type", "application/activity+json")
	w := httptest.NewRecorder()
	h.POSTInbox(w, r)
	assert.Equal(t, http.StatusAccepted, w.Code)
}
