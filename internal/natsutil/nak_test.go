package natsutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNAKBackoffDelay(t *testing.T) {
	t.Parallel()
	backoff := []time.Duration{30 * time.Second, 5 * time.Minute, 30 * time.Minute}
	assert.Equal(t, 30*time.Second, nakBackoffDelay(0, backoff))
	assert.Equal(t, 30*time.Second, nakBackoffDelay(1, backoff))
	assert.Equal(t, 5*time.Minute, nakBackoffDelay(2, backoff))
	assert.Equal(t, 30*time.Minute, nakBackoffDelay(3, backoff))
	assert.Equal(t, 30*time.Minute, nakBackoffDelay(4, backoff))
	assert.Equal(t, 30*time.Minute, nakBackoffDelay(100, backoff))
}

func TestNAKBackoffDelay_EmptyBackoff(t *testing.T) {
	t.Parallel()
	assert.Equal(t, time.Duration(0), nakBackoffDelay(0, nil))
	assert.Equal(t, time.Duration(0), nakBackoffDelay(1, []time.Duration{}))
}
