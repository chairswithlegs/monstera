package apimodel

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
	Note      string  `json:"note"`
	Privacy   string  `json:"privacy"`
	Sensitive bool    `json:"sensitive"`
	Language  string  `json:"language"`
	Fields    []Field `json:"fields"`
}

// Status is the Mastodon API status response shape.
type Status struct {
	ID                 string            `json:"id"`
	CreatedAt          string            `json:"created_at"`
	InReplyToID        *string           `json:"in_reply_to_id"`
	InReplyToAccountID *string           `json:"in_reply_to_account_id"`
	Sensitive          bool              `json:"sensitive"`
	SpoilerText        string            `json:"spoiler_text"`
	Visibility         string            `json:"visibility"`
	Language           *string           `json:"language"`
	URI                string            `json:"uri"`
	URL                *string           `json:"url"`
	RepliesCount       int               `json:"replies_count"`
	ReblogsCount       int               `json:"reblogs_count"`
	FavouritesCount    int               `json:"favourites_count"`
	Content            string            `json:"content"`
	Reblog             *Status           `json:"reblog"`
	Account            Account           `json:"account"`
	MediaAttachments   []MediaAttachment `json:"media_attachments"`
	Mentions           []Mention         `json:"mentions"`
	Tags               []Tag             `json:"tags"`
	Emojis             []any             `json:"emojis"`
	Card               *any              `json:"card"`
	Poll               *any              `json:"poll"`
	Favourited         bool              `json:"favourited"`
	Reblogged          bool              `json:"reblogged"`
	Muted              bool              `json:"muted"`
	Bookmarked         bool              `json:"bookmarked"`
	Pinned             bool              `json:"pinned"`
}

// Mention is an account mention in a status.
type Mention struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Acct     string `json:"acct"`
	URL      string `json:"url"`
}

// Tag is a hashtag in a status.
type Tag struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// StatusEdit is one revision in a status's edit history (GET .../history).
type StatusEdit struct {
	Content     string `json:"content"`
	SpoilerText string `json:"spoiler_text"`
	Sensitive   bool   `json:"sensitive"`
	CreatedAt   string `json:"created_at"`
}

// StatusSource is the plain-text source for a status (GET .../source).
type StatusSource struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	SpoilerText string `json:"spoiler_text"`
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
