package mastodon

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
)

// SuggestionsHandler handles Mastodon API v2 suggestions endpoints.
type SuggestionsHandler struct{}

// NewSuggestionsHandler returns a new SuggestionsHandler.
func NewSuggestionsHandler() *SuggestionsHandler {
	return &SuggestionsHandler{}
}

// GETSuggestions handles GET /api/v1/suggestions and GET /api/v2/suggestions.
// Returns an empty array until account suggestions are implemented.
func (h *SuggestionsHandler) GETSuggestions(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, http.StatusOK, []any{})
}
