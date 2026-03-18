package domain

import "time"

// PushSubscription represents a Web Push subscription tied to one OAuth access token.
type PushSubscription struct {
	ID            string
	AccessTokenID string
	AccountID     string
	Endpoint      string
	KeyP256DH     string
	KeyAuth       string
	Alerts        PushAlerts
	Policy        string // "all", "followed", "follower", "none"
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// PushAlerts controls which notification types trigger a push.
type PushAlerts struct {
	Follow        bool `json:"follow"`
	Favourite     bool `json:"favourite"`
	Reblog        bool `json:"reblog"`
	Mention       bool `json:"mention"`
	Poll          bool `json:"poll"`
	Status        bool `json:"status"`
	Update        bool `json:"update"`
	FollowRequest bool `json:"follow_request"`
}
