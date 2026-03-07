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

// AnnouncementsHandler handles Mastodon API announcement endpoints.
type AnnouncementsHandler struct {
	announcements service.AnnouncementService
}

// NewAnnouncementsHandler returns a new AnnouncementsHandler.
func NewAnnouncementsHandler(announcements service.AnnouncementService) *AnnouncementsHandler {
	return &AnnouncementsHandler{announcements: announcements}
}

// GETAnnouncements handles GET /api/v1/announcements.
func (h *AnnouncementsHandler) GETAnnouncements(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	list, err := h.announcements.ListActive(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Announcement, 0, len(list))
	for _, item := range list {
		out = append(out, apimodel.ToAnnouncement(item.Announcement, item.Read, item.Reactions))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTDismissAnnouncement handles POST /api/v1/announcements/:id/dismiss.
func (h *AnnouncementsHandler) POSTDismissAnnouncement(w http.ResponseWriter, r *http.Request) {
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
	err := h.announcements.Dismiss(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, struct{}{})
}

// PUTAnnouncementReaction handles PUT /api/v1/announcements/:id/reactions/:name.
func (h *AnnouncementsHandler) PUTAnnouncementReaction(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	name := chi.URLParam(r, "name")
	if id == "" || name == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	err := h.announcements.AddReaction(r.Context(), account.ID, id, name)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrValidation) {
			api.HandleError(w, r, api.NewUnprocessableError("Validation failed: Name is not a recognized emoji"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, struct{}{})
}

// DELETEAnnouncementReaction handles DELETE /api/v1/announcements/:id/reactions/:name.
func (h *AnnouncementsHandler) DELETEAnnouncementReaction(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	name := chi.URLParam(r, "name")
	if id == "" || name == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	err := h.announcements.RemoveReaction(r.Context(), account.ID, id, name)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrValidation) {
			api.HandleError(w, r, api.NewUnprocessableError("Validation failed: Name is not a recognized emoji"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, struct{}{})
}
