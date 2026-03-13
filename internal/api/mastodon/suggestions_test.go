package mastodon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuggestionsHandler_GETSuggestions(t *testing.T) {
	t.Parallel()
	handler := NewSuggestionsHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v2/suggestions?limit=20", nil)
	rec := httptest.NewRecorder()
	handler.GETSuggestions(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body []any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Empty(t, body)
}
