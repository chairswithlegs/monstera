package mastodon

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// ReportsHandler handles Mastodon API report endpoints.
type ReportsHandler struct {
	moderation     service.ModerationService
	accounts       service.AccountService
	instanceDomain string
}

// NewReportsHandler returns a new ReportsHandler.
func NewReportsHandler(moderation service.ModerationService, accounts service.AccountService, instanceDomain string) *ReportsHandler {
	return &ReportsHandler{
		moderation:     moderation,
		accounts:       accounts,
		instanceDomain: instanceDomain,
	}
}

// POSTReportsRequest is the request body for POST /api/v1/reports.
type POSTReportsRequest struct {
	AccountID string   `json:"account_id"`
	StatusIDs []string `json:"status_ids"`
	Comment   string   `json:"comment"`
	Category  string   `json:"category"`
	RuleIDs   []string `json:"rule_ids"`
	Forward   bool     `json:"forward"` // Accepted but ignored; report forwarding is out of scope.
}

// POSTReports handles POST /api/v1/reports.
func (h *ReportsHandler) POSTReports(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}

	var body POSTReportsRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	if body.AccountID == "" {
		api.HandleError(w, r, api.NewUnprocessableError("account_id is required"))
		return
	}

	category := body.Category
	if category == "" {
		category = domain.ReportCategoryOther
	}
	switch category {
	case domain.ReportCategorySpam, domain.ReportCategoryIllegal, domain.ReportCategoryViolation, domain.ReportCategoryOther:
	default:
		api.HandleError(w, r, api.NewUnprocessableError("invalid category"))
		return
	}

	var comment *string
	if body.Comment != "" {
		comment = &body.Comment
	}

	rep, err := h.moderation.CreateReport(ctx, service.CreateReportInput{
		AccountID: account.ID,
		TargetID:  body.AccountID,
		StatusIDs: body.StatusIDs,
		Comment:   comment,
		Category:  category,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}

	target, err := h.accounts.GetByID(ctx, rep.TargetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, apimodel.ToReport(rep, target, h.instanceDomain))
}
