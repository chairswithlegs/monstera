package domain

import "time"

// FilterKeyword is one keyword entry within a v2 user filter.
type FilterKeyword struct {
	ID        string
	FilterID  string
	Keyword   string
	WholeWord bool
}

// FilterStatus is one status entry within a v2 user filter.
type FilterStatus struct {
	ID       string
	FilterID string
	StatusID string
}

// UserFilterV2 is the v2 per-account content filter that supports multiple keywords
// and per-status matching, with explicit warn/hide action.
type UserFilterV2 struct {
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

// FilterResult describes a v2 filter match on a status, returned in the status.filtered array.
type FilterResult struct {
	Filter         UserFilterV2
	KeywordMatches []string
	StatusMatches  []string
}
