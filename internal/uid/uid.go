package uid

import (
	"math/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// mu protects the monotonic entropy source for concurrent callers within this process only.
// Multiple replicas (pods) each have their own entropy; ULIDs are globally unique without cross-replica coordination.
var (
	// ULID entropy does not require crypto-grade randomness; the timestamp is the primary sort key.
	//nolint:gosec // G404: math/rand is acceptable for ULID monotonic entropy per IMPLEMENTATION 02
	entropy = ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
	mu      sync.Mutex
)

// New returns a new, time-sortable ULID string.
func New() string {
	mu.Lock()
	defer mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}

// NewWithTime returns a new ULID string with the given timestamp encoded in its time component.
// This is useful for backfilled entities that should sort by their original creation time.
func NewWithTime(t time.Time) string {
	mu.Lock()
	defer mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(t), entropy).String()
}
