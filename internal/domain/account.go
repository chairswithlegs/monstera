package domain

import (
	"encoding/json"
	"time"
)

type Account struct {
	ID                  string
	Username            string
	Domain              *string
	DisplayName         *string
	Note                *string
	AvatarMediaID       *string
	HeaderMediaID       *string
	PublicKey           string
	PrivateKey          *string
	InboxURL            string
	OutboxURL           string
	FollowersURL        string
	FollowingURL        string
	APID                string
	APRaw               json.RawMessage
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
