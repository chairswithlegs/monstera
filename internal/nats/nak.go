package nats

import (
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// NAKWithBackoff NAKs the message with a delay from backoff based on meta.NumDelivered.
// If meta is nil, the first backoff duration is used. backoff must be non-empty.
func NAKWithBackoff(msg jetstream.Msg, meta *jetstream.MsgMetadata, backoff []time.Duration) {
	numDelivered := uint64(0)
	if meta != nil {
		numDelivered = meta.NumDelivered
	}
	_ = msg.NakWithDelay(nakBackoffDelay(numDelivered, backoff))
}

// NAKBackoffDelay returns the NAK delay for the given delivery count (0 = first attempt).
// backoff must be non-empty; index is min(numDelivered, len(backoff)) with 0-based indexing
// for numDelivered 1..len(backoff), and the last element is used for numDelivered > len(backoff).
func nakBackoffDelay(numDelivered uint64, backoff []time.Duration) time.Duration {
	if len(backoff) == 0 {
		return 0
	}
	if numDelivered == 0 {
		return backoff[0]
	}
	idx := len(backoff) - 1
	if numDelivered <= uint64(len(backoff)) {
		idx = int(numDelivered - 1) //nolint:gosec // G115: bounded by len(backoff), small in practice
	}
	return backoff[idx]
}
