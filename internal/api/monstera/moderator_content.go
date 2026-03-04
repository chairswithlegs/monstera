package monstera

import (
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/go-chi/chi/v5"
)

// ModeratorContentHandler handles server filters.
type ModeratorContentHandler struct {
	filters service.ServerFilterService
}

// NewModeratorContentHandler returns a new ModeratorContentHandler.
func NewModeratorContentHandler(filters service.ServerFilterService) *ModeratorContentHandler {
	return &ModeratorContentHandler{filters: filters}
}

// GETFilters returns server filters.
func (h *ModeratorContentHandler) GETFilters(w http.ResponseWriter, r *http.Request) {
	filters, err := h.filters.ListServerFilters(r.Context())
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.AdminServerFilter, 0, len(filters))
	for i := range filters {
		out = append(out, apimodel.ToAdminServerFilter(&filters[i]))
	}
	api.WriteJSON(w, http.StatusOK, apimodel.AdminServerFilterList{Filters: out})
}

// POSTFilters creates a server filter.
func (h *ModeratorContentHandler) POSTFilters(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Phrase    string `json:"phrase"`
		Scope     string `json:"scope"`
		Action    string `json:"action"`
		WholeWord bool   `json:"whole_word"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	if body.Phrase == "" {
		api.HandleError(w, r, api.NewBadRequestError("phrase required"))
		return
	}
	if body.Scope == "" {
		body.Scope = domain.ServerFilterScopeAll
	}
	if body.Action == "" {
		body.Action = domain.ServerFilterActionHide
	}
	filter, err := h.filters.CreateServerFilter(r.Context(), body.Phrase, body.Scope, body.Action, body.WholeWord)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, apimodel.ToAdminServerFilter(filter))
}

// PUTFilter updates a server filter.
func (h *ModeratorContentHandler) PUTFilter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	var body struct {
		Phrase    string `json:"phrase"`
		Scope     string `json:"scope"`
		Action    string `json:"action"`
		WholeWord bool   `json:"whole_word"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	filter, err := h.filters.UpdateServerFilter(r.Context(), id, body.Phrase, body.Scope, body.Action, body.WholeWord)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToAdminServerFilter(filter))
}

// DELETEFilter deletes a server filter.
func (h *ModeratorContentHandler) DELETEFilter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	if err := h.filters.DeleteServerFilter(r.Context(), id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
