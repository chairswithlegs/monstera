package domain

import "time"

const (
	FilterContextHome          = "home"
	FilterContextNotifications = "notifications"
	FilterContextPublic        = "public"
	FilterContextThread        = "thread"
	FilterContextAccount       = "account"
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

// UserFilter is a per-account content filter. It supports both the v1 API
// (single phrase, stored in Phrase/WholeWord/Irreversible) and the v2 API
// (title, filter_action, and multiple Keywords/Statuses). The v1 and v2 fields
// are populated by their respective store paths.
type UserFilter struct {
	ID           string
	AccountID    string
	Title        string
	Phrase       string // v1 legacy; also stored as first keyword after migration
	Context      []string
	ExpiresAt    *time.Time
	FilterAction string // "warn" or "hide"
	WholeWord    bool   // v1 legacy
	Irreversible bool   // v1 legacy; corresponds to filter_action "hide"
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
