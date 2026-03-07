package domain

import (
	"encoding/json"
	"time"
)

// ScheduledStatus is a status scheduled for future publication.
type ScheduledStatus struct {
	ID          string
	AccountID   string
	Params      json.RawMessage
	ScheduledAt time.Time
	CreatedAt   time.Time
}

// ScheduledStatusParams is the JSON shape stored in scheduled_statuses.params for replay into CreateWithContent.
type ScheduledStatusParams struct {
	Text        string   `json:"text"`
	Visibility  string   `json:"visibility"`
	SpoilerText string   `json:"spoiler_text"`
	Sensitive   bool     `json:"sensitive"`
	Language    string   `json:"language"`
	InReplyToID string   `json:"in_reply_to_id"`
	MediaIDs    []string `json:"media_ids"`
}

type Status struct {
	ID                 string
	URI                string
	AccountID          string
	Text               *string
	Content            *string
	ContentWarning     *string
	Visibility         string
	Language           *string
	InReplyToID        *string
	InReplyToAccountID *string
	ReblogOfID         *string
	APID               string
	APRaw              json.RawMessage
	Sensitive          bool
	Local              bool
	EditedAt           *time.Time
	RepliesCount       int
	ReblogsCount       int
	FavouritesCount    int
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          *time.Time
}

type StatusEdit struct {
	ID             string
	StatusID       string
	AccountID      string
	Text           *string
	Content        *string
	ContentWarning *string
	Sensitive      bool
	CreatedAt      time.Time
}

// Status visibility determines who can read a status. When checking access, viewer may be nil (unauthenticated).
//
//	                No viewer   Viewer=author   Viewer follows author   Viewer in mentions
//	public          yes        yes             yes                      yes
//	unlisted        yes        yes             yes                      yes
//	private         no         yes             yes                      yes
//	direct          no         yes             no                       yes
//
// Regardless of visibility, if the viewer has blocked the author or the author has blocked the viewer, the status is not visible.
const (
	VisibilityPublic   = "public"
	VisibilityUnlisted = "unlisted"
	VisibilityPrivate  = "private"
	VisibilityDirect   = "direct"
)

// Poll is a poll attached to a status.
type Poll struct {
	ID        string
	StatusID  string
	ExpiresAt *time.Time
	Multiple  bool
	CreatedAt time.Time
}

// PollOption is one option in a poll.
type PollOption struct {
	ID       string
	PollID   string
	Title    string
	Position int
}
