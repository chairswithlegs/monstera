package monstera

import (
	"fmt"
	"net/http"
	"time"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/go-chi/chi/v5"
)

// AdminInvitesHandler handles invite code management.
type AdminInvitesHandler struct {
	accounts     service.AccountService
	registration service.RegistrationService
}

// NewAdminInvitesHandler returns a new AdminInvitesHandler.
func NewAdminInvitesHandler(accounts service.AccountService, registration service.RegistrationService) *AdminInvitesHandler {
	return &AdminInvitesHandler{accounts: accounts, registration: registration}
}

func (h *AdminInvitesHandler) moderatorUserID(r *http.Request) (string, error) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		return "", api.ErrForbidden
	}
	_, user, err := h.accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		return "", fmt.Errorf("GetAccountWithUser: %w", err)
	}
	if user.Role != domain.RoleAdmin && user.Role != domain.RoleModerator {
		return "", api.ErrForbidden
	}
	return user.ID, nil
}

// GETInvites returns invites created by the current user.
func (h *AdminInvitesHandler) GETInvites(w http.ResponseWriter, r *http.Request) {
	userID, err := h.moderatorUserID(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	invites, err := h.registration.ListInvites(r.Context(), userID)
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
func (h *AdminInvitesHandler) POSTInvites(w http.ResponseWriter, r *http.Request) {
	userID, err := h.moderatorUserID(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	var body struct {
		MaxUses   *int       `json:"max_uses"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	inv, err := h.registration.CreateInvite(r.Context(), userID, body.MaxUses, body.ExpiresAt)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, apimodel.ToAdminInvite(inv))
}

// DELETEInvite revokes an invite.
func (h *AdminInvitesHandler) DELETEInvite(w http.ResponseWriter, r *http.Request) {
	if _, err := h.moderatorUserID(r); err != nil {
		api.HandleError(w, r, err)
		return
	}
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
