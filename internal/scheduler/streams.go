package scheduler

import (
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/natsutil"
)

const (
	streamName       = "SCHEDULER"
	subjectPrefix    = "scheduler.tick."
	duplicatesWindow = 5 * time.Minute
)

// StreamConfigs defines the SCHEDULER stream configuration.
// Per-job consumers are created dynamically in Start, not here.
var StreamConfigs = []natsutil.StreamConfig{
	{
		Stream: jetstream.StreamConfig{
			Name:       streamName,
			Subjects:   []string{subjectPrefix + ">"},
			Retention:  jetstream.WorkQueuePolicy,
			Storage:    jetstream.FileStorage,
			MaxAge:     10 * time.Minute,
			Duplicates: duplicatesWindow,
			MaxBytes:   1 << 20,
			Discard:    jetstream.DiscardOld,
		},
	},
}
