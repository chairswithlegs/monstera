package store

import (
	"encoding/json"
	"time"
)

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
	ID                 string
	AccountID          string
	Email              string
	PasswordHash       string
	Role               string
	RegistrationReason *string
}

// CreateStatusInput is the input for creating a status.
type CreateStatusInput struct {
	ID                 string
	URI                string
	AccountID          string
	Text               *string
	Content            *string
	ContentWarning     *string
	Visibility         string
	Language           *string
	InReplyToID        *string
	InReplyToAccountID *string
	ReblogOfID         *string
	APID               string
	ApRaw              []byte
	Sensitive          bool
	Local              bool
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

// CreateMuteInput is the input for creating or updating a mute.
type CreateMuteInput struct {
	ID                string
	AccountID         string
	TargetID          string
	HideNotifications bool
}

// CreateFavouriteInput is the input for creating a favourite.
type CreateFavouriteInput struct {
	ID        string
	AccountID string
	StatusID  string
	APID      *string
}

// CreateBookmarkInput is the input for creating a bookmark.
type CreateBookmarkInput struct {
	ID        string
	AccountID string
	StatusID  string
}

// CreateListInput is the input for creating a list.
type CreateListInput struct {
	ID            string
	AccountID     string
	Title         string
	RepliesPolicy string
	Exclusive     bool
}

// UpdateListInput is the input for updating a list.
type UpdateListInput struct {
	ID            string
	Title         string
	RepliesPolicy string
	Exclusive     bool
}

// CreateUserFilterInput is the input for creating a user filter.
type CreateUserFilterInput struct {
	ID           string
	AccountID    string
	Phrase       string
	Context      []string
	WholeWord    bool
	ExpiresAt    *time.Time
	Irreversible bool
}

// UpdateUserFilterInput is the input for updating a user filter.
type UpdateUserFilterInput struct {
	ID           string
	Phrase       string
	Context      []string
	WholeWord    bool
	ExpiresAt    *time.Time
	Irreversible bool
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
	Fields        json.RawMessage // when not updating fields, pass current account.Fields
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

// UpdateMediaAttachmentInput is the input for updating a media attachment (description, meta).
type UpdateMediaAttachmentInput struct {
	ID          string
	AccountID   string
	Description *string
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

// CreateReportInput is the input for creating a report.
type CreateReportInput struct {
	ID        string
	AccountID string
	TargetID  string
	StatusIDs []string
	Comment   *string
	Category  string
}

// CreateDomainBlockInput is the input for creating a domain block.
type CreateDomainBlockInput struct {
	ID       string
	Domain   string
	Severity string
	Reason   *string
}

// CreateAdminActionInput is the input for creating an admin action audit log entry.
type CreateAdminActionInput struct {
	ID              string
	ModeratorID     string
	TargetAccountID *string
	Action          string
	Comment         *string
	Metadata        []byte
}

// CreateInviteInput is the input for creating an invite.
type CreateInviteInput struct {
	ID        string
	Code      string
	CreatedBy string
	MaxUses   *int
	ExpiresAt *time.Time
}

// CreateServerFilterInput is the input for creating a server filter.
type CreateServerFilterInput struct {
	ID        string
	Phrase    string
	Scope     string
	Action    string
	WholeWord bool
}

// UpdateServerFilterInput is the input for updating a server filter.
type UpdateServerFilterInput struct {
	ID        string
	Phrase    string
	Scope     string
	Action    string
	WholeWord bool
}
