package store

import "time"

// CreateAccountInput is the input for creating an account.
type CreateAccountInput struct {
	ID           string
	Username     string
	Domain       *string
	DisplayName  *string
	Note         *string
	PublicKey    string
	PrivateKey   *string
	InboxURL     string
	OutboxURL    string
	FollowersURL string
	FollowingURL string
	APID         string
	ApRaw        []byte
	Bot          bool
	Locked       bool
}

// CreateUserInput is the input for creating a user.
type CreateUserInput struct {
	ID           string
	AccountID    string
	Email        string
	PasswordHash string
	Role         string
}

// CreateStatusInput is the input for creating a status.
type CreateStatusInput struct {
	ID             string
	URI            string
	AccountID      string
	Text           *string
	Content        *string
	ContentWarning *string
	Visibility     string
	Language       *string
	InReplyToID    *string
	ReblogOfID     *string
	APID           string
	ApRaw          []byte
	Sensitive      bool
	Local          bool
}

// CreateApplicationInput is the input for creating an OAuth application.
type CreateApplicationInput struct {
	ID           string
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURIs string
	Scopes       string
	Website      *string
}

// CreateAuthorizationCodeInput is the input for creating an OAuth authorization code.
type CreateAuthorizationCodeInput struct {
	ID                  string
	Code                string
	ApplicationID       string
	AccountID           string
	RedirectURI         string
	Scopes              string
	CodeChallenge       *string
	CodeChallengeMethod *string
	ExpiresAt           time.Time
}

// CreateAccessTokenInput is the input for creating an OAuth access token.
type CreateAccessTokenInput struct {
	ID            string
	ApplicationID string
	AccountID     *string
	Token         string
	Scopes        string
	ExpiresAt     *time.Time
}
