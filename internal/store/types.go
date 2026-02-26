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

// CreateNotificationInput is the input for creating a notification.
type CreateNotificationInput struct {
	ID        string
	AccountID string
	FromID    string
	Type      string
	StatusID  *string
}

// CreateFollowInput is the input for creating a follow.
type CreateFollowInput struct {
	ID        string
	AccountID string
	TargetID  string
	State     string
	APID      *string
}

// CreateBlockInput is the input for creating a block.
type CreateBlockInput struct {
	ID        string
	AccountID string
	TargetID  string
}

// CreateFavouriteInput is the input for creating a favourite.
type CreateFavouriteInput struct {
	ID        string
	AccountID string
	StatusID  string
	APID      *string
}

// UpdateAccountInput is the input for updating an account (profile fields).
type UpdateAccountInput struct {
	ID            string
	DisplayName   *string
	Note          *string
	AvatarMediaID *string
	HeaderMediaID *string
	APRaw         []byte
	Bot           bool
	Locked        bool
}

// CreateMediaAttachmentInput is the input for creating a media attachment.
type CreateMediaAttachmentInput struct {
	ID          string
	AccountID   string
	Type        string
	StorageKey  string
	URL         string
	PreviewURL  *string
	RemoteURL   *string
	Description *string
	Blurhash    *string
	Meta        []byte
}

// CreateStatusEditInput is the input for creating a status edit.
type CreateStatusEditInput struct {
	ID             string
	StatusID       string
	AccountID      string
	Text           *string
	Content        *string
	ContentWarning *string
	Sensitive      bool
}

// UpdateStatusInput is the input for updating a status (edit fields).
type UpdateStatusInput struct {
	ID             string
	Text           *string
	Content        *string
	ContentWarning *string
	Sensitive      bool
}
