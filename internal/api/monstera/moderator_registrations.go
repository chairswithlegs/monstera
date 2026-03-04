package monstera

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/go-chi/chi/v5"
)

// ModeratorRegistrationsHandler handles pending registration approval/rejection.
type ModeratorRegistrationsHandler struct {
	registration service.RegistrationService
}

// NewModeratorRegistrationsHandler returns a new ModeratorRegistrationsHandler.
func NewModeratorRegistrationsHandler(registration service.RegistrationService) *ModeratorRegistrationsHandler {
	return &ModeratorRegistrationsHandler{registration: registration}
}

// GETRegistrations returns the list of pending registrations.
func (h *ModeratorRegistrationsHandler) GETRegistrations(w http.ResponseWriter, r *http.Request) {
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
func (h *ModeratorRegistrationsHandler) POSTApprove(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.ErrForbidden)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	if err := h.registration.Approve(r.Context(), user.ID, id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POSTReject rejects a pending registration.
func (h *ModeratorRegistrationsHandler) POSTReject(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.ErrForbidden)
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
	if err := h.registration.Reject(r.Context(), user.ID, id, body.Reason); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
