package domain

import (
	"encoding/json"
	"time"
)

type Account struct {
	ID            string  // Internal ID for the account.
	Username      string  // Username for the account. This maps to the ActivityPub Actor "preferredUsername" field.
	Domain        *string // Domain for the account, null for local accounts.
	DisplayName   *string // Vanity name for the account. This maps to the ActivityPub Actor "name" field.
	Note          *string
	AvatarMediaID *string
	HeaderMediaID *string
	// AvatarURL and HeaderURL are populated by the store via LEFT JOIN on media_attachments.
	AvatarURL           string
	HeaderURL           string
	PublicKey           string
	PrivateKey          *string
	InboxURL            string          // ActivityPub Inbox URL.
	OutboxURL           string          // ActivityPub Outbox URL.
	FollowersURL        string          // ActivityPub Followers URL.
	FollowingURL        string          // ActivityPub Following URL.
	APID                string          // ActivityPub IRI for the account.
	APRaw               json.RawMessage // Raw ActivityPub Actor document.
	FollowersCount      int
	FollowingCount      int
	StatusesCount       int
	Fields              json.RawMessage
	Bot                 bool
	Locked              bool
	Suspended           bool
	Silenced            bool
	SuspensionOrigin    *string
	DeletionRequestedAt *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
