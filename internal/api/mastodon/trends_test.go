package mastodon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrendsHandler_GETTrendsStatuses(t *testing.T) {
	t.Parallel()
	handler := NewTrendsHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/statuses", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsStatuses(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body)
}

func TestTrendsHandler_GETTrendsTags(t *testing.T) {
	t.Parallel()
	handler := NewTrendsHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/tags?limit=20", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsTags(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body)
}

func TestTrendsHandler_GETTrendsLinks(t *testing.T) {
	t.Parallel()
	handler := NewTrendsHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trends/links", nil)
	rec := httptest.NewRecorder()
	handler.GETTrendsLinks(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body)
}
