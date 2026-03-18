package domain

import "time"

// Notification is an activity (mention, reblog, favourite, etc.) delivered to an account.
type Notification struct {
	ID        string
	AccountID string
	FromID    string
	Type      string
	StatusID  *string
	Read      bool
	CreatedAt time.Time
}

const (
	NotificationTypeFollow        = "follow"
	NotificationTypeMention       = "mention"
	NotificationTypeReblog        = "reblog"
	NotificationTypeFavourite     = "favourite"
	NotificationTypeFollowRequest = "follow_request"
	NotificationTypeQuote         = "quote"
	NotificationTypeQuotedUpdate  = "quoted_update"
)
