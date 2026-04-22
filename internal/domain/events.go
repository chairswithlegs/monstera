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
	EventFollowRequested     = "follow.requested"
	EventFavouriteCreated    = "favourite.created"
	EventFavouriteRemoved    = "favourite.removed"
	EventReblogCreated       = "reblog.created"
	EventReblogRemoved       = "reblog.removed"
	EventBlockCreated        = "block.created"
	EventBlockRemoved        = "block.removed"
	EventAccountUpdated      = "account.updated"
	EventAccountDeleted      = "account.deleted"
	EventStatusUpdatedRemote = "status.updated.remote"
	EventPollUpdated         = "poll.updated"
	EventPollExpired         = "poll.expired"
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

// StatusCreatedPayload carries data for a created status. Used by both
// the federation subscriber (to build Create{Note}) and the SSE subscriber.
type StatusCreatedPayload struct {
	Status              *Status           `json:"status"`
	Author              *Account          `json:"author"`
	Mentions            []*Account        `json:"mentions"`
	Tags                []Hashtag         `json:"tags"`
	Media               []MediaAttachment `json:"media"`
	MentionedAccountIDs []string          `json:"mentioned_account_ids"`
	ParentAPID          string            `json:"parent_ap_id,omitempty"`
	Local               bool              `json:"local"`
	Poll                *Poll             `json:"poll,omitempty"`
	PollOptions         []PollOption      `json:"poll_options,omitempty"`
	PollVotersCount     int               `json:"poll_voters_count,omitempty"`
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
	Status              *Status           `json:"status"`
	Author              *Account          `json:"author"`
	Mentions            []*Account        `json:"mentions"`
	Tags                []Hashtag         `json:"tags"`
	Media               []MediaAttachment `json:"media"`
	MentionedAccountIDs []string          `json:"mentioned_account_ids"`
	ParentAPID          string            `json:"parent_ap_id,omitempty"`
	Local               bool              `json:"local"`
	Poll                *Poll             `json:"poll,omitempty"`
	PollOptions         []PollOption      `json:"poll_options,omitempty"`
	PollVotersCount     int               `json:"poll_voters_count,omitempty"`
}

// FollowCreatedPayload carries data when a follow is created.
type FollowCreatedPayload struct {
	Follow *Follow  `json:"follow"`
	Actor  *Account `json:"actor"`
	Target *Account `json:"target"`
	Local  bool     `json:"local"`
}

// FollowRemovedPayload carries data when a follow is removed (unfollow).
type FollowRemovedPayload struct {
	FollowID string   `json:"follow_id"`
	Actor    *Account `json:"actor"`
	Target   *Account `json:"target"`
	Local    bool     `json:"local"`
}

// FollowAcceptedPayload carries data when a follow request is accepted.
// Target is the account that accepted; Actor is the follower.
type FollowAcceptedPayload struct {
	Follow *Follow  `json:"follow"`
	Target *Account `json:"target"`
	Actor  *Account `json:"actor"`
	Local  bool     `json:"local"`
}

// BlockCreatedPayload carries data when a user blocks another.
type BlockCreatedPayload struct {
	Actor  *Account `json:"actor"`
	Target *Account `json:"target"`
	Local  bool     `json:"local"`
}

// BlockRemovedPayload carries data when a block is removed.
type BlockRemovedPayload struct {
	Actor  *Account `json:"actor"`
	Target *Account `json:"target"`
	Local  bool     `json:"local"`
}

// FollowRequestedPayload carries data for a pending follow request.
type FollowRequestedPayload struct {
	Follow *Follow  `json:"follow"`
	Actor  *Account `json:"actor"`
	Target *Account `json:"target"`
	Local  bool     `json:"local"`
}

// FavouriteCreatedPayload carries data when a status is favourited.
type FavouriteCreatedPayload struct {
	AccountID      string   `json:"account_id"`
	StatusID       string   `json:"status_id"`
	StatusAuthorID string   `json:"status_author_id"`
	FromAccount    *Account `json:"from_account"`
	StatusAuthor   *Account `json:"status_author,omitempty"`
	StatusAPID     string   `json:"status_ap_id,omitempty"`
	Local          bool     `json:"local"`
}

