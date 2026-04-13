package domain

import "time"

// FilterContext identifies where a content filter should be applied.
type FilterContext string

const (
	FilterContextHome          FilterContext = "home"
	FilterContextNotifications FilterContext = "notifications"
	FilterContextPublic        FilterContext = "public"
	FilterContextThread        FilterContext = "thread"
	FilterContextAccount       FilterContext = "account"
)

// FilterAction determines the server/client behavior when a filter matches.
type FilterAction string

const (
	FilterActionWarn FilterAction = "warn"
	FilterActionHide FilterAction = "hide"
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
	Context      []FilterContext
	ExpiresAt    *time.Time
	FilterAction FilterAction
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
