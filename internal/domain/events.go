package domain

import "encoding/json"

// Domain event type constants. These are used as the event_type column in the
// outbox_events table and as NATS subject suffixes (domain.events.<type>).
const (
	EventStatusCreated       = "status.created"
	EventStatusDeleted       = "status.deleted"
	EventStatusUpdated       = "status.updated"
	EventStatusCreatedRemote = "status.created.remote"
	EventStatusDeletedRemote = "status.deleted.remote"
	EventFollowCreated       = "follow.created"
	EventFollowRemoved       = "follow.removed"
	EventFollowAccepted      = "follow.accepted"
	EventBlockCreated        = "block.created"
	EventBlockRemoved        = "block.removed"
	EventAccountUpdated      = "account.updated"
	EventNotificationCreated = "notification.created"
)

// DomainEvent is the envelope stored in the outbox_events table and published
// to NATS. Subscribers unmarshal Payload into the appropriate typed struct
// based on EventType.
type DomainEvent struct {
	ID            string          `json:"id"`
	EventType     string          `json:"event_type"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	Payload       json.RawMessage `json:"payload"`
}

// StatusCreatedPayload carries data for a locally created status. Used by both
// the federation subscriber (to build Create{Note}) and the SSE subscriber.
type StatusCreatedPayload struct {
	Status              *Status           `json:"status"`
	Author              *Account          `json:"author"`
	Mentions            []*Account        `json:"mentions"`
	Tags                []Hashtag         `json:"tags"`
	Media               []MediaAttachment `json:"media"`
	MentionedAccountIDs []string          `json:"mentioned_account_ids"`
}

// StatusDeletedPayload carries data for a deleted status.
type StatusDeletedPayload struct {
	StatusID            string   `json:"status_id"`
	AccountID           string   `json:"account_id"`
	Author              *Account `json:"author"`
	Visibility          string   `json:"visibility"`
	Local               bool     `json:"local"`
	APID                string   `json:"ap_id"`
	URI                 string   `json:"uri"`
	HashtagNames        []string `json:"hashtag_names"`
	MentionedAccountIDs []string `json:"mentioned_account_ids"`
}

// StatusUpdatedPayload carries data for an edited status.
type StatusUpdatedPayload struct {
	Status *Status  `json:"status"`
	Author *Account `json:"author"`
}

// FollowCreatedPayload carries data when a local user follows someone.
type FollowCreatedPayload struct {
	Follow *Follow  `json:"follow"`
	Actor  *Account `json:"actor"`
	Target *Account `json:"target"`
}

// FollowRemovedPayload carries data when a follow is removed (unfollow).
type FollowRemovedPayload struct {
	FollowID string   `json:"follow_id"`
	Actor    *Account `json:"actor"`
	Target   *Account `json:"target"`
}

// FollowAcceptedPayload carries data when a follow request is accepted.
// Target is the local account that accepted; Actor is the follower.
type FollowAcceptedPayload struct {
	Follow *Follow  `json:"follow"`
	Target *Account `json:"target"`
	Actor  *Account `json:"actor"`
}

// BlockCreatedPayload carries data when a user blocks another.
type BlockCreatedPayload struct {
	Actor  *Account `json:"actor"`
	Target *Account `json:"target"`
}

// BlockRemovedPayload carries data when a block is removed.
type BlockRemovedPayload struct {
	Actor  *Account `json:"actor"`
	Target *Account `json:"target"`
}

// AccountUpdatedPayload carries data when a user updates their profile.
type AccountUpdatedPayload struct {
	Account *Account `json:"account"`
}

// NotificationCreatedPayload carries data for a new notification (SSE-only).
type NotificationCreatedPayload struct {
	RecipientAccountID string        `json:"recipient_account_id"`
	Notification       *Notification `json:"notification"`
	FromAccount        *Account      `json:"from_account"`
	StatusID           *string       `json:"status_id"`
}
