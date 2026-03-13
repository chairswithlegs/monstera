package domain

import "time"

// List is a user-created list of accounts (e.g. for list timelines).
type List struct {
	ID            string
	AccountID     string
	Title         string
	RepliesPolicy string
	Exclusive     bool
	CreatedAt     time.Time
}

const (
	ListRepliesPolicyFollowed = "followed"
	ListRepliesPolicyList     = "list"
	ListRepliesPolicyNone     = "none"
)
