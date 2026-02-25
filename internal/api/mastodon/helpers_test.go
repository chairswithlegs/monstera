package mastodon

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPageParamsFromRequest(t *testing.T) {
	t.Run("defaults when no query", func(t *testing.T) {
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/timelines/home", nil)
		require.NoError(t, err)
		p := PageParamsFromRequest(r)
		assert.Empty(t, p.MaxID)
		assert.Empty(t, p.MinID)
		assert.Empty(t, p.SinceID)
		assert.Equal(t, defaultPageLimit, p.Limit)
	})

	t.Run("parses max_id min_id since_id", func(t *testing.T) {
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/home?max_id=01H&min_id=01G&since_id=01F", nil)
		require.NoError(t, err)
		p := PageParamsFromRequest(r)
		assert.Equal(t, "01H", p.MaxID)
		assert.Equal(t, "01G", p.MinID)
		assert.Equal(t, "01F", p.SinceID)
		assert.Equal(t, defaultPageLimit, p.Limit)
	})

	t.Run("parses valid limit", func(t *testing.T) {
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/home?limit=10", nil)
		require.NoError(t, err)
		p := PageParamsFromRequest(r)
		assert.Equal(t, 10, p.Limit)
	})

	t.Run("clamps limit to maxPageLimit", func(t *testing.T) {
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/home?limit=100", nil)
		require.NoError(t, err)
		p := PageParamsFromRequest(r)
		assert.Equal(t, maxPageLimit, p.Limit)
	})

	t.Run("invalid limit uses default", func(t *testing.T) {
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/home?limit=abc", nil)
		require.NoError(t, err)
		p := PageParamsFromRequest(r)
		assert.Equal(t, defaultPageLimit, p.Limit)
	})

	t.Run("zero limit uses default", func(t *testing.T) {
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/home?limit=0", nil)
		require.NoError(t, err)
		p := PageParamsFromRequest(r)
		assert.Equal(t, defaultPageLimit, p.Limit)
	})

	t.Run("negative limit uses default", func(t *testing.T) {
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/home?limit=-5", nil)
		require.NoError(t, err)
		p := PageParamsFromRequest(r)
		assert.Equal(t, defaultPageLimit, p.Limit)
	})
}

func TestLinkHeader(t *testing.T) {
	t.Run("empty first and last returns empty", func(t *testing.T) {
		assert.Empty(t, LinkHeader("https://example.com/home", "", ""))
	})

	t.Run("invalid URL returns empty", func(t *testing.T) {
		assert.Empty(t, LinkHeader("://bad", "01G", "01H"))
	})

	t.Run("lastID only produces next link", func(t *testing.T) {
		out := LinkHeader("https://example.com/home", "", "01H")
		assert.Contains(t, out, "rel=\"next\"")
		assert.Contains(t, out, "max_id=01H")
		assert.NotContains(t, out, "prev")
	})

	t.Run("firstID only produces prev link", func(t *testing.T) {
		out := LinkHeader("https://example.com/home", "01G", "")
		assert.Contains(t, out, "rel=\"prev\"")
		assert.Contains(t, out, "min_id=01G")
		assert.NotContains(t, out, "next")
	})

	t.Run("both IDs produce next and prev", func(t *testing.T) {
		out := LinkHeader("https://example.com/home", "01G", "01H")
		assert.Contains(t, out, "rel=\"next\"")
		assert.Contains(t, out, "max_id=01H")
		assert.Contains(t, out, "rel=\"prev\"")
		assert.Contains(t, out, "min_id=01G")
		assert.Contains(t, out, ", ")
	})

	t.Run("preserves existing query params for next", func(t *testing.T) {
		out := LinkHeader("https://example.com/home?limit=10", "", "01H")
		assert.Contains(t, out, "max_id=01H")
		assert.Contains(t, out, "limit=10")
	})

	t.Run("prev link removes max_id and sets min_id", func(t *testing.T) {
		out := LinkHeader("https://example.com/home?max_id=01X&limit=5", "01G", "")
		assert.Contains(t, out, "min_id=01G")
		assert.Contains(t, out, "limit=5")
		assert.NotContains(t, out, "max_id=01X")
	})
}
