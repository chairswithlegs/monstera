package apimodel

import (
	"github.com/chairswithlegs/monstera/internal/domain"
)

const timeFormatRFC3339Milli = "2006-01-02T15:04:05.000Z"

// AdminReport is one report in the admin API. AccountID (reporter) and
// TargetID (reported) are nullable: they serialize to null when the referenced
// account has been deleted (FK ON DELETE SET NULL preserves the report row
// itself as moderation history).
type AdminReport struct {
	ID           string   `json:"id"`
	AccountID    *string  `json:"account_id"`
	TargetID     *string  `json:"target_id"`
	StatusIDs    []string `json:"status_ids"`
	Comment      *string  `json:"comment"`
	Category     string   `json:"category"`
	State        string   `json:"state"`
	AssignedToID *string  `json:"assigned_to_id"`
	ActionTaken  *string  `json:"action_taken"`
	CreatedAt    string   `json:"created_at"`
	ResolvedAt   *string  `json:"resolved_at,omitempty"`
}

// ToAdminReport converts a domain report to the admin API shape.
func ToAdminReport(r *domain.Report) AdminReport {
	createdAt := r.CreatedAt.Format(timeFormatRFC3339Milli)
	var resolvedAt *string
	if r.ResolvedAt != nil {
		s := r.ResolvedAt.Format(timeFormatRFC3339Milli)
		resolvedAt = &s
	}
	return AdminReport{
		ID:           r.ID,
		AccountID:    r.AccountID,
		TargetID:     r.TargetID,
		StatusIDs:    r.StatusIDs,
		Comment:      r.Comment,
		Category:     r.Category,
		State:        r.State,
		AssignedToID: r.AssignedToID,
		ActionTaken:  r.ActionTaken,
		CreatedAt:    createdAt,
		ResolvedAt:   resolvedAt,
	}
}

// AdminReportList is the response for GET /admin/reports.
type AdminReportList struct {
	Reports []AdminReport `json:"reports"`
}

type PostAssignReportRequest struct {
	AssigneeID *string `json:"assignee_id"`
}

func (b *PostAssignReportRequest) Validate() error { return nil }

type PostResolveReportRequest struct {
	Resolution string `json:"resolution"`
}

func (b *PostResolveReportRequest) Validate() error { return nil }
