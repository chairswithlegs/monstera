package service

import (
	"context"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestContentService(accounts ...*domain.Account) *statusWriteService {
	fake := testutil.NewFakeStore()
	for _, a := range accounts {
		fake.SeedAccount(a)
	}
	return &statusWriteService{
		store:          fake,
		instanceDomain: "example.com",
	}
}

func TestRenderContent_misc_characters(t *testing.T) {
	t.Parallel()
	svc := newTestContentService()
	result, err := svc.renderContent(context.Background(), "Hello, world! 😊. !@#$%^&amp;*()_+-=[]{}|\\;':\",.&lt;.&gt;?/")
	require.NoError(t, err)
	assert.Equal(t, "<p>Hello, world! 😊. !@#$%^&amp;*()_+-=[]{}|\\;':\",.&lt;.&gt;?/</p>", result.HTML)
}

func TestRenderContent_quotes_display_as_literal_characters(t *testing.T) {
	t.Parallel()
	svc := newTestContentService()
	result, err := svc.renderContent(context.Background(), `He said "hello" and 'world'.`)
	require.NoError(t, err)
	assert.Contains(t, result.HTML, `"hello"`, "double quotes should appear as literal characters")
	assert.Contains(t, result.HTML, `'world'`, "single quotes should appear as literal characters")
	assert.NotContains(t, result.HTML, "&#34;", "double quote should not be HTML entity")
	assert.NotContains(t, result.HTML, "&#39;", "single quote should not be HTML entity")
}

func TestRenderContent_hashtags_turned_into_links(t *testing.T) {
	t.Parallel()
	svc := newTestContentService()
	result, err := svc.renderContent(context.Background(), "Check out #golang and #opensource!")
	require.NoError(t, err)
	assert.Equal(t, []string{"golang", "opensource"}, result.Tags)
	want := `<p>Check out <a href="https://example.com/tags/golang" class="mention hashtag" rel="tag nofollow">#<span>golang</span></a> and <a href="https://example.com/tags/opensource" class="mention hashtag" rel="tag nofollow">#<span>opensource</span></a>!</p>`
	assert.Equal(t, want, result.HTML)
}

func TestRenderContent_mentions_turned_into_links_when_resolved(t *testing.T) {
	t.Parallel()
	alice := &domain.Account{ID: "alice-id", Username: "alice", APID: "https://example.com/users/alice"}
	svc := newTestContentService(alice)
	result, err := svc.renderContent(context.Background(), "Hey @alice, welcome!")
	require.NoError(t, err)
	require.Len(t, result.Mentions, 1)
	assert.Equal(t, "alice", result.Mentions[0].Username)
	assert.Nil(t, result.Mentions[0].Domain)
	assert.Equal(t, "alice-id", result.Mentions[0].AccountID)
	want := `<p>Hey <span class="h-card"><a href="https://example.com/users/alice" class="u-url mention" rel="nofollow">@<span>alice</span></a></span>, welcome!</p>`
	assert.Equal(t, want, result.HTML)
}

func TestRenderContent_mentions_unchanged_when_unresolved(t *testing.T) {
	t.Parallel()
	svc := newTestContentService()
	result, err := svc.renderContent(context.Background(), "Hey @nobody, hi.")
	require.NoError(t, err)
	assert.Empty(t, result.Mentions)
	assert.Equal(t, "<p>Hey @nobody, hi.</p>", result.HTML)
}

func TestCountStatusCharacters(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 13, countStatusCharacters("Hello, world!"))
	assert.Equal(t, 37, countStatusCharacters("Hello, world! https://example.com"))
}
