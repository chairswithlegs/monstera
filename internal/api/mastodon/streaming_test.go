package mastodon

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamingHandler_GETHealth(t *testing.T) {
	t.Parallel()
	handler := NewStreamingHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/streaming/health", nil)
	rec := httptest.NewRecorder()
	handler.GETHealth(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Equal(t, "OK", rec.Body.String())
}
