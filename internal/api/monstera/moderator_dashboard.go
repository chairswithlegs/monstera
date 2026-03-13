package monstera

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
)

// ModeratorDashboardHandler serves moderator dashboard stats.
type ModeratorDashboardHandler struct {
	instance   service.InstanceService
	moderation service.ModerationService
}

// NewModeratorDashboardHandler returns a new ModeratorDashboardHandler.
func NewModeratorDashboardHandler(instance service.InstanceService, moderation service.ModerationService) *ModeratorDashboardHandler {
	return &ModeratorDashboardHandler{instance: instance, moderation: moderation}
}

// GETDashboard returns dashboard stats.
func (h *ModeratorDashboardHandler) GETDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := h.instance.GetNodeInfoStats(ctx)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	reports, err := h.moderation.ListReports(ctx, "open", 10000, 0)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	resp := apimodel.AdminDashboard{
		LocalUsersCount:    stats.UserCount,
		LocalStatusesCount: stats.LocalPostCount,
		OpenReportsCount:   len(reports),
	}
	api.WriteJSON(w, http.StatusOK, resp)
}
