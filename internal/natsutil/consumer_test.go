package natsutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConsumeConfig_Defaults(t *testing.T) {
	t.Parallel()
	cfg := consumeConfig{
		maxMessages: 10,
		pullExpiry:  5_000_000_000,
		label:       "test-consumer",
	}

	var applied consumeConfig
	applied.maxMessages = 10
	applied.pullExpiry = cfg.pullExpiry
	applied.label = "test-consumer"

	assert.Equal(t, 10, applied.maxMessages)
	assert.Equal(t, cfg.pullExpiry, applied.pullExpiry)
	assert.Equal(t, "test-consumer", applied.label)
}

func TestWithMaxMessages(t *testing.T) {
	t.Parallel()
	cfg := consumeConfig{}
	WithMaxMessages(42)(&cfg)
	assert.Equal(t, 42, cfg.maxMessages)
}

func TestWithLabel(t *testing.T) {
	t.Parallel()
	cfg := consumeConfig{}
	WithLabel("my-worker")(&cfg)
	assert.Equal(t, "my-worker", cfg.label)
}

func TestConsumeOpts_Compose(t *testing.T) {
	t.Parallel()
	cfg := consumeConfig{
		maxMessages: 10,
		label:       "default",
	}
	opts := []ConsumeOpt{
		WithMaxMessages(25),
		WithLabel("custom-label"),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	assert.Equal(t, 25, cfg.maxMessages)
	assert.Equal(t, "custom-label", cfg.label)
}

func TestWithMaxMessages_OverwritesPrevious(t *testing.T) {
	t.Parallel()
	cfg := consumeConfig{}
	WithMaxMessages(5)(&cfg)
	WithMaxMessages(99)(&cfg)
	assert.Equal(t, 99, cfg.maxMessages)
}

func TestWithLabel_OverwritesPrevious(t *testing.T) {
	t.Parallel()
	cfg := consumeConfig{}
	WithLabel("first")(&cfg)
	WithLabel("second")(&cfg)
	assert.Equal(t, "second", cfg.label)
}
