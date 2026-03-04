package monstera

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
)

// AdminSettingsHandler handles instance settings (admin only; route protected by RequireAdmin).
type AdminSettingsHandler struct {
	instance service.InstanceService
}

// NewAdminSettingsHandler returns a new AdminSettingsHandler.
func NewAdminSettingsHandler(instance service.InstanceService) *AdminSettingsHandler {
	return &AdminSettingsHandler{instance: instance}
}

// GETSettings returns all instance settings.
func (h *AdminSettingsHandler) GETSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.instance.GetAllSettings(r.Context())
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.AdminSettings{Settings: settings})
}

// PUTSettings updates instance settings (merge with existing).
func (h *AdminSettingsHandler) PUTSettings(w http.ResponseWriter, r *http.Request) {
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
