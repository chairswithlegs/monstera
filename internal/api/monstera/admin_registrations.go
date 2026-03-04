package monstera

import (
	"fmt"
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/go-chi/chi/v5"
)

// AdminRegistrationsHandler handles pending registration approval/rejection.
type AdminRegistrationsHandler struct {
	accounts     service.AccountService
	registration service.RegistrationService
}

// NewAdminRegistrationsHandler returns a new AdminRegistrationsHandler.
func NewAdminRegistrationsHandler(accounts service.AccountService, registration service.RegistrationService) *AdminRegistrationsHandler {
	return &AdminRegistrationsHandler{accounts: accounts, registration: registration}
}

func (h *AdminRegistrationsHandler) moderatorUserID(r *http.Request) (string, error) {
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

// GETRegistrations returns the list of pending registrations.
func (h *AdminRegistrationsHandler) GETRegistrations(w http.ResponseWriter, r *http.Request) {
	if _, err := h.moderatorUserID(r); err != nil {
		api.HandleError(w, r, err)
		return
	}
	pending, err := h.registration.ListPending(r.Context())
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.AdminPendingRegistration, 0, len(pending))
	for _, p := range pending {
		out = append(out, apimodel.AdminPendingRegistration{
			UserID:             p.User.ID,
			AccountID:          p.User.AccountID,
			Email:              p.User.Email,
			Username:           p.Account.Username,
			RegistrationReason: p.User.RegistrationReason,
		})
	}
	api.WriteJSON(w, http.StatusOK, apimodel.AdminPendingRegistrationList{Pending: out})
}

// POSTApprove approves a pending registration.
func (h *AdminRegistrationsHandler) POSTApprove(w http.ResponseWriter, r *http.Request) {
	modID, err := h.moderatorUserID(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	if err := h.registration.Approve(r.Context(), modID, id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POSTReject rejects a pending registration.
func (h *AdminRegistrationsHandler) POSTReject(w http.ResponseWriter, r *http.Request) {
	modID, err := h.moderatorUserID(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	if err := h.registration.Reject(r.Context(), modID, id, body.Reason); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
