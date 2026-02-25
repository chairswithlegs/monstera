package domain

import "time"

type InstanceSetting struct {
	Key   string
	Value string
}

type Mute struct {
	ID                string
	AccountID         string
	TargetID          string
	HideNotifications bool
	CreatedAt         time.Time
}

type Block struct {
	ID        string
	AccountID string
	TargetID  string
	CreatedAt time.Time
}

type Hashtag struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Favourite struct {
	ID        string
	AccountID string
	StatusID  string
	CreatedAt time.Time
}
