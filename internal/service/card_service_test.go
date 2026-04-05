package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/testutil"
)

func TestCardService_FetchAndStoreCard_noURL_plainContent(t *testing.T) {
	t.Parallel()
	fs := testutil.NewFakeStore()
	content := "<p>Just some text, no links here.</p>"
	fs.SeedStatus(&domain.Status{
		ID:        "status1",
		AccountID: "acct1",
		Content:   &content,
	})

	svc := NewCardService(fs, nil)
	err := svc.FetchAndStoreCard(context.Background(), "status1")
	require.NoError(t, err)

	card, ok := fs.StatusCards["status1"]
	require.True(t, ok, "card row should have been written")
	assert.Equal(t, domain.CardStateNoURL, card.ProcessingState)
}

// This is a bit of a weird test case that exists to accommodate the limitations of the store lookup logic.
// In certain situations, the store lookup might return a status with "link-like" content but no external URLs.
// This test case ensures that we handle this situation correctly.
func TestCardService_FetchAndStoreCard_noURL_withHTTPContent(t *testing.T) {
	t.Parallel()
	fs := testutil.NewFakeStore()
	// Status with "http" in content but no <a href> links with external URLs.
	content := "<p>No real links but mentions http in text.</p>"
	fs.SeedStatus(&domain.Status{
		ID:        "status1",
		AccountID: "acct1",
		Content:   &content,
	})

	svc := NewCardService(fs, nil)
	err := svc.FetchAndStoreCard(context.Background(), "status1")
	require.NoError(t, err)

	card, ok := fs.StatusCards["status1"]
	require.True(t, ok, "card row should have been written")
	assert.Equal(t, domain.CardStateNoURL, card.ProcessingState)
}

func TestCardService_extractFirstURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "external link",
			html:     `<p><a href="https://example.com/article">Read more</a></p>`,
			expected: "https://example.com/article",
		},
		{
			name:     "mention link skipped",
			html:     `<p><a class="mention" href="https://mastodon.social/@user">@user</a></p>`,
			expected: "",
		},
		{
			name:     "hashtag link skipped",
			html:     `<p><a class="hashtag" href="https://mastodon.social/tags/go">#go</a></p>`,
			expected: "",
		},
		{
			name:     "mention before external link",
			html:     `<p><a class="mention" href="https://mastodon.social/@user">@user</a> check <a href="https://example.com">this</a></p>`,
			expected: "https://example.com",
		},
		{
			name:     "no links",
			html:     `<p>Plain text with no anchors.</p>`,
			expected: "",
		},
		{
			name:     "http link",
			html:     `<p><a href="http://example.com">old school</a></p>`,
			expected: "http://example.com",
		},
		{
			name:     "remote mention link with rel=mention skipped",
			html:     `<p><a rel="mention" href="https://other.social/@user">@user@other.social</a></p>`,
			expected: "",
		},
		{
			name:     "remote hashtag link with rel=tag skipped",
			html:     `<p><a rel="tag" href="https://other.social/tags/go">#go</a></p>`,
			expected: "",
		},
		{
			name:     "remote mention before external link",
			html:     `<p><a rel="mention" href="https://other.social/@user">@user</a> check <a href="https://example.com">this</a></p>`,
			expected: "https://example.com",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractFirstURL(tc.html)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestCardService_FetchAndStoreCard_httpFetch(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
  <title>Fallback Title</title>
  <meta property="og:title" content="OG Title" />
  <meta property="og:description" content="OG Description" />
  <meta property="og:image" content="https://example.com/image.png" />
</head>
<body><p>Hello</p></body>
</html>`))
	}))
	defer srv.Close()

	fs := testutil.NewFakeStore()
	content := `<p>Check out <a href="` + srv.URL + `">this link</a></p>`
	fs.SeedStatus(&domain.Status{
		ID:        "status1",
		AccountID: "acct1",
		Content:   &content,
	})

	svc := NewCardService(fs, nil)
	// Use the default HTTP client for testing purposes, since the secure egress HTTP client
	// will block requests to the test server.
	svc.(*cardService).httpClient = http.DefaultClient

	err := svc.FetchAndStoreCard(context.Background(), "status1")
	require.NoError(t, err)

	card, ok := fs.StatusCards["status1"]
	require.True(t, ok)
	assert.Equal(t, domain.CardStateFetched, card.ProcessingState)
	assert.Equal(t, "OG Title", card.Title)
	assert.Equal(t, "OG Description", card.Description)
	assert.Equal(t, "https://example.com/image.png", card.ImageURL)
	assert.Equal(t, srv.URL, card.URL)
}

func TestCardService_FetchAndStoreCard_mentionOnlyStatus(t *testing.T) {
	t.Parallel()
	fs := testutil.NewFakeStore()
	// Status with only a rel="mention" anchor (remote server style) — no card URL should be found.
	content := `<p><a rel="mention" href="https://mastodon.social/@user">@user@mastodon.social</a></p>`
	fs.SeedStatus(&domain.Status{
		ID:        "status1",
		AccountID: "acct1",
		Content:   &content,
	})

	svc := NewCardService(fs, nil)
	err := svc.FetchAndStoreCard(context.Background(), "status1")
	require.NoError(t, err)

	card, ok := fs.StatusCards["status1"]
	require.True(t, ok, "card row should have been written")
	assert.Equal(t, domain.CardStateNoURL, card.ProcessingState)
}

type fakeDomainBlockChecker struct {
	suspended map[string]bool
}

func (f *fakeDomainBlockChecker) IsSuspended(_ context.Context, domain string) bool {
	return f.suspended[domain]
}

func TestCardService_FetchAndStoreCard_SuspendedDomainURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fs := testutil.NewFakeStore()

	content := `<p>Check <a href="https://suspended.example/page">this</a></p>`
	fs.SeedStatus(&domain.Status{
		ID:        "status1",
		AccountID: "acct1",
		Content:   &content,
	})

	checker := &fakeDomainBlockChecker{suspended: map[string]bool{"suspended.example": true}}
	svc := NewCardService(fs, checker)

	err := svc.FetchAndStoreCard(ctx, "status1")
	require.NoError(t, err)

	card, ok := fs.StatusCards["status1"]
	require.True(t, ok, "card row should have been written")
	assert.Equal(t, domain.CardStateFetchFailed, card.ProcessingState)
}

func TestCardService_FetchAndStoreCard_fetchFailed(t *testing.T) {
	t.Parallel()

	fs := testutil.NewFakeStore()
	content := `<p>See <a href="http://localhost:1">broken link</a></p>`
	fs.SeedStatus(&domain.Status{
		ID:        "status1",
		AccountID: "acct1",
		Content:   &content,
	})

	svc := NewCardService(fs, nil)
	err := svc.FetchAndStoreCard(context.Background(), "status1")
	require.NoError(t, err) // FetchAndStoreCard logs the OG fetch error and writes CardStateFetchFailed

	card, ok := fs.StatusCards["status1"]
	require.True(t, ok)
	assert.Equal(t, domain.CardStateFetchFailed, card.ProcessingState)
}
