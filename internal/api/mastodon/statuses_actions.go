package mastodon

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
)

// POSTReblog handles POST /api/v1/statuses/:id/reblog.
func (h *StatusesHandler) POSTReblog(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.interactions.CreateReblog(r.Context(), account.ID, account.Username, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTUnreblog handles POST /api/v1/statuses/:id/unreblog.
func (h *StatusesHandler) POSTUnreblog(w http.ResponseWriter, r *http.Request) {
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
	if err := h.interactions.DeleteReblog(r.Context(), account.ID, id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	result, err := h.statuses.GetByIDEnriched(r.Context(), id, &account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, enrichedStatusToAPIModel(result, h.instanceDomain))
}

// POSTFavourite handles POST /api/v1/statuses/:id/favourite.
func (h *StatusesHandler) POSTFavourite(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.interactions.CreateFavourite(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Favourited = true
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTUnfavourite handles POST /api/v1/statuses/:id/unfavourite.
func (h *StatusesHandler) POSTUnfavourite(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.interactions.DeleteFavourite(r.Context(), account.ID, id)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Favourited = false
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTBookmark handles POST /api/v1/statuses/:id/bookmark.
func (h *StatusesHandler) POSTBookmark(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.interactions.Bookmark(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Bookmarked = true
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTUnbookmark handles POST /api/v1/statuses/:id/unbookmark.
func (h *StatusesHandler) POSTUnbookmark(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.interactions.Unbookmark(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Bookmarked = false
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTMuteConversation handles POST /api/v1/statuses/:id/mute (thread mute).
func (h *StatusesHandler) POSTMuteConversation(w http.ResponseWriter, r *http.Request) {
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
	if err := h.conversations.MuteConversation(r.Context(), account.ID, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	viewerID := &account.ID
	result, err := h.statuses.GetByIDEnriched(r.Context(), id, viewerID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Muted = true
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTUnmuteConversation handles POST /api/v1/statuses/:id/unmute (thread unmute).
func (h *StatusesHandler) POSTUnmuteConversation(w http.ResponseWriter, r *http.Request) {
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
	if err := h.conversations.UnmuteConversation(r.Context(), account.ID, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	viewerID := &account.ID
	result, err := h.statuses.GetByIDEnriched(r.Context(), id, viewerID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := enrichedStatusToAPIModel(result, h.instanceDomain)
	out.Muted = false
	api.WriteJSON(w, http.StatusOK, out)
}
