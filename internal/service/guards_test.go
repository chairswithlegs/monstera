package service

import (
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireLocal(t *testing.T) {
	t.Parallel()

	t.Run("passes when local is true", func(t *testing.T) {
		t.Parallel()
		err := requireLocal(true, "TestMethod")
		assert.NoError(t, err)
	})

	t.Run("returns ErrForbidden when local is false", func(t *testing.T) {
		t.Parallel()
		err := requireLocal(false, "TestMethod")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("includes method name in error", func(t *testing.T) {
		t.Parallel()
		err := requireLocal(false, "MyMethod")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MyMethod")
	})
}

func TestRequireRemote(t *testing.T) {
	t.Parallel()

	t.Run("passes when local is false", func(t *testing.T) {
		t.Parallel()
		err := requireRemote(false, "TestMethod")
		assert.NoError(t, err)
	})

	t.Run("returns ErrForbidden when local is true", func(t *testing.T) {
		t.Parallel()
		err := requireRemote(true, "TestMethod")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrForbidden)
	})

	t.Run("includes method name in error", func(t *testing.T) {
		t.Parallel()
		err := requireRemote(true, "MyMethod")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MyMethod")
	})
}
