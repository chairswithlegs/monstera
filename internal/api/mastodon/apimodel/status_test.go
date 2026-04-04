package apimodel

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMentionFromAccount(t *testing.T) {
	t.Parallel()

	remoteDomain := "remote.example"

	tests := []struct {
		name    string
		account *domain.Account
		wantURL string
		wantAcc string
	}{
		{
			name: "local account uses instance URL",
			account: &domain.Account{
				ID:       "01LOCAL",
				Username: "alice",
				APID:     "https://local.example/ap/users/01LOCAL",
			},
			wantURL: "https://local.example/@alice",
			wantAcc: "alice",
		},
		{
			name: "remote account with ProfileURL uses ProfileURL",
			account: &domain.Account{
				ID:         "01REMOTE",
				Username:   "bob",
				Domain:     &remoteDomain,
				APID:       "https://remote.example/users/bob",
				ProfileURL: "https://remote.example/@bob",
			},
			wantURL: "https://remote.example/@bob",
			wantAcc: "bob@remote.example",
		},
		{
			name: "remote account without ProfileURL falls back to APID",
			account: &domain.Account{
				ID:       "01REMOTE2",
				Username: "carol",
				Domain:   &remoteDomain,
				APID:     "https://remote.example/users/carol",
			},
			wantURL: "https://remote.example/users/carol",
			wantAcc: "carol@remote.example",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := MentionFromAccount(tc.account, "local.example")
			assertJSONShape(t, "Mention", m, mentionFields)
			assert.Equal(t, tc.account.ID, m.ID)
			assert.Equal(t, tc.account.Username, m.Username)
			assert.Equal(t, tc.wantAcc, m.Acct)
			assert.Equal(t, tc.wantURL, m.URL)
		})
	}
}

func TestCreateStatusRequest_Sanitize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       CreateStatusRequest
		wantStatus  string
		wantSpoiler string
		wantLang    string
	}{
		{
			name:        "strips dangerous tags from status",
			input:       CreateStatusRequest{Status: `hello <script>alert('xss')</script> world`},
			wantStatus:  "hello  world",
			wantSpoiler: "",
			wantLang:    "",
		},
		{
			name:        "strips dangerous tags from spoiler_text",
			input:       CreateStatusRequest{Status: "hi", SpoilerText: `cw <script>evil()</script>`},
			wantStatus:  "hi",
			wantSpoiler: "cw ",
		},
		{
			name:  "preserves safe formatting in status",
			input: CreateStatusRequest{Status: `<p>Hello <a href="https://example.com">link</a></p>`},
			// bluemonday UGC policy preserves <p> and <a href=...> but may modify attributes; check structural preservation
			wantStatus: `<p>Hello <a href="https://example.com" rel="nofollow">link</a></p>`,
		},
		{
			name:       "strips HTML from language",
			input:      CreateStatusRequest{Status: "hi", Language: `en<script>bad</script>`},
			wantStatus: "hi",
			wantLang:   "en",
		},
		{
			name:        "plain text passes through unchanged",
			input:       CreateStatusRequest{Status: "Hello world", SpoilerText: "content warning", Language: "en"},
			wantStatus:  "Hello world",
			wantSpoiler: "content warning",
			wantLang:    "en",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := tc.input
			req.Sanitize()
			assert.Equal(t, tc.wantStatus, req.Status)
			if tc.wantSpoiler != "" || tc.input.SpoilerText != "" {
				assert.Equal(t, tc.wantSpoiler, req.SpoilerText)
			}
			if tc.wantLang != "" || tc.input.Language != "" {
				assert.Equal(t, tc.wantLang, req.Language)
			}
		})
	}
}

func TestCreateStatusRequest_Sanitize_pollOptions(t *testing.T) {
	t.Parallel()

	req := CreateStatusRequest{
		Status: "poll",
		Poll: &struct {
			Options   []string `json:"options"`
			ExpiresIn int      `json:"expires_in"`
			Multiple  bool     `json:"multiple"`
		}{
			Options: []string{`yes <script>bad()</script>`, `no <b>bold</b>`},
		},
	}
	req.Sanitize()

	require.Len(t, req.Poll.Options, 2, "poll options must not be duplicated after sanitize")
	assert.Equal(t, "yes ", req.Poll.Options[0])
	assert.Equal(t, "no <b>bold</b>", req.Poll.Options[1])
}

