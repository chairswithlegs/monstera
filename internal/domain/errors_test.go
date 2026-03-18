package domain

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSentinelErrors_UniqueMessages(t *testing.T) {
	t.Parallel()

	sentinels := []error{
		ErrNotFound,
		ErrConflict,
		ErrForbidden,
		ErrUnauthorized,
		ErrValidation,
		ErrRateLimited,
		ErrGone,
		ErrUnprocessable,
		ErrAccountSuspended,
	}

	seen := make(map[string]bool, len(sentinels))
	for _, err := range sentinels {
		msg := err.Error()
		assert.False(t, seen[msg], "duplicate sentinel message: %s", msg)
		seen[msg] = true
	}
}

func TestSentinelErrors_WrappedWithIs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sentinel error
	}{
		{"ErrNotFound", ErrNotFound},
		{"ErrConflict", ErrConflict},
		{"ErrForbidden", ErrForbidden},
		{"ErrUnauthorized", ErrUnauthorized},
		{"ErrValidation", ErrValidation},
		{"ErrRateLimited", ErrRateLimited},
		{"ErrGone", ErrGone},
		{"ErrUnprocessable", ErrUnprocessable},
		{"ErrAccountSuspended", ErrAccountSuspended},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wrapped := fmt.Errorf("context: %w", tt.sentinel)
			require.ErrorIs(t, wrapped, tt.sentinel)
			assert.NotEqual(t, wrapped, tt.sentinel, "wrapped error should differ from sentinel")
		})
	}
}

func TestSentinelErrors_NotMatchOtherSentinels(t *testing.T) {
	t.Parallel()

	require.NotErrorIs(t, ErrNotFound, ErrConflict)
	require.NotErrorIs(t, ErrForbidden, ErrUnauthorized)
	require.NotErrorIs(t, ErrValidation, ErrUnprocessable)
}
