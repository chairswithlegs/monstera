package mastodon

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
)

// GETContext handles GET /api/v1/statuses/:id/context.
func (h *StatusesHandler) GETContext(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	ctxResult, err := h.statuses.GetContext(r.Context(), id, viewerID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	ancestorIDs := make([]string, len(ctxResult.Ancestors))
	for i := range ctxResult.Ancestors {
		ancestorIDs[i] = ctxResult.Ancestors[i].ID
	}
	enrichedAncestors, err := h.statuses.GetByIDsEnriched(r.Context(), ancestorIDs, viewerID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	ancestors := make([]apimodel.Status, 0, len(enrichedAncestors))
	for i := range enrichedAncestors {
		ancestors = append(ancestors, enrichedStatusToAPIModel(enrichedAncestors[i], h.instanceDomain))
	}
	descendantIDs := make([]string, len(ctxResult.Descendants))
	for i := range ctxResult.Descendants {
		descendantIDs[i] = ctxResult.Descendants[i].ID
	}
	enrichedDescendants, err := h.statuses.GetByIDsEnriched(r.Context(), descendantIDs, viewerID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	descendants := make([]apimodel.Status, 0, len(enrichedDescendants))
	for i := range enrichedDescendants {
		descendants = append(descendants, enrichedStatusToAPIModel(enrichedDescendants[i], h.instanceDomain))
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ancestors":   ancestors,
		"descendants": descendants,
	})
}

// GETQuotes handles GET /api/v1/statuses/:id/quotes (Mastodon-style quotes). Auth required. Returns statuses that quote the given status.
func (h *StatusesHandler) GETQuotes(w http.ResponseWriter, r *http.Request) {
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
	params := PageParamsFromRequest(r)
	maxID := optionalString(params.MaxID)
	limit := params.Limit
	if limit <= 0 {
		limit = DefaultPageLimit
	}
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	}
	enriched, err := h.statuses.ListQuotesOfStatus(r.Context(), id, maxID, limit, &account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Status, 0, len(enriched))
	for i := range enriched {
		s := enrichedStatusToAPIModel(enriched[i], h.instanceDomain)
		h.setQuoteApprovalOnStatus(r.Context(), enriched[i], &s, &account.ID)
		out = append(out, s)
	}
	firstID, lastID := firstLastIDsFromEnriched(enriched)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETStatusHistory handles GET /api/v1/statuses/:id/history.
func (h *StatusesHandler) GETStatusHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	edits, err := h.statuses.GetStatusHistory(r.Context(), id, viewerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.StatusEdit, 0, len(edits))
	if len(edits) > 0 {
		author, err := h.accounts.GetByID(r.Context(), edits[0].AccountID)
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		authorAPI := apimodel.ToAccount(author, h.instanceDomain)
		for _, e := range edits {
			out = append(out, apimodel.StatusEditFromDomain(e, authorAPI))
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETStatusSource handles GET /api/v1/statuses/:id/source.
func (h *StatusesHandler) GETStatusSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	text, spoiler, err := h.statuses.GetStatusSource(r.Context(), id, viewerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.StatusSource{
		ID:          id,
		Text:        text,
		SpoilerText: spoiler,
	})
}

// GETFavouritedBy handles GET /api/v1/statuses/:id/favourited_by.
func (h *StatusesHandler) GETFavouritedBy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	params := PageParamsFromRequest(r)
	list, err := h.statuses.GetFavouritedBy(r.Context(), id, viewerID, optionalString(params.MaxID), params.Limit)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(list))
	for _, a := range list {
		out = append(out, apimodel.ToAccount(a, h.instanceDomain))
	}
	firstID, lastID := firstLastAccountIDs(list)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETRebloggedBy handles GET /api/v1/statuses/:id/reblogged_by.
func (h *StatusesHandler) GETRebloggedBy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var viewerID *string
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		viewerID = &account.ID
	}
	params := PageParamsFromRequest(r)
	list, err := h.statuses.GetRebloggedBy(r.Context(), id, viewerID, optionalString(params.MaxID), params.Limit)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(list))
	for _, a := range list {
		out = append(out, apimodel.ToAccount(a, h.instanceDomain))
	}
	firstID, lastID := firstLastAccountIDs(list)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}
