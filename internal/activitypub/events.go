package activitypub

import (
	"context"
	"encoding/json"
)

// StatusEventOpts carries routing metadata for publishing status events from the inbox.
type StatusEventOpts struct {
	AccountID           string
	Visibility          string
	Local               bool
	HashtagNames        []string
	MentionedAccountIDs []string
}

// InboxEventPublisher publishes SSE events for activities processed by the inbox (remote statuses, notifications).
// Accepts pre-serialized Mastodon API JSON to avoid double-serialization in the federation path.
//
// TODO: I have a love/hate relationship with this design.
// It feels wrong to pass raw Mastodon API JSON around the system without converting it to the domain model.
// However, in this use case, we are emitting the event so that it can be consumed by the SSE Hub (and ultimately
// a Mastodon client).
//
// In the future, I think we will want to convert to the domain model to support other use cases.
type InboxEventPublisher interface {
	PublishStatusCreatedRaw(ctx context.Context, statusJSON json.RawMessage, opts StatusEventOpts)
	PublishStatusDeletedRaw(ctx context.Context, statusID string, opts StatusEventOpts)
	PublishNotificationCreatedRaw(ctx context.Context, accountID string, notifJSON json.RawMessage)
}
