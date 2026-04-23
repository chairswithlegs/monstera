package domain

import "time"

// OAuthApplication is a registered OAuth 2.0 client (Mastodon app).
type OAuthApplication struct {
	ID           string
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURIs string
	Scopes       string
	Website      *string
	CreatedAt    time.Time
}

// OAuthAccessToken is an issued access token for API authentication.
type OAuthAccessToken struct {
	ID            string
	ApplicationID string
	AccountID     *string
	Token         string
	Scopes        string
	ExpiresAt     *time.Time
	RevokedAt     *time.Time
	CreatedAt     time.Time
}

// AuthorizedApplication is a deduplicated view of the OAuth applications that
// currently hold an active access token for a given account. It joins fields
// from oauth_applications with metadata from the latest matching token.
type AuthorizedApplication struct {
	ApplicationID string
	Name          string
	Website       *string
	RedirectURIs  string
	AppScopes     string
	TokenScopes   string
	AuthorizedAt  time.Time
}

// OAuthAuthorizationCode is a short-lived code exchanged for an access token.
type OAuthAuthorizationCode struct {
	ID                  string
	Code                string
	ApplicationID       string
	AccountID           string
	RedirectURI         string
	Scopes              string
	CodeChallenge       *string
	CodeChallengeMethod *string
	ExpiresAt           time.Time
	CreatedAt           time.Time
}
