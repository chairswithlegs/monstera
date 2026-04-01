package domain

import "time"

// NotificationFilterPolicy is a string enum for notification policy filter values.
// Valid values are "accept", "filter", "drop".
type NotificationFilterPolicy string

const (
	// NotificationFilterAccept allows notifications through (default).
	NotificationFilterAccept NotificationFilterPolicy = "accept"
	// NotificationFilterFilter diverts notifications to notification requests.
	NotificationFilterFilter NotificationFilterPolicy = "filter"
	// NotificationFilterDrop silently discards notifications.
	NotificationFilterDrop NotificationFilterPolicy = "drop"
)

// ShouldFilter returns true if the policy value is "filter" or "drop".
func (p NotificationFilterPolicy) ShouldFilter() bool {
	return p == NotificationFilterFilter || p == NotificationFilterDrop
}

// NotificationPolicy holds an account's notification filtering preferences.
type NotificationPolicy struct {
	ID                    string
	AccountID             string
	FilterNotFollowing    NotificationFilterPolicy
	FilterNotFollowers    NotificationFilterPolicy
	FilterNewAccounts     NotificationFilterPolicy
	FilterPrivateMentions NotificationFilterPolicy
	ForLimitedAccounts    NotificationFilterPolicy
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
