package monstera

import (
	"net/http"
	"strconv"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/go-chi/chi/v5"
)

// ModeratorReportsHandler handles report queue and actions.
type ModeratorReportsHandler struct {
	moderation service.ModerationService
}

// NewModeratorReportsHandler returns a new ModeratorReportsHandler.
func NewModeratorReportsHandler(moderation service.ModerationService) *ModeratorReportsHandler {
	return &ModeratorReportsHandler{moderation: moderation}
}

// GETReports returns a paginated list of reports.
func (h *ModeratorReportsHandler) GETReports(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, _ := strconv.Atoi(l); n > 0 && n <= 100 {
			limit = n
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, _ := strconv.Atoi(o); n >= 0 {
			offset = n
		}
	}
	reports, err := h.moderation.ListReports(r.Context(), state, limit, offset)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.AdminReport, 0, len(reports))
	for i := range reports {
		out = append(out, apimodel.ToAdminReport(&reports[i]))
	}
	api.WriteJSON(w, http.StatusOK, apimodel.AdminReportList{Reports: out})
}

// GETReport returns a single report.
func (h *ModeratorReportsHandler) GETReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	rep, err := h.moderation.GetReport(r.Context(), id)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToAdminReport(rep))
}

// POSTAssign assigns a report to a user.
func (h *ModeratorReportsHandler) POSTAssign(w http.ResponseWriter, r *http.Request) {
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
		AssigneeID *string `json:"assignee_id"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	if err := h.moderation.AssignReport(r.Context(), user.ID, id, body.AssigneeID); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POSTResolve resolves a report.
func (h *ModeratorReportsHandler) POSTResolve(w http.ResponseWriter, r *http.Request) {
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
		Resolution string `json:"resolution"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	if err := h.moderation.ResolveReport(r.Context(), user.ID, id, body.Resolution); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