func TestUpdateStatusRequest_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  string
		wantErr bool
	}{
		{"valid text", "hello world", false},
		{"empty string", "", true},
		{"whitespace only", "   \t\n  ", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := UpdateStatusRequest{Status: tc.status}
			err := req.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateStatusRequest_Sanitize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		status      string
		spoiler     string
		wantStatus  string
		wantSpoiler string
	}{
		{
			name:        "strips script tags",
			status:      `update <script>alert(1)</script>`,
			spoiler:     `cw <iframe src="evil"/>`,
			wantStatus:  "update ",
			wantSpoiler: "cw ",
		},
		{
			name:        "preserves safe HTML in status",
			status:      `<p>Edited <strong>post</strong></p>`,
			wantStatus:  `<p>Edited <strong>post</strong></p>`,
			wantSpoiler: "",
		},
		{
			name:        "plain text unchanged",
			status:      "plain edit",
			spoiler:     "plain cw",
			wantStatus:  "plain edit",
			wantSpoiler: "plain cw",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := UpdateStatusRequest{Status: tc.status, SpoilerText: tc.spoiler}
			req.Sanitize()
			assert.Equal(t, tc.wantStatus, req.Status)
			assert.Equal(t, tc.wantSpoiler, req.SpoilerText)
		})
	}
}

func TestPUTInteractionPolicyRequest_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  string
		wantErr bool
		want    string
	}{
		{name: "empty policy fails", policy: "", wantErr: true},
		{name: "whitespace-only policy fails", policy: "   ", wantErr: true},
		{name: "valid policy trimmed", policy: "  public  ", wantErr: false, want: "public"},
		{name: "followers policy accepted", policy: "followers", wantErr: false, want: "followers"},
		{name: "nobody policy accepted", policy: "nobody", wantErr: false, want: "nobody"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := PUTInteractionPolicyRequest{QuoteApprovalPolicy: tc.policy}
			err := req.Validate()
			if tc.wantErr {
				assert.ErrorIs(t, err, api.ErrUnprocessable)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, req.QuoteApprovalPolicy)
			}
		})
	}
}

func TestParseCreateStatusRequest(t *testing.T) {
	t.Parallel()

	t.Run("invalid JSON returns error", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/statuses", bytes.NewBufferString(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		_, err := ParseCreateStatusRequest(req)
		assert.ErrorIs(t, err, api.ErrBadRequest)
	})

	t.Run("empty status without media or poll returns error", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/statuses", bytes.NewBufferString(`{"status":""}`))
		req.Header.Set("Content-Type", "application/json")
		_, err := ParseCreateStatusRequest(req)
		assert.ErrorIs(t, err, api.ErrUnprocessable)
	})

	t.Run("empty status with media_ids succeeds", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/statuses", bytes.NewBufferString(`{"status":"","media_ids":["abc123"]}`))
		req.Header.Set("Content-Type", "application/json")
		parsed, err := ParseCreateStatusRequest(req)
		require.NoError(t, err)
		assert.Empty(t, parsed.Status)
		assert.Equal(t, []string{"abc123"}, parsed.MediaIDs)
	})

	t.Run("empty status with poll succeeds", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/statuses", bytes.NewBufferString(`{"status":"","poll":{"options":["a","b"],"expires_in":3600}}`))
		req.Header.Set("Content-Type", "application/json")
		parsed, err := ParseCreateStatusRequest(req)
		require.NoError(t, err)
		assert.Empty(t, parsed.Status)
		require.NotNil(t, parsed.Poll)
		assert.Equal(t, []string{"a", "b"}, parsed.Poll.Options)
	})

	t.Run("valid JSON parses fields", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/statuses", bytes.NewBufferString(`{"status":"hi","visibility":"private","spoiler_text":"cw","sensitive":true,"language":"en"}`))
		req.Header.Set("Content-Type", "application/json")
		parsed, err := ParseCreateStatusRequest(req)
		require.NoError(t, err)
		assert.Equal(t, "hi", parsed.Status)
		assert.Equal(t, "private", parsed.Visibility)
		assert.Equal(t, "cw", parsed.SpoilerText)
		assert.True(t, parsed.Sensitive)
		assert.Equal(t, "en", parsed.Language)
	})
}

