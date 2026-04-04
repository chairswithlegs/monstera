package monstera

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/service"
)

// AdminTrendsHandler handles admin endpoints for trending link moderation.
type AdminTrendsHandler struct {
	svc service.TrendingLinkDenylistService
}

// NewAdminTrendsHandler returns a new AdminTrendsHandler.
func NewAdminTrendsHandler(svc service.TrendingLinkDenylistService) *AdminTrendsHandler {
	return &AdminTrendsHandler{svc: svc}
}

// GETDenylist returns the trending link denylist.
func (h *AdminTrendsHandler) GETDenylist(w http.ResponseWriter, r *http.Request) {
	urls, err := h.svc.GetDenylist(r.Context())
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if urls == nil {
		urls = []string{}
	}
	api.WriteJSON(w, http.StatusOK, map[string][]string{"urls": urls})
}

type addTrendingURLRequest struct {
	URL string `json:"url"`
}

func (r addTrendingURLRequest) Validate() error {
	return api.ValidateRequiredField(r.URL, "url")
}

// POSTDenylist adds a URL to the trending link denylist.
func (h *AdminTrendsHandler) POSTDenylist(w http.ResponseWriter, r *http.Request) {
	var body addTrendingURLRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	if err := h.svc.AddDenylist(r.Context(), body.URL); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETEDenylist removes a URL from the trending link denylist.
// The URL to remove is passed as the ?url= query parameter.
func (h *AdminTrendsHandler) DELETEDenylist(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if err := api.ValidateRequiredField(url, "url"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	if err := h.svc.RemoveDenylist(r.Context(), url); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
