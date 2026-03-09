package mastodon

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
)

// TrendsHandler handles Mastodon trends API endpoints.
type TrendsHandler struct{}

// NewTrendsHandler returns a new TrendsHandler.
func NewTrendsHandler() *TrendsHandler {
	return &TrendsHandler{}
}

// GETTrendsStatuses handles GET /api/v1/trends/statuses.
// Returns an empty array until trending statuses are implemented.
func (h *TrendsHandler) GETTrendsStatuses(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, http.StatusOK, []any{})
}

// GETTrendsTags handles GET /api/v1/trends/tags.
// Returns an empty array until trending tags are implemented.
func (h *TrendsHandler) GETTrendsTags(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, http.StatusOK, []any{})
}

// GETTrendsLinks handles GET /api/v1/trends/links.
// Returns an empty array until trending links are implemented.
func (h *TrendsHandler) GETTrendsLinks(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, http.StatusOK, []any{})
}
