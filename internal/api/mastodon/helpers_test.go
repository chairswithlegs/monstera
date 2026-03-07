package mastodon

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbsoluteRequestURL(t *testing.T) {
	t.Parallel()
	const instanceDomain = "monstera.local"
	tests := []struct {
		name    string
		path    string
		wantURL string
	}{
		{
			name:    "account statuses with query",
			path:    "/api/v1/accounts/123/statuses?pinned=true",
			wantURL: "https://monstera.local/api/v1/accounts/123/statuses?pinned=true",
		},
		{
			name:    "account statuses with max_id",
			path:    "/api/v1/accounts/01KK/statuses?max_id=01KK4&pinned=true",
			wantURL: "https://monstera.local/api/v1/accounts/01KK/statuses?max_id=01KK4&pinned=true",
		},
		{
			name:    "timeline home",
			path:    "/api/v1/timeline/home",
			wantURL: "https://monstera.local/api/v1/timeline/home",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://monstera.local"+tt.path, nil)
			require.NoError(t, err)
			got := AbsoluteRequestURL(r, instanceDomain)
			assert.Equal(t, tt.wantURL, got)
		})
	}
}
