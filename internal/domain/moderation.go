package domain

import "time"

// Mute hides a target account's posts and optionally notifications from the muter.
type Mute struct {
	ID                string
	AccountID         string
	TargetID          string
	HideNotifications bool
	CreatedAt         time.Time
}

// Block prevents interaction between two accounts; both see each other as blocked.
type Block struct {
	ID        string
	AccountID string
	TargetID  string
	CreatedAt time.Time
}

// DomainBlock restricts federation with a remote instance (silence or suspend).
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

// Report is a moderation report filed by a user against an account or content.
type Report struct {
	ID string
	// AccountID is the reporter. Nullable because the reporter's account may
	// have been deleted since the report was filed (FK ON DELETE SET NULL).
	AccountID *string
	// TargetID is the reported account. Nullable for the same reason as
	// AccountID — preserves moderation history after account deletion.
	TargetID     *string
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

// Invite is a registration invite code with optional use limit and expiry.
type Invite struct {
	ID        string
	Code      string
	CreatedBy string
	MaxUses   *int
	Uses      int
	ExpiresAt *time.Time
	CreatedAt time.Time
}

// AdminAction records a moderator action (suspend, silence, etc.) for audit.
type AdminAction struct {
	ID              string
	ModeratorID     string
	TargetAccountID *string
	Action          string
	Comment         *string
	Metadata        []byte
	CreatedAt       time.Time
}
