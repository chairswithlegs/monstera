package domain

import "time"

const (
	FilterContextHome          = "home"
	FilterContextNotifications = "notifications"
	FilterContextPublic        = "public"
	FilterContextThread        = "thread"
	FilterContextAccount       = "account"

	FilterActionWarn = "warn"
	FilterActionHide = "hide"
)

// FilterKeyword is one keyword entry within a user filter.
type FilterKeyword struct {
	ID        string
	FilterID  string
	Keyword   string
	WholeWord bool
}

// FilterStatus is one status entry within a user filter.
type FilterStatus struct {
	ID       string
	FilterID string
	StatusID string
}

// UserFilter is a per-account content filter with multiple keywords and
// optional status-based filter entries.
type UserFilter struct {
	ID           string
	AccountID    string
	Title        string
	Context      []string
	ExpiresAt    *time.Time
	FilterAction string // "warn" or "hide"
	Keywords     []FilterKeyword
	Statuses     []FilterStatus
	CreatedAt    time.Time
}

// FilterResult describes a filter match on a status, returned in the status.filtered array.
type FilterResult struct {
	Filter         UserFilter
	KeywordMatches []string
	StatusMatches  []string
}
