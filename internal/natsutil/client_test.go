package natsutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/config"
)

func TestNew_EmptyURL(t *testing.T) {
	t.Helper()
	cfg := &config.Config{NATSUrl: ""}
	_, err := New(cfg)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEmptyURL)
}

func TestNew_NilConfig(t *testing.T) {
	t.Helper()
	_, err := New(nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEmptyURL)
}
