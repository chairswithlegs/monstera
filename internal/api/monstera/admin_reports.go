package monstera

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/go-chi/chi/v5"
)

// AdminReportsHandler handles report queue and actions.
type AdminReportsHandler struct {
	accounts   service.AccountService
	moderation service.ModerationService
}

// NewAdminReportsHandler returns a new AdminReportsHandler.
func NewAdminReportsHandler(accounts service.AccountService, moderation service.ModerationService) *AdminReportsHandler {
	return &AdminReportsHandler{accounts: accounts, moderation: moderation}
}

func (h *AdminReportsHandler) moderatorUserID(r *http.Request) (string, error) {
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

// GETReports returns a paginated list of reports.
func (h *AdminReportsHandler) GETReports(w http.ResponseWriter, r *http.Request) {
	if _, err := h.moderatorUserID(r); err != nil {
		api.HandleError(w, r, err)
		return
	}
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
func (h *AdminReportsHandler) GETReport(w http.ResponseWriter, r *http.Request) {
	if _, err := h.moderatorUserID(r); err != nil {
		api.HandleError(w, r, err)
		return
	}
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
func (h *AdminReportsHandler) POSTAssign(w http.ResponseWriter, r *http.Request) {
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
		AssigneeID *string `json:"assignee_id"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	if err := h.moderation.AssignReport(r.Context(), modID, id, body.AssigneeID); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POSTResolve resolves a report.
func (h *AdminReportsHandler) POSTResolve(w http.ResponseWriter, r *http.Request) {
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
		Resolution string `json:"resolution"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	if err := h.moderation.ResolveReport(r.Context(), modID, id, body.Resolution); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
