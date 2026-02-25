package domain

import "time"

type DomainBlock struct {
	ID        string
	Domain    string
	Severity  string
	Reason    *string
	CreatedAt time.Time
}

const (
	DomainBlockSeveritySilence = "silence"
	DomainBlockSeveritySuspend = "suspend"
)

type Report struct {
	ID           string
	AccountID    string
	TargetID     string
	StatusIDs    []string
	Comment      *string
	Category     string
	State        string
	AssignedToID *string
	ActionTaken  *string
	CreatedAt    time.Time
	ResolvedAt   *time.Time
}

const (
	ReportCategorySpam      = "spam"
	ReportCategoryIllegal   = "illegal"
	ReportCategoryViolation = "violation"
	ReportCategoryOther     = "other"
)

const (
	ReportStateOpen     = "open"
	ReportStateResolved = "resolved"
)

type Invite struct {
	ID        string
	Code      string
	CreatedBy string
	MaxUses   *int
	Uses      int
	ExpiresAt *time.Time
	CreatedAt time.Time
}

type ServerFilter struct {
	ID        string
	Phrase    string
	Scope     string
	Action    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

const (
	ServerFilterScopePublicTimeline = "public_timeline"
	ServerFilterScopeAll            = "all"
)

const (
	ServerFilterActionWarn = "warn"
	ServerFilterActionHide = "hide"
)
