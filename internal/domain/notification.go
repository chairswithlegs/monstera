package domain

import "time"

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
)
