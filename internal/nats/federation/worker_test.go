package federation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNakBackoffDelay(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 30*time.Second, nakBackoffDelay(0))
	assert.Equal(t, 30*time.Second, nakBackoffDelay(1))
	assert.Equal(t, 5*time.Minute, nakBackoffDelay(2))
	assert.Equal(t, 30*time.Minute, nakBackoffDelay(3))
	assert.Equal(t, 30*time.Minute, nakBackoffDelay(4))
	assert.Equal(t, 30*time.Minute, nakBackoffDelay(100))
}