func TestToStatus_shape(t *testing.T) {
	t.Parallel()
	acc := &domain.Account{
		ID:        "01ACC",
		Username:  "alice",
		CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	content := "<p>Hello</p>"
	lang := "en"
	s := &domain.Status{
		ID:         "01STATUS",
		URI:        "https://example.com/statuses/01STATUS",
		AccountID:  acc.ID,
		Content:    &content,
		Visibility: "public",
		Language:   &lang,
		Local:      true,
		CreatedAt:  time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC),
	}
	author := ToAccount(acc, "example.com")
	st := ToStatus(s, author, []Mention{}, []Tag{}, []MediaAttachment{}, nil, "example.com")
	assertJSONShape(t, "Status", st, statusFields)
}

func TestStatusFromEnriched_shape(t *testing.T) {
	t.Parallel()
	acc := &domain.Account{
		ID:        "01ACC",
		Username:  "alice",
		CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	content := "<p>Hello</p>"
	s := &domain.Status{
		ID:         "01STATUS",
		URI:        "https://example.com/statuses/01STATUS",
		AccountID:  acc.ID,
		Content:    &content,
		Visibility: "public",
		Local:      true,
		CreatedAt:  time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC),
	}
	enriched := service.EnrichedStatus{
		Status:   s,
		Author:   acc,
		Mentions: []*domain.Account{},
		Tags:     []domain.Hashtag{},
		Media:    []domain.MediaAttachment{},
	}
	st := StatusFromEnriched(enriched, "example.com")
	assertJSONShape(t, "Status", st, statusFields)
}

func TestMediaFromDomain_shape(t *testing.T) {
	t.Parallel()
	m := &domain.MediaAttachment{
		ID:   "01MEDIA",
		Type: "image",
		URL:  "https://example.com/image.jpg",
	}
	out := MediaFromDomain(m)
	assertJSONShape(t, "MediaAttachment", out, mediaAttachmentFields)
}

func TestPollFromEnriched_shape(t *testing.T) {
	t.Parallel()
	expires := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	enriched := &service.EnrichedPoll{
		Poll: domain.Poll{
			ID:        "01POLL",
			Multiple:  false,
			ExpiresAt: &expires,
		},
		Options: []service.PollOptionWithCount{
			{Title: "Yes", VotesCount: 3},
			{Title: "No", VotesCount: 1},
		},
		OwnVotes: []int{},
	}
	out := PollFromEnriched(enriched)
	assertJSONShape(t, "Poll", out, pollFields)
}

func TestCardFromDomain_shape(t *testing.T) {
	t.Parallel()
	c := &domain.Card{
		URL:             "https://example.com/article",
		Title:           "Test Article",
		Description:     "An article",
		Type:            "link",
		ProcessingState: domain.CardStateFetched,
	}
	out := CardFromDomain(c)
	require.NotNil(t, out)
	assertJSONShape(t, "PreviewCard", *out, cardFields)
}

func TestToRelationship_shape(t *testing.T) {
	t.Parallel()

	t.Run("with values", func(t *testing.T) {
		t.Parallel()
		r := &domain.Relationship{
			TargetID:       "01TARGET",
			Following:      true,
			FollowedBy:     true,
			ShowingReblogs: true,
		}
		out := ToRelationship(r)
		assertJSONShape(t, "Relationship", out, relationshipFields)
	})

	t.Run("nil relationship", func(t *testing.T) {
		t.Parallel()
		out := ToRelationship(nil)
		assertJSONShape(t, "Relationship", out, relationshipFields)
	})
}

func TestToNotification_shape(t *testing.T) {
	t.Parallel()
	acc := &domain.Account{
		ID:        "01FROM",
		Username:  "bob",
		CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	n := &domain.Notification{
		ID:        "01NOTIF",
		AccountID: "01ACC",
		FromID:    acc.ID,
		Type:      "follow",
		GroupKey:  "follow-01FROM",
		CreatedAt: time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC),
	}
	out := ToNotification(n, acc, nil, "example.com")
	assertJSONShape(t, "Notification", out, notificationFields)
}

func TestTrendingLinkFromDomain(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	link := domain.TrendingLink{
		URL: "https://example.com/article",
		History: []domain.TrendingLinkHistoryDay{
			{Day: day, Uses: 42, Accounts: 10},
		},
	}
	card := TrendingLinkFromDomain(link)
	assert.Equal(t, "https://example.com/article", card.URL)
	assert.Equal(t, "link", card.Type)
	require.Len(t, card.History, 1)
	assert.Equal(t, "42", card.History[0].Uses)
	assert.Equal(t, "10", card.History[0].Accounts)
}
