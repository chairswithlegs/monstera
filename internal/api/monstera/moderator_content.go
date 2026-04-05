package monstera

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/service"
)

// ModeratorContentHandler handles trending link filters.
type ModeratorContentHandler struct {
	linkDeny service.TrendingLinkDenylistService
}

// NewModeratorContentHandler returns a new ModeratorContentHandler.
func NewModeratorContentHandler(linkDeny service.TrendingLinkDenylistService) *ModeratorContentHandler {
	return &ModeratorContentHandler{linkDeny: linkDeny}
}

// GETTrendingLinkFilters returns the trending link filter list.
func (h *ModeratorContentHandler) GETTrendingLinkFilters(w http.ResponseWriter, r *http.Request) {
	urls, err := h.linkDeny.GetDenylist(r.Context())
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
	if err := h.linkDeny.AddDenylist(r.Context(), body.URL); err != nil {
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
	if err := h.linkDeny.RemoveDenylist(r.Context(), url); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
