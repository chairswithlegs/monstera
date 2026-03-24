package apimodel

import "encoding/json"

// Account is the Mastodon API account response shape.
type Account struct {
	ID             string  `json:"id"`
	Username       string  `json:"username"`
	Acct           string  `json:"acct"`
	DisplayName    string  `json:"display_name"`
	Locked         bool    `json:"locked"`
	Bot            bool    `json:"bot"`
	CreatedAt      string  `json:"created_at"`
	Note           string  `json:"note"`
	URL            string  `json:"url"`
	Avatar         string  `json:"avatar"`
	AvatarStatic   string  `json:"avatar_static"`
	Header         string  `json:"header"`
	HeaderStatic   string  `json:"header_static"`
	FollowersCount int     `json:"followers_count"`
	FollowingCount int     `json:"following_count"`
	StatusesCount  int     `json:"statuses_count"`
	LastStatusAt   *string `json:"last_status_at"`
	Emojis         []any   `json:"emojis"`
	Fields         []Field `json:"fields"`
	Source         *Source `json:"source,omitempty"`
}

// Field is a profile metadata field.
type Field struct {
	Name       string  `json:"name"`
	Value      string  `json:"value"`
	VerifiedAt *string `json:"verified_at"`
}

// Source is the raw account preferences (only for verify_credentials).
type Source struct {
	Note        string  `json:"note"`
	Privacy     string  `json:"privacy"`
	Sensitive   bool    `json:"sensitive"`
	Language    string  `json:"language"`
	QuotePolicy string  `json:"quote_policy,omitempty"`
	Fields      []Field `json:"fields"`
}

// Status is the Mastodon API status response shape.
type Status struct {
	ID                  string            `json:"id"`
	CreatedAt           string            `json:"created_at"`
	InReplyToID         *string           `json:"in_reply_to_id"`
	InReplyToAccountID  *string           `json:"in_reply_to_account_id"`
	Sensitive           bool              `json:"sensitive"`
	SpoilerText         string            `json:"spoiler_text"`
	Visibility          string            `json:"visibility"`
	Language            *string           `json:"language"`
	URI                 string            `json:"uri"`
	URL                 *string           `json:"url"`
	RepliesCount        int               `json:"replies_count"`
	ReblogsCount        int               `json:"reblogs_count"`
	FavouritesCount     int               `json:"favourites_count"`
	QuotesCount         int               `json:"quotes_count"`
	QuoteApprovalPolicy string            `json:"quote_approval_policy,omitempty"` // public | followers | nobody (Mastodon-style quotes)
	Content             string            `json:"content"`
	Reblog              *Status           `json:"reblog"`
	QuoteApproval       *QuoteApproval    `json:"quote_approval,omitempty"`
	Account             Account           `json:"account"`
	MediaAttachments    []MediaAttachment `json:"media_attachments"`
	Mentions            []Mention         `json:"mentions"`
	Tags                []Tag             `json:"tags"`
	Emojis              []any             `json:"emojis"`
	Card                *Card             `json:"card"`
	Poll                *Poll             `json:"poll"`
	Favourited          bool              `json:"favourited"`
	Reblogged           bool              `json:"reblogged"`
	Muted               bool              `json:"muted"`
	Bookmarked          bool              `json:"bookmarked"`
	Pinned              bool              `json:"pinned"`
}

// QuoteApproval is the quote approval state and optional quoted status (Mastodon API entity).
type QuoteApproval struct {
	State        string  `json:"state"` // "accepted" | "revoked" | "pending" | ...
	QuotedStatus *Status `json:"quoted_status,omitempty"`
}

// Mention is an account mention in a status.
type Mention struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Acct     string `json:"acct"`
	URL      string `json:"url"`
}

// Tag is a hashtag in a status or in followed/featured tag lists.
type Tag struct {
	ID        string       `json:"id,omitempty"`
	Name      string       `json:"name"`
	URL       string       `json:"url"`
	Following bool         `json:"following"`
	History   []TagHistory `json:"history"`
}

// TagHistory is one day of usage statistics for a trending hashtag.
type TagHistory struct {
	Day      string `json:"day"` // Unix timestamp as string
	Uses     string `json:"uses"`
	Accounts string `json:"accounts"`
}

