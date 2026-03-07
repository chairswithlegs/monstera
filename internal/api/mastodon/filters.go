package mastodon

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// FiltersHandler handles GET/POST/PUT/DELETE /api/v1/filters.
type FiltersHandler struct {
	filters service.UserFilterService
}

// NewFiltersHandler returns a new FiltersHandler.
func NewFiltersHandler(filters service.UserFilterService) *FiltersHandler {
	return &FiltersHandler{filters: filters}
}

// GETFilters handles GET /api/v1/filters.
func (h *FiltersHandler) GETFilters(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	list, err := h.filters.ListFilters(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Filter, 0, len(list))
	for i := range list {
		out = append(out, apimodel.ToFilter(&list[i]))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETFilter handles GET /api/v1/filters/:id.
func (h *FiltersHandler) GETFilter(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	f, err := h.filters.GetFilter(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToFilter(f))
}

// POSTFiltersRequest is the body for POST /api/v1/filters.
type POSTFiltersRequest struct {
	Phrase       string   `json:"phrase"`
	Context      []string `json:"context"`
	WholeWord    bool     `json:"whole_word"`
	ExpiresAt    *string  `json:"expires_at"`
	Irreversible bool     `json:"irreversible"`
}

// POSTFilters handles POST /api/v1/filters.
func (h *FiltersHandler) POSTFilters(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	var body POSTFiltersRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	f, err := h.filters.CreateFilter(r.Context(), account.ID, body.Phrase, body.Context, body.WholeWord, body.ExpiresAt, body.Irreversible)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			api.HandleError(w, r, api.NewUnprocessableError("phrase is required"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToFilter(f))
}

// PUTFilterRequest is the body for PUT /api/v1/filters/:id.
type PUTFilterRequest struct {
	Phrase       string   `json:"phrase"`
	Context      []string `json:"context"`
	WholeWord    bool     `json:"whole_word"`
	ExpiresAt    *string  `json:"expires_at"`
	Irreversible bool     `json:"irreversible"`
}

// PUTFilter handles PUT /api/v1/filters/:id.
func (h *FiltersHandler) PUTFilter(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var body PUTFilterRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	f, err := h.filters.UpdateFilter(r.Context(), account.ID, id, body.Phrase, body.Context, body.WholeWord, body.ExpiresAt, body.Irreversible)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToFilter(f))
}

// DELETEFilter handles DELETE /api/v1/filters/:id.
func (h *FiltersHandler) DELETEFilter(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	if err := h.filters.DeleteFilter(r.Context(), account.ID, id); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
