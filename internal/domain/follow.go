package domain

import "time"

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
