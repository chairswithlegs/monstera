package domain

import (
	"encoding/json"
	"time"
)

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
