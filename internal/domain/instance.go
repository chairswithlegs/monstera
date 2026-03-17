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

// KnownInstance represents a remote instance discovered through federation.
type KnownInstance struct {
	ID              string
	Domain          string
	Software        *string
	SoftwareVersion *string
	FirstSeenAt     time.Time
	LastSeenAt      time.Time
	AccountsCount   int64
}
