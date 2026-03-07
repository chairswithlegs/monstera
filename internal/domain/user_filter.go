package domain

import "time"

// UserFilter is a per-account content filter (e.g. hide posts containing a phrase).
type UserFilter struct {
	ID           string
	AccountID    string
	Phrase       string
	Context      []string // e.g. "home", "notifications", "public", "thread", "account"
	WholeWord    bool
	ExpiresAt    *time.Time
	Irreversible bool
	CreatedAt    time.Time
}

const (
	FilterContextHome          = "home"
	FilterContextNotifications = "notifications"
	FilterContextPublic        = "public"
	FilterContextThread        = "thread"
	FilterContextAccount       = "account"
)
