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
