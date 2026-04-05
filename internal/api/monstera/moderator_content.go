package monstera

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/service"
)

// ModeratorContentHandler handles trending link filters.
type ModeratorContentHandler struct {
	trends service.TrendsService
}

// NewModeratorContentHandler returns a new ModeratorContentHandler.
func NewModeratorContentHandler(trends service.TrendsService) *ModeratorContentHandler {
	return &ModeratorContentHandler{trends: trends}
}

// GETTrendingLinkFilters returns the trending link filter list.
func (h *ModeratorContentHandler) GETTrendingLinkFilters(w http.ResponseWriter, r *http.Request) {
	urls, err := h.trends.ListTrendingLinkFilters(r.Context())
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if urls == nil {
		urls = []string{}
	}
	api.WriteJSON(w, http.StatusOK, map[string][]string{"urls": urls})
}

type addTrendingLinkFilterRequest struct {
	URL string `json:"url"`
}

func (r addTrendingLinkFilterRequest) Validate() error {
	return api.ValidateRequiredField(r.URL, "url")
}

// POSTTrendingLinkFilters adds a URL to the trending link filter list.
func (h *ModeratorContentHandler) POSTTrendingLinkFilters(w http.ResponseWriter, r *http.Request) {
	var body addTrendingLinkFilterRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	if err := h.trends.AddTrendingLinkFilter(r.Context(), body.URL); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETETrendingLinkFilter removes a URL from the trending link filter list.
// The URL to remove is passed as the ?url= query parameter.
func (h *ModeratorContentHandler) DELETETrendingLinkFilter(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if err := api.ValidateRequiredField(url, "url"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	if err := h.trends.RemoveTrendingLinkFilter(r.Context(), url); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