// FeaturedTag is a hashtag featured on an account profile.
type FeaturedTag struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	URL           string  `json:"url"`
	StatusesCount int     `json:"statuses_count"`
	LastStatusAt  *string `json:"last_status_at"`
}

// Card is the Mastodon API preview card entity (link preview).
type Card struct {
	URL          string  `json:"url"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	Type         string  `json:"type"`
	AuthorName   string  `json:"author_name"`
	AuthorURL    string  `json:"author_url"`
	ProviderName string  `json:"provider_name"`
	ProviderURL  string  `json:"provider_url"`
	HTML         string  `json:"html"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	Image        *string `json:"image"`
	Blurhash     *string `json:"blurhash"`
	PublishedAt  *string `json:"published_at"`
}

// Poll is the Mastodon API poll entity (GET /api/v1/polls/:id, or embedded in Status).
type Poll struct {
	ID          string       `json:"id"`
	ExpiresAt   *string      `json:"expires_at"`
	Expired     bool         `json:"expired"`
	Multiple    bool         `json:"multiple"`
	VotesCount  int          `json:"votes_count"`
	VotersCount *int         `json:"voters_count"`
	Voted       bool         `json:"voted"`
	OwnVotes    []int        `json:"own_votes"`
	Options     []PollOption `json:"options"`
	Emojis      []any        `json:"emojis"`
}

// PollOption is one option in a Poll.
type PollOption struct {
	Title      string `json:"title"`
	VotesCount int    `json:"votes_count"`
}

// StatusEdit is one revision in a status's edit history (GET .../history).
type StatusEdit struct {
	Content          string            `json:"content"`
	SpoilerText      string            `json:"spoiler_text"`
	Sensitive        bool              `json:"sensitive"`
	CreatedAt        string            `json:"created_at"`
	Account          Account           `json:"account"`
	MediaAttachments []MediaAttachment `json:"media_attachments"`
	Emojis           []any             `json:"emojis"`
}

// StatusSource is the plain-text source for a status (GET .../source).
type StatusSource struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	SpoilerText string `json:"spoiler_text"`
}

// ScheduledStatus is the Mastodon API scheduled status response (GET/POST/PUT scheduled_statuses).
type ScheduledStatus struct {
	ID               string            `json:"id"`
	ScheduledAt      string            `json:"scheduled_at"`
	Params           json.RawMessage   `json:"params"`
	MediaAttachments []MediaAttachment `json:"media_attachments"`
}

// MediaAttachment is the Mastodon API media attachment shape.
type MediaAttachment struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	PreviewURL  string `json:"preview_url"`
	RemoteURL   string `json:"remote_url,omitempty"`
	Description string `json:"description"`
	Blurhash    string `json:"blurhash,omitempty"`
}

// Announcement is the Mastodon API announcement entity (GET /api/v1/announcements).
type Announcement struct {
	ID          string       `json:"id"`
	Content     string       `json:"content"`
	StartsAt    *string      `json:"starts_at"`
	EndsAt      *string      `json:"ends_at"`
	AllDay      bool         `json:"all_day"`
	PublishedAt string       `json:"published_at"`
	UpdatedAt   string       `json:"updated_at"`
	Read        bool         `json:"read"`
	Mentions    []AccountRef `json:"mentions"`
	Statuses    []StatusRef  `json:"statuses"`
	Tags        []Tag        `json:"tags"`
	Emojis      []any        `json:"emojis"`
	Reactions   []Reaction   `json:"reactions"`
}

// AccountRef is a minimal account reference in an announcement (mentions).
type AccountRef struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	URL      string `json:"url"`
	Acct     string `json:"acct"`
}

// StatusRef is a minimal status reference in an announcement.
type StatusRef struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// Reaction is one emoji reaction on an announcement.
type Reaction struct {
	Name      string `json:"name"`
	Count     int    `json:"count"`
	Me        bool   `json:"me"`
	URL       string `json:"url,omitempty"`
	StaticURL string `json:"static_url,omitempty"`
}

// Conversation is the Mastodon API conversation entity (GET /api/v1/conversations).
type Conversation struct {
	ID         string    `json:"id"`
	Unread     bool      `json:"unread"`
	Accounts   []Account `json:"accounts"`
	LastStatus *Status   `json:"last_status"`
}
