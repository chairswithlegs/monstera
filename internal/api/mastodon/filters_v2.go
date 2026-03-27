package mastodon

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// FiltersV2Handler handles /api/v2/filters and /api/v2/filter_keywords and /api/v2/filter_statuses endpoints.
type FiltersV2Handler struct {
	filters service.FilterService
}

// NewFiltersV2Handler returns a new FiltersV2Handler.
func NewFiltersV2Handler(filters service.FilterService) *FiltersV2Handler {
	return &FiltersV2Handler{filters: filters}
}

// ─── Filter CRUD ─────────────────────────────────────────────────────────────

// GETFiltersV2 handles GET /api/v2/filters.
func (h *FiltersV2Handler) GETFiltersV2(w http.ResponseWriter, r *http.Request) {
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
	out := make([]apimodel.FilterV2, 0, len(list))
	for i := range list {
		out = append(out, apimodel.ToFilterV2(&list[i]))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETFilterV2 handles GET /api/v2/filters/:id.
func (h *FiltersV2Handler) GETFilterV2(w http.ResponseWriter, r *http.Request) {
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
	api.WriteJSON(w, http.StatusOK, apimodel.ToFilterV2(f))
}

// postFilterV2Request is the request body for POST /api/v2/filters.
type postFilterV2Request struct {
	Title        string   `json:"title"`
	Context      []string `json:"context"`
	FilterAction string   `json:"filter_action"`
	ExpiresIn    *int     `json:"expires_in"`
}

func (b *postFilterV2Request) Validate() error {
	return api.ValidateRequiredField(b.Title, "title")
}

// POSTFiltersV2 handles POST /api/v2/filters.
func (h *FiltersV2Handler) POSTFiltersV2(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	var body postFilterV2Request
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	f, err := h.filters.CreateFilter(r.Context(), account.ID, body.Title, body.Context, nil, body.FilterAction)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			api.HandleError(w, r, api.NewMissingRequiredFieldError("title"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToFilterV2(f))
}

// putFilterV2Request is the request body for PUT /api/v2/filters/:id.
type putFilterV2Request struct {
	Title        string   `json:"title"`
	Context      []string `json:"context"`
	FilterAction string   `json:"filter_action"`
	ExpiresIn    *int     `json:"expires_in"`
}

func (b *putFilterV2Request) Validate() error {
	return api.ValidateRequiredField(b.Title, "title")
}

// PUTFilterV2 handles PUT /api/v2/filters/:id.
func (h *FiltersV2Handler) PUTFilterV2(w http.ResponseWriter, r *http.Request) {
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
	var body putFilterV2Request
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	f, err := h.filters.UpdateFilter(r.Context(), account.ID, id, body.Title, body.Context, nil, body.FilterAction)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToFilterV2(f))
}

// DELETEFilterV2 handles DELETE /api/v2/filters/:id.
func (h *FiltersV2Handler) DELETEFilterV2(w http.ResponseWriter, r *http.Request) {
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
		if errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ─── Filter keywords ─────────────────────────────────────────────────────────

// GETFilterKeywords handles GET /api/v2/filters/:id/keywords.
func (h *FiltersV2Handler) GETFilterKeywords(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	filterID := chi.URLParam(r, "id")
	if filterID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	kws, err := h.filters.ListKeywords(r.Context(), account.ID, filterID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.FilterKeyword, 0, len(kws))
	for i := range kws {
		out = append(out, apimodel.ToFilterKeyword(&kws[i]))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// postFilterKeywordRequest is the body for POST /api/v2/filters/:id/keywords.
type postFilterKeywordRequest struct {
	Keyword   string `json:"keyword"`
	WholeWord bool   `json:"whole_word"`
}

func (b *postFilterKeywordRequest) Validate() error {
	return api.ValidateRequiredField(b.Keyword, "keyword")
}

// POSTFilterKeyword handles POST /api/v2/filters/:id/keywords.
func (h *FiltersV2Handler) POSTFilterKeyword(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	filterID := chi.URLParam(r, "id")
	if filterID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var body postFilterKeywordRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	kw, err := h.filters.AddKeyword(r.Context(), account.ID, filterID, body.Keyword, body.WholeWord)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToFilterKeyword(kw))
}

// GETFilterKeyword handles GET /api/v2/filter_keywords/:id.
func (h *FiltersV2Handler) GETFilterKeyword(w http.ResponseWriter, r *http.Request) {
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
	kw, err := h.filters.GetKeyword(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToFilterKeyword(kw))
}

// putFilterKeywordRequest is the body for PUT /api/v2/filter_keywords/:id.
type putFilterKeywordRequest struct {
	Keyword   string `json:"keyword"`
	WholeWord bool   `json:"whole_word"`
}

func (b *putFilterKeywordRequest) Validate() error {
	return api.ValidateRequiredField(b.Keyword, "keyword")
}

// PUTFilterKeyword handles PUT /api/v2/filter_keywords/:id.
func (h *FiltersV2Handler) PUTFilterKeyword(w http.ResponseWriter, r *http.Request) {
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
	var body putFilterKeywordRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	kw, err := h.filters.UpdateKeyword(r.Context(), account.ID, id, body.Keyword, body.WholeWord)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToFilterKeyword(kw))
}

// DELETEFilterKeyword handles DELETE /api/v2/filter_keywords/:id.
func (h *FiltersV2Handler) DELETEFilterKeyword(w http.ResponseWriter, r *http.Request) {
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
	if err := h.filters.DeleteKeyword(r.Context(), account.ID, id); err != nil {
		if errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ─── Filter statuses ─────────────────────────────────────────────────────────

// GETFilterStatuses handles GET /api/v2/filters/:id/statuses.
func (h *FiltersV2Handler) GETFilterStatuses(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	filterID := chi.URLParam(r, "id")
	if filterID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	fsts, err := h.filters.ListFilterStatuses(r.Context(), account.ID, filterID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.FilterStatus, 0, len(fsts))
	for i := range fsts {
		out = append(out, apimodel.ToFilterStatus(&fsts[i]))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// postFilterStatusRequest is the body for POST /api/v2/filters/:id/statuses.
type postFilterStatusRequest struct {
	StatusID string `json:"status_id"`
}

func (b *postFilterStatusRequest) Validate() error {
	return api.ValidateRequiredField(b.StatusID, "status_id")
}

// POSTFilterStatus handles POST /api/v2/filters/:id/statuses.
func (h *FiltersV2Handler) POSTFilterStatus(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	filterID := chi.URLParam(r, "id")
	if filterID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var body postFilterStatusRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	fs, err := h.filters.AddFilterStatus(r.Context(), account.ID, filterID, body.StatusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToFilterStatus(fs))
}

// GETFilterStatus handles GET /api/v2/filter_statuses/:id.
func (h *FiltersV2Handler) GETFilterStatus(w http.ResponseWriter, r *http.Request) {
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
	fs, err := h.filters.GetFilterStatus(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToFilterStatus(fs))
}

// DELETEFilterStatus handles DELETE /api/v2/filter_statuses/:id.
func (h *FiltersV2Handler) DELETEFilterStatus(w http.ResponseWriter, r *http.Request) {
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
	if err := h.filters.DeleteFilterStatus(r.Context(), account.ID, id); err != nil {
		if errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
