// Package sse provides the event-delivery side of SSE: wire format (SSEEvent), subject/stream-key mapping, and Hub.
package sse

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// NATS subjects
	SubjectPrefixPublic           = "events.public"
	SubjectPrefixPublicLocal      = "events.public.local"
	SubjectPrefixUser             = "events.user."
	SubjectPrefixUserNotification = "events.user.notification."
	SubjectPrefixHashtag          = "events.hashtag."
	SubjectPrefixList             = "events.list."
	SubjectPrefixDirect           = "events.direct."

	// SSE stream keys
	StreamPublic                 = "public"
	StreamPublicLocal            = "public:local"
	StreamUserPrefix             = "user:"
	StreamUserNotificationPrefix = "user:notification:"
	StreamHashtagPrefix          = "hashtag:"
	StreamListPrefix             = "list:"
	StreamDirectPrefix           = "direct:"

	// SSE event types
	EventUpdate       = "update"
	EventStatusUpdate = "status.update"
	EventNotification = "notification"
	EventDelete       = "delete"
)

// SSEEvent is the wire format for NATS pub/sub. Data is pre-serialized JSON.
type SSEEvent struct {
	Stream string `json:"stream"`
	Event  string `json:"event"`
	Data   string `json:"data"`
}

// StreamKeyToSubject maps an internal stream key to the NATS subject.
// Stream keys: "public", "public:local", "user:{accountID}", "user:notification:{accountID}", "hashtag:{tag}".
//
// In all three mapping functions below, user:notification must be checked before
// user: because user: is a prefix of user:notification:.
func StreamKeyToSubject(streamKey string) string {
	switch {
	case streamKey == StreamPublic:
		return SubjectPrefixPublic
	case streamKey == StreamPublicLocal:
		return SubjectPrefixPublicLocal
	case strings.HasPrefix(streamKey, StreamUserNotificationPrefix):
		return SubjectPrefixUserNotification + strings.TrimPrefix(streamKey, StreamUserNotificationPrefix)
	case strings.HasPrefix(streamKey, StreamUserPrefix):
		return SubjectPrefixUser + strings.TrimPrefix(streamKey, StreamUserPrefix)
	case strings.HasPrefix(streamKey, StreamHashtagPrefix):
		return SubjectPrefixHashtag + strings.TrimPrefix(streamKey, StreamHashtagPrefix)
	case strings.HasPrefix(streamKey, StreamListPrefix):
		return SubjectPrefixList + strings.TrimPrefix(streamKey, StreamListPrefix)
	case strings.HasPrefix(streamKey, StreamDirectPrefix):
		return SubjectPrefixDirect + strings.TrimPrefix(streamKey, StreamDirectPrefix)
	default:
		return ""
	}
}

// SubjectToStreamKey maps a NATS subject to the internal stream key.
func SubjectToStreamKey(subject string) string {
	switch {
	case subject == SubjectPrefixPublic:
		return StreamPublic
	case subject == SubjectPrefixPublicLocal:
		return StreamPublicLocal
	case strings.HasPrefix(subject, SubjectPrefixUserNotification):
		return StreamUserNotificationPrefix + strings.TrimPrefix(subject, SubjectPrefixUserNotification)
	case strings.HasPrefix(subject, SubjectPrefixUser):
		return StreamUserPrefix + strings.TrimPrefix(subject, SubjectPrefixUser)
	case strings.HasPrefix(subject, SubjectPrefixHashtag):
		return StreamHashtagPrefix + strings.TrimPrefix(subject, SubjectPrefixHashtag)
	case strings.HasPrefix(subject, SubjectPrefixList):
		return StreamListPrefix + strings.TrimPrefix(subject, SubjectPrefixList)
	case strings.HasPrefix(subject, SubjectPrefixDirect):
		return StreamDirectPrefix + strings.TrimPrefix(subject, SubjectPrefixDirect)
	default:
		return ""
	}
}

// StreamKeyMetricLabel returns the label value for the active_sse_connections gauge.
// Uses prefix only for user and hashtag to avoid unbounded cardinality.
func StreamKeyMetricLabel(streamKey string) string {
	switch {
	case streamKey == StreamPublic:
		return StreamPublic
	case streamKey == StreamPublicLocal:
		return StreamPublicLocal
	case strings.HasPrefix(streamKey, StreamUserNotificationPrefix):
		return "user:notification"
	case strings.HasPrefix(streamKey, StreamUserPrefix):
		return "user"
	case strings.HasPrefix(streamKey, StreamHashtagPrefix):
		return "hashtag"
	case strings.HasPrefix(streamKey, StreamListPrefix):
		return "list"
	case strings.HasPrefix(streamKey, StreamDirectPrefix):
		return "direct"
	default:
		return "unknown"
	}
}

// DecodeSSEEvent deserializes a NATS message payload into SSEEvent.
func DecodeSSEEvent(data []byte) (SSEEvent, error) {
	var e SSEEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return SSEEvent{}, fmt.Errorf("sse: decode SSEEvent: %w", err)
	}
	return e, nil
}
