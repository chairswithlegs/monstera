package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRequiredField(t *testing.T) {
	t.Parallel()

	t.Run("empty returns ErrUnprocessable", func(t *testing.T) {
		err := ValidateRequiredField("", "foo")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrUnprocessable)
		assert.Contains(t, err.Error(), "foo")
	})

	t.Run("non-empty returns nil", func(t *testing.T) {
		err := ValidateRequiredField("x", "foo")
		require.NoError(t, err)
	})
}

func TestValidateOneOf(t *testing.T) {
	t.Parallel()

	t.Run("value in allowed returns nil", func(t *testing.T) {
		err := ValidateOneOf("b", []string{"a", "b", "c"}, "field")
		require.NoError(t, err)
	})

	t.Run("value not in allowed returns ErrUnprocessable", func(t *testing.T) {
		err := ValidateOneOf("x", []string{"a", "b"}, "field")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrUnprocessable)
	})

	t.Run("works with int", func(t *testing.T) {
		err := ValidateOneOf(2, []int{1, 2, 3}, "n")
		require.NoError(t, err)
		err = ValidateOneOf(0, []int{1, 2, 3}, "n")
		require.Error(t, err)
	})
}

func TestValidateRFC3339(t *testing.T) {
	t.Parallel()

	t.Run("empty returns ErrUnprocessable", func(t *testing.T) {
		_, err := ValidateRFC3339("", "t")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrUnprocessable)
	})

	t.Run("invalid format returns ErrUnprocessable", func(t *testing.T) {
		_, err := ValidateRFC3339("not-a-date", "t")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrUnprocessable)
	})

	t.Run("valid RFC3339 returns time", func(t *testing.T) {
		tt, err := ValidateRFC3339("2024-01-15T12:00:00Z", "t")
		require.NoError(t, err)
		assert.True(t, tt.Equal(time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)))
	})
}

func TestValidatePositiveInt(t *testing.T) {
	t.Parallel()

	t.Run("empty returns default", func(t *testing.T) {
		n, err := ValidatePositiveInt("", "n", 5, 100)
		require.NoError(t, err)
		assert.Equal(t, 5, n)
	})

	t.Run("invalid returns ErrUnprocessable", func(t *testing.T) {
		_, err := ValidatePositiveInt("x", "n", 1, 100)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrUnprocessable)
	})

	t.Run("zero returns ErrUnprocessable", func(t *testing.T) {
		_, err := ValidatePositiveInt("0", "n", 1, 100)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrUnprocessable)
	})

	t.Run("valid returns parsed value", func(t *testing.T) {
		n, err := ValidatePositiveInt("42", "n", 1, 100)
		require.NoError(t, err)
		assert.Equal(t, 42, n)
	})

	t.Run("value above max returns max", func(t *testing.T) {
		n, err := ValidatePositiveInt("200", "n", 1, 100)
		require.NoError(t, err)
		assert.Equal(t, 100, n)
	})
}
