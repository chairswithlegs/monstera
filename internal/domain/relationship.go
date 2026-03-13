package domain

// Relationship holds the viewer-to-target relationship flags for the Mastodon API.
type Relationship struct {
	TargetID            string
	Following           bool
	ShowingReblogs      bool
	Notifying           bool
	FollowedBy          bool
	Blocking            bool
	BlockedBy           bool
	Muting              bool
	MutingNotifications bool
	Requested           bool
	DomainBlocking      bool
	Endorsed            bool
	Note                string
}
