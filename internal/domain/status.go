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

const (
	VisibilityPublic   = "public"
	VisibilityUnlisted = "unlisted"
	VisibilityPrivate  = "private"
	VisibilityDirect   = "direct"
)
