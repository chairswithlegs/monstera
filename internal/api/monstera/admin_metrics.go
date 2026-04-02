package monstera

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
)

// AdminMetricsHandler serves aggregated server metrics for admin visibility.
type AdminMetricsHandler struct {
	svc service.AdminMetricsService
}

// NewAdminMetricsHandler returns a new AdminMetricsHandler.
func NewAdminMetricsHandler(svc service.AdminMetricsService) *AdminMetricsHandler {
	return &AdminMetricsHandler{svc: svc}
}

// GETMetrics returns aggregated server metrics.
func (h *AdminMetricsHandler) GETMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.svc.GetMetrics(r.Context())
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.AdminMetricsFromService(metrics))
}
