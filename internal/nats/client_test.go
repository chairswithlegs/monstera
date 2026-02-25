package nats

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/config"
)

func TestNew_EmptyURL(t *testing.T) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{NATSUrl: ""}
	_, err := New(cfg, logger)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEmptyURL)
}

func TestNew_NilConfig(t *testing.T) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	_, err := New(nil, logger)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEmptyURL)
}
