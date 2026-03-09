package domain

import "time"

// Announcement is an admin-created announcement shown to users.
type Announcement struct {
	ID          string
	Content     string
	StartsAt    *time.Time
	EndsAt      *time.Time
	AllDay      bool
	PublishedAt time.Time
	UpdatedAt   time.Time
}

// AnnouncementReactionCount is the count and "me" for one reaction name on an announcement.
type AnnouncementReactionCount struct {
	Name  string
	Count int
	Me    bool
}

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

type Favourite struct {
	ID        string
	AccountID string
	StatusID  string
	APID      string
	CreatedAt time.Time
}
