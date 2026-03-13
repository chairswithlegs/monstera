package uid

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	id := New()
	require.NotEmpty(t, id)
	assert.Len(t, id, 26, "ULID must be 26 characters")
}

func TestNew_lexicographicOrdering(t *testing.T) {
	t.Parallel()

	id1 := New()
	time.Sleep(2 * time.Millisecond)
	id2 := New()

	assert.Less(t, id1, id2, "later ULID should be lexicographically greater")
}

func TestNew_uniqueness(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := New()
		require.False(t, seen[id], "duplicate ULID at iteration %d", i)
		seen[id] = true
	}
}
