package monstera

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
)

// AdminSettingsHandler handles Monstera settings (admin only; route protected by RequireAdmin).
type AdminSettingsHandler struct {
	settings service.MonsteraSettingsService
}

// NewAdminSettingsHandler returns a new AdminSettingsHandler.
func NewAdminSettingsHandler(settings service.MonsteraSettingsService) *AdminSettingsHandler {
	return &AdminSettingsHandler{settings: settings}
}

// GETSettings returns the current Monstera settings.
func (h *AdminSettingsHandler) GETSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settings.Get(r.Context())
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.AdminSettingsFromDomain(settings))
}

// PUTSettings updates Monstera settings.
func (h *AdminSettingsHandler) PUTSettings(w http.ResponseWriter, r *http.Request) {
	var body apimodel.AdminSettings
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}

	settings := body.ToDomain()
	if err := h.settings.Update(r.Context(), settings); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
