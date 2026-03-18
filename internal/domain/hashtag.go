package domain

import "time"

// Hashtag is a tag used in statuses for discovery and trending.
type Hashtag struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// FeaturedTag is a hashtag featured on an account's profile.
type FeaturedTag struct {
	ID            string
	AccountID     string
	TagID         string
	Name          string
	StatusesCount int
	LastStatusAt  *time.Time
	CreatedAt     time.Time
}
