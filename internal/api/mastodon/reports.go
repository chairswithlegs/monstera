package mastodon

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

var reportCategories = []string{
	domain.ReportCategorySpam, domain.ReportCategoryIllegal,
	domain.ReportCategoryViolation, domain.ReportCategoryOther,
}

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

func (r *POSTReportsRequest) Validate() error {
	if err := api.ValidateRequiredField(r.AccountID, "account_id"); err != nil {
		return fmt.Errorf("account_id: %w", err)
	}
	if r.Category == "" {
		r.Category = domain.ReportCategoryOther
	}
	if err := api.ValidateOneOf(r.Category, reportCategories, "category"); err != nil {
		return fmt.Errorf("category: %w", err)
	}
	return nil
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
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
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
		Category:  body.Category,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}

	// rep was just created: TargetID is guaranteed non-nil here because the
	// service path only sets it from the caller's body.AccountID (non-empty,
	// validated). Dereference directly; nil would be an internal bug.
	if rep.TargetID == nil {
		api.HandleError(w, r, api.ErrInternalServerError)
		return
	}
	target, err := h.accounts.GetByID(ctx, *rep.TargetID)
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
