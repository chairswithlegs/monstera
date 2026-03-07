package apimodel

import (
	"github.com/chairswithlegs/monstera/internal/domain"
)

// Report is the Mastodon API report entity returned by POST /api/v1/reports.
type Report struct {
	ID            string   `json:"id"`
	ActionTaken   bool     `json:"action_taken"`
	ActionTakenAt *string  `json:"action_taken_at"`
	Category      string   `json:"category"`
	Comment       *string  `json:"comment"`
	Forwarded     bool     `json:"forwarded"` // Report forwarding is out of scope; always false.
	CreatedAt     string   `json:"created_at"`
	StatusIDs     []string `json:"status_ids"`
	RuleIDs       []string `json:"rule_ids"` // Optional; not stored in Phase 1.
	TargetAccount Account  `json:"target_account"`
}

const reportTimeFormat = "2006-01-02T15:04:05.000Z"

// ToReport converts a domain report and target account to the Mastodon API Report shape.
func ToReport(r *domain.Report, targetAccount *domain.Account, instanceDomain string) Report {
	createdAt := r.CreatedAt.UTC().Format(reportTimeFormat)
	out := Report{
		ID:            r.ID,
		ActionTaken:   r.State == domain.ReportStateResolved,
		Category:      r.Category,
		Comment:       r.Comment,
		Forwarded:     false,
		CreatedAt:     createdAt,
		StatusIDs:     r.StatusIDs,
		RuleIDs:       nil,
		TargetAccount: ToAccount(targetAccount, instanceDomain),
	}
	if r.ResolvedAt != nil {
		s := r.ResolvedAt.UTC().Format(reportTimeFormat)
		out.ActionTakenAt = &s
	}
	return out
}
