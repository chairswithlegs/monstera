package monstera

import (
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// AdminSettingsHandler handles instance settings (admin only).
type AdminSettingsHandler struct {
	accounts service.AccountService
	instance service.InstanceService
}

// NewAdminSettingsHandler returns a new AdminSettingsHandler.
func NewAdminSettingsHandler(accounts service.AccountService, instance service.InstanceService) *AdminSettingsHandler {
	return &AdminSettingsHandler{accounts: accounts, instance: instance}
}

func (h *AdminSettingsHandler) requireAdmin(r *http.Request) bool {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		return false
	}
	_, user, err := h.accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		return false
	}
	return user.Role == domain.RoleAdmin
}

// GETSettings returns all instance settings.
func (h *AdminSettingsHandler) GETSettings(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		api.HandleError(w, r, api.ErrForbidden)
		return
	}
	settings, err := h.instance.GetAllSettings(r.Context())
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.AdminSettings{Settings: settings})
}

// PUTSettings updates instance settings (merge with existing).
func (h *AdminSettingsHandler) PUTSettings(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		api.HandleError(w, r, api.ErrForbidden)
		return
	}
	var body struct {
		Settings map[string]string `json:"settings"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	for k, v := range body.Settings {
		if err := h.instance.SetSetting(r.Context(), k, v); err != nil {
			api.HandleError(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
