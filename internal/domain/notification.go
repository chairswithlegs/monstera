package domain

import "time"

// Notification is an activity (mention, reblog, favourite, etc.) delivered to an account.
type Notification struct {
	ID        string
	AccountID string
	FromID    string
	Type      string
	StatusID  *string
	GroupKey  string
	Read      bool
	CreatedAt time.Time
}

// NotificationGroup is a group of notifications with the same group_key.
type NotificationGroup struct {
	GroupKey                 string
	NotificationsCount       int
	Type                     string
	MostRecentNotificationID string
	PageMinID                string
	PageMaxID                string
	LatestPageNotificationAt time.Time
	SampleAccountIDs         []string
	StatusID                 *string
}

const (
	NotificationTypeFollow        = "follow"
	NotificationTypeMention       = "mention"
	NotificationTypeReblog        = "reblog"
	NotificationTypeFavourite     = "favourite"
	NotificationTypeFollowRequest = "follow_request"
	NotificationTypeQuote         = "quote"
	NotificationTypeQuotedUpdate  = "quoted_update"
	NotificationTypePoll          = "poll"
)
