package monstera

import (
	"net/http"
	"time"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/go-chi/chi/v5"
)

// ModeratorInvitesHandler handles invite code management.
type ModeratorInvitesHandler struct {
	registration service.RegistrationService
	settings     service.MonsteraSettingsService
}

// NewModeratorInvitesHandler returns a new ModeratorInvitesHandler.
func NewModeratorInvitesHandler(registration service.RegistrationService, settings service.MonsteraSettingsService) *ModeratorInvitesHandler {
	return &ModeratorInvitesHandler{registration: registration, settings: settings}
}

// GETInvites returns invites created by the current user.
func (h *ModeratorInvitesHandler) GETInvites(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.ErrForbidden)
		return
	}
	invites, err := h.registration.ListInvites(r.Context(), user.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.AdminInvite, 0, len(invites))
	for _, inv := range invites {
		out = append(out, apimodel.ToAdminInvite(&inv))
	}
	api.WriteJSON(w, http.StatusOK, apimodel.AdminInviteList{Invites: out})
}

// POSTInvites creates a new invite.
func (h *ModeratorInvitesHandler) POSTInvites(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.ErrForbidden)
		return
	}
	var maxUses *int
	var expiresAt *time.Time
	if s, err := h.settings.Get(r.Context()); err == nil {
		maxUses = s.InviteMaxUses
		if s.InviteExpiresInDays != nil {
			t := time.Now().AddDate(0, 0, *s.InviteExpiresInDays)
			expiresAt = &t
		}
	}
	inv, err := h.registration.CreateInvite(r.Context(), user.ID, maxUses, expiresAt)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, apimodel.ToAdminInvite(inv))
}

// DELETEInvite revokes an invite.
func (h *ModeratorInvitesHandler) DELETEInvite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	if err := h.registration.RevokeInvite(r.Context(), id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
