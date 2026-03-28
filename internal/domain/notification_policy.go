package domain

import "time"

// NotificationPolicy holds an account's notification filtering preferences.
type NotificationPolicy struct {
	ID                    string
	AccountID             string
	FilterNotFollowing    bool
	FilterNotFollowers    bool
	FilterNewAccounts     bool
	FilterPrivateMentions bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// NotificationRequest represents a pending notification from an account that
// was filtered by the notification policy.
type NotificationRequest struct {
	ID                 string
	AccountID          string
	FromAccountID      string
	LastStatusID       *string
	NotificationsCount int32
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
