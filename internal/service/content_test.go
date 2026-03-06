package service

import (
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_misc_characters(t *testing.T) {
	t.Parallel()
	var resolve MentionResolver = func(string, *string) *domain.Account { return nil }

	result, err := Render("Hello, world! 😊. !@#$%^&amp;*()_+-=[]{}|\\;':\",.&lt;.&gt;?/", "example.com", resolve)
	require.NoError(t, err)
	assert.Equal(t, "<p>Hello, world! 😊. !@#$%^&amp;*()_+-=[]{}|\\;':\",.&lt;.&gt;?/</p>", result.HTML)
}

func TestRender_quotes_display_as_literal_characters(t *testing.T) {
	t.Parallel()
	var resolve MentionResolver = func(string, *string) *domain.Account { return nil }

	result, err := Render(`He said "hello" and 'world'.`, "example.com", resolve)
	require.NoError(t, err)
	assert.Contains(t, result.HTML, `"hello"`, "double quotes should appear as literal characters")
	assert.Contains(t, result.HTML, `'world'`, "single quotes should appear as literal characters")
	assert.NotContains(t, result.HTML, "&#34;", "double quote should not be HTML entity")
	assert.NotContains(t, result.HTML, "&#39;", "single quote should not be HTML entity")
}

func TestRender_hashtags_turned_into_links(t *testing.T) {
	t.Parallel()
	var resolve MentionResolver = func(string, *string) *domain.Account { return nil }

	result, err := Render("Check out #golang and #opensource!", "example.com", resolve)
	require.NoError(t, err)
	assert.Equal(t, []string{"golang", "opensource"}, result.Tags)
	want := `<p>Check out <a href="https://example.com/tags/golang" class="mention hashtag" rel="tag nofollow">#<span>golang</span></a> and <a href="https://example.com/tags/opensource" class="mention hashtag" rel="tag nofollow">#<span>opensource</span></a>!</p>`
	assert.Equal(t, want, result.HTML)
}

func TestRender_mentions_turned_into_links_when_resolved(t *testing.T) {
	t.Parallel()
	alice := &domain.Account{ID: "alice-id", Username: "alice", APID: "https://example.com/users/alice"}
	var resolve MentionResolver = func(username string, domain *string) *domain.Account {
		if username == "alice" && domain == nil {
			return alice
		}
		return nil
	}

	result, err := Render("Hey @alice, welcome!", "example.com", resolve)
	require.NoError(t, err)
	require.Len(t, result.Mentions, 1)
	assert.Equal(t, "alice", result.Mentions[0].Username)
	assert.Nil(t, result.Mentions[0].Domain)
	assert.Equal(t, "alice-id", result.Mentions[0].AccountID)
	want := `<p>Hey <span class="h-card"><a href="https://example.com/users/alice" class="u-url mention" rel="nofollow">@<span>alice</span></a></span>, welcome!</p>`
	assert.Equal(t, want, result.HTML)
}

func TestRender_mentions_unchanged_when_unresolved(t *testing.T) {
	t.Parallel()
	var resolve MentionResolver = func(string, *string) *domain.Account { return nil }

	result, err := Render("Hey @nobody, hi.", "example.com", resolve)
	require.NoError(t, err)
	assert.Empty(t, result.Mentions)
	assert.Equal(t, "<p>Hey @nobody, hi.</p>", result.HTML)
}

func TestCountStatusCharacters(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 13, CountStatusCharacters("Hello, world!"))
	assert.Equal(t, 37, CountStatusCharacters("Hello, world! https://example.com"))
}
