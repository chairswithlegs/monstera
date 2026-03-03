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
type InboxEventPublisher interface {
	PublishStatusCreatedRaw(ctx context.Context, statusJSON json.RawMessage, opts StatusEventOpts)
	PublishStatusDeletedRaw(ctx context.Context, statusID string, opts StatusEventOpts)
	PublishNotificationCreatedRaw(ctx context.Context, accountID string, notifJSON json.RawMessage)
}
