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

// Status is a post, reblog, or reply in the federated timeline.
type Status struct {
	ID                  string
	URI                 string
	AccountID           string
	Text                *string
	Content             *string
	ContentWarning      *string
	Visibility          string
	Language            *string
	InReplyToID         *string
	InReplyToAccountID  *string
	ReblogOfID          *string
	QuotedStatusID      *string
	QuoteApprovalPolicy string // public | followers | nobody
	QuotesCount         int
	APID                string
	Sensitive           bool
	Local               bool
	EditedAt            *time.Time
	RepliesCount        int
	ReblogsCount        int
	FavouritesCount     int
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           *time.Time
}

// StatusEdit records one revision of an edited status.
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

// Quote approval policy values (who may quote this status).
const (
	QuotePolicyPublic    = "public"
	QuotePolicyFollowers = "followers"
	QuotePolicyNobody    = "nobody"
)

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

// QuoteApprovalRecord is the persistence record for a quote (quoting status -> quoted status); used to derive API quote_approval state.
type QuoteApprovalRecord struct {
	QuotingStatusID string
	QuotedStatusID  string
	RevokedAt       *time.Time
}

// Favourite records that an account favourited (liked) a status.
type Favourite struct {
	ID        string
	AccountID string
	StatusID  string
	APID      string
	CreatedAt time.Time
}
