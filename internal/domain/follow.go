package domain

import "time"

// Follow represents a follow relationship between two accounts (pending or accepted).
type Follow struct {
	ID        string
	AccountID string
	TargetID  string
	State     string
	APID      *string
	CreatedAt time.Time
}

const (
	FollowStatePending  = "pending"
	FollowStateAccepted = "accepted"
)
