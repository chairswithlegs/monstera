package domain

import (
	"encoding/json"
	"time"
)

// IsLocal reports whether the account belongs to this instance.
func (a *Account) IsLocal() bool { return a.Domain == nil }

// IsRemote reports whether the account belongs to a remote instance.
func (a *Account) IsRemote() bool { return a.Domain != nil }

// IsHidden reports whether the account should be hidden from user-facing
// lookups — either individually suspended (moderator action or federation
// Delete{Person}) or currently covered by a severity=suspend domain block.
// The two causes are tracked separately so removing a domain block only
// reverses the domain-level hide; an individually suspended account stays
// hidden.
func (a *Account) IsHidden() bool { return a.Suspended || a.DomainSuspended }

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
	AvatarURL        string
	HeaderURL        string
	PublicKey        string
	PrivateKey       *string
	InboxURL         string // ActivityPub Inbox URL.
	OutboxURL        string // ActivityPub Outbox URL.
	FollowersURL     string // ActivityPub Followers URL.
	FollowingURL     string // ActivityPub Following URL.
	APID             string // ActivityPub IRI for the account.
	ProfileURL       string // Human-readable profile page URL (from AP Actor "url" field). For remote accounts stored from Actor; for local accounts computed at render time.
	FeaturedURL      string // ActivityPub featured collection URL. Remote accounts only.
	FollowersCount   int
	FollowingCount   int
	StatusesCount    int
	Fields           json.RawMessage
	Bot              bool
	Locked           bool
	Suspended        bool
	DomainSuspended  bool // hidden because an active severity=suspend domain block covers this account's domain; reset when the block is deleted
	Silenced         bool
	SuspensionOrigin *string    // Origin of the suspension. Remote accounts only.
	LastBackfilledAt *time.Time // Last time the account was backfilled. Remote accounts only.
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
