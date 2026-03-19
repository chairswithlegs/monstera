package domain

import (
	"encoding/json"
	"time"
)

// Account is the federated identity exposed via ActivityPub and the Mastodon API.
type Account struct {
	ID            string  // Internal ID for the account.
	Username      string  // Username for the account. This maps to the ActivityPub Actor "preferredUsername" field.
	Domain        *string // Domain for the account, null for local accounts.
	DisplayName   *string // Vanity name for the account. This maps to the ActivityPub Actor "name" field.
	Note          *string
	AvatarMediaID *string
	HeaderMediaID *string
	// AvatarURL and HeaderURL are stored directly on the accounts table.
	AvatarURL           string
	HeaderURL           string
	PublicKey           string
	PrivateKey          *string
	InboxURL            string // ActivityPub Inbox URL.
	OutboxURL           string // ActivityPub Outbox URL.
	FollowersURL        string // ActivityPub Followers URL.
	FollowingURL        string // ActivityPub Following URL.
	APID                string // ActivityPub IRI for the account.
	ProfileURL          string // Human-readable profile page URL (from AP Actor "url" field). For remote accounts stored from Actor; for local accounts computed at render time.
	FeaturedURL         string // ActivityPub featured collection URL. Remote accounts only.
	FollowersCount      int
	FollowingCount      int
	StatusesCount       int
	Fields              json.RawMessage
	Bot                 bool
	Locked              bool
	Suspended           bool
	Silenced            bool
	SuspensionOrigin    *string    // Origin of the suspension. Remote accounts only.
	LastBackfilledAt    *time.Time // Last time the account was backfilled. Remote accounts only.
	DeletionRequestedAt *time.Time // Time the account was requested to be deleted. Remote accounts only.
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