// ReblogCreatedPayload carries data when a status is reblogged.
type ReblogCreatedPayload struct {
	AccountID          string   `json:"account_id"`
	ReblogStatusID     string   `json:"reblog_status_id"`
	OriginalStatusID   string   `json:"original_status_id"`
	OriginalAuthorID   string   `json:"original_author_id"`
	FromAccount        *Account `json:"from_account"`
	OriginalAuthor     *Account `json:"original_author,omitempty"`
	OriginalStatusAPID string   `json:"original_status_ap_id,omitempty"`
	Local              bool     `json:"local"`
}

// FavouriteRemovedPayload carries data when a favourite is removed (undo like).
type FavouriteRemovedPayload struct {
	AccountID      string   `json:"account_id"`
	StatusID       string   `json:"status_id"`
	StatusAuthorID string   `json:"status_author_id"`
	FromAccount    *Account `json:"from_account"`
	StatusAuthor   *Account `json:"status_author,omitempty"`
	StatusAPID     string   `json:"status_ap_id,omitempty"`
	Local          bool     `json:"local"`
}

// ReblogRemovedPayload carries data when a reblog is removed (undo announce).
type ReblogRemovedPayload struct {
	AccountID          string   `json:"account_id"`
	ReblogStatusID     string   `json:"reblog_status_id"`
	OriginalStatusID   string   `json:"original_status_id"`
	OriginalAuthorID   string   `json:"original_author_id"`
	FromAccount        *Account `json:"from_account"`
	OriginalStatusAPID string   `json:"original_status_ap_id,omitempty"`
	Local              bool     `json:"local"`
}

// AccountUpdatedPayload carries data when a user updates their profile.
type AccountUpdatedPayload struct {
	Account *Account `json:"account"`
	Local   bool     `json:"local"`
}

// AccountDeletedPayload carries data when an account has been deleted.
//
// For local deletes (Local=true), the payload references an
// account_deletion_snapshots row by DeletionID. The federation subscriber,
// fanout worker, and delivery worker all read the signing material and the
// pending follower inbox URLs from that side table — this keeps the private
// key out of the outbox_events table and off the NATS stream. APID is
// denormalized onto the payload only so the subscriber can construct the
// Delete activity without an extra DB round trip.
//
// For remote deletes (Local=false), subscribers must not federate;
// DeletionID is empty.
type AccountDeletedPayload struct {
	DeletionID string `json:"deletion_id,omitempty"`
	APID       string `json:"ap_id,omitempty"`
	Local      bool   `json:"local"`
}

// PollUpdatedPayload carries data when poll vote counts change (local vote cast).
type PollUpdatedPayload struct {
	Status          *Status           `json:"status"`
	Author          *Account          `json:"author"`
	Poll            *Poll             `json:"poll"`
	PollOptions     []PollOption      `json:"poll_options"`
	VotersCount     int               `json:"voters_count"`
	Mentions        []*Account        `json:"mentions,omitempty"`
	Tags            []Hashtag         `json:"tags,omitempty"`
	Media           []MediaAttachment `json:"media,omitempty"`
	VoterAccountID  string            `json:"voter_account_id,omitempty"`  // set when triggered by a vote; SSE skips this user's stream
	VoterAccount    *Account          `json:"voter_account,omitempty"`     // voter's account for federation delivery
	VoteOptionNames []string          `json:"vote_option_names,omitempty"` // option titles the voter selected (for remote vote delivery)
	StatusAPID      string            `json:"status_ap_id,omitempty"`      // APID of the poll status (Question IRI for remote vote delivery)
	AuthorAPID      string            `json:"author_ap_id,omitempty"`      // APID of the poll author (for remote vote delivery)
	AuthorInboxURL  string            `json:"author_inbox_url,omitempty"`  // inbox URL of the poll author (for remote vote delivery)
	Local           bool              `json:"local"`
}

// NotificationCreatedPayload carries data for a new notification (SSE-only).
type NotificationCreatedPayload struct {
	RecipientAccountID string        `json:"recipient_account_id"`
	Notification       *Notification `json:"notification"`
	FromAccount        *Account      `json:"from_account"`
	StatusID           *string       `json:"status_id"`
}
