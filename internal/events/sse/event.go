// Package sse provides the event-delivery side of SSE: wire format (SSEEvent), subject/stream-key mapping, and Hub.
package sse

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// NATS subjects
	SubjectPrefixPublic      = "events.public"
	SubjectPrefixPublicLocal = "events.public.local"
	SubjectPrefixUser        = "events.user."
	SubjectPrefixHashtag     = "events.hashtag."

	// SSE stream keys
	StreamPublic        = "public"
	StreamPublicLocal   = "public:local"
	StreamUserPrefix    = "user:"
	StreamHashtagPrefix = "hashtag:"

	// SSE event types
	EventUpdate       = "update"
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
// Stream keys: "public", "public:local", "user:{accountID}", "hashtag:{tag}".
func StreamKeyToSubject(streamKey string) string {
	switch {
	case streamKey == StreamPublic:
		return SubjectPrefixPublic
	case streamKey == StreamPublicLocal:
		return SubjectPrefixPublicLocal
	case strings.HasPrefix(streamKey, StreamUserPrefix):
		return SubjectPrefixUser + strings.TrimPrefix(streamKey, StreamUserPrefix)
	case strings.HasPrefix(streamKey, StreamHashtagPrefix):
		return SubjectPrefixHashtag + strings.TrimPrefix(streamKey, StreamHashtagPrefix)
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
	case strings.HasPrefix(subject, SubjectPrefixUser):
		return StreamUserPrefix + strings.TrimPrefix(subject, SubjectPrefixUser)
	case strings.HasPrefix(subject, SubjectPrefixHashtag):
		return StreamHashtagPrefix + strings.TrimPrefix(subject, SubjectPrefixHashtag)
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
	case strings.HasPrefix(streamKey, StreamUserPrefix):
		return "user"
	case strings.HasPrefix(streamKey, StreamHashtagPrefix):
		return "hashtag"
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
