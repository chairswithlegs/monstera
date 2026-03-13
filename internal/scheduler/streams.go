package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const (
	streamName       = "SCHEDULER"
	subjectPrefix    = "scheduler.tick."
	duplicatesWindow = 5 * time.Minute
)

// CreateOrUpdateStreams creates or updates the SCHEDULER stream.
// Per-job consumers are created in Start, not here.
func CreateOrUpdateStreams(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       streamName,
		Subjects:   []string{subjectPrefix + ">"},
		Retention:  jetstream.WorkQueuePolicy,
		Storage:    jetstream.FileStorage,
		MaxAge:     10 * time.Minute,
		Duplicates: duplicatesWindow,
		MaxBytes:   1 << 20,
		Discard:    jetstream.DiscardOld,
	})
	if err != nil {
		return fmt.Errorf("nats: create stream %s: %w", streamName, err)
	}
	return nil
}
