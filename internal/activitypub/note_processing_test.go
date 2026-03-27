package activitypub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteContentPolicy_preserves_mention_and_hashtag_classes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "preserves mention classes on anchor",
			input: `<p><a href="https://remote.example/@alice" class="u-url mention" rel="nofollow">@<span>alice</span></a></p>`,
			want:  `<p><a href="https://remote.example/@alice" class="u-url mention" rel="nofollow">@<span>alice</span></a></p>`,
		},
		{
			name:  "preserves hashtag classes on anchor",
			input: `<p><a href="https://remote.example/tags/test" class="mention hashtag" rel="tag">#<span>test</span></a></p>`,
			want:  `<p><a href="https://remote.example/tags/test" class="mention hashtag" rel="tag nofollow">#<span>test</span></a></p>`,
		},
		{
			name:  "preserves h-card class on span",
			input: `<p><span class="h-card"><a href="https://remote.example/@alice" class="u-url mention">@<span>alice</span></a></span></p>`,
			want:  `<p><span class="h-card"><a href="https://remote.example/@alice" class="u-url mention" rel="nofollow">@<span>alice</span></a></span></p>`,
		},
		{
			name:  "strips script tags",
			input: `<p>hello <script>alert('xss')</script></p>`,
			want:  `<p>hello </p>`,
		},
	}

	policy := remoteContentPolicy()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := policy.Sanitize(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
