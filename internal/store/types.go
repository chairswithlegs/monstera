package store

import (
	"encoding/json"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// CreateAccountInput is the input for creating an account.
type CreateAccountInput struct {
	ID             string
	Username       string
	Domain         *string
	DisplayName    *string
	Note           *string
	PublicKey      string
	PrivateKey     *string
	InboxURL       string
	OutboxURL      string
	FollowersURL   string
	FollowingURL   string
	APID           string
	Bot            bool
	Locked         bool
	URL            *string
	AvatarURL      string
	HeaderURL      string
	FeaturedURL    string
	FollowersCount int
	FollowingCount int
	StatusesCount  int
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
	ID                  string
	URI                 string
	AccountID           string
	Text                *string
	Content             *string
	ContentWarning      *string
	Visibility          string
	Language            *string
	InReplyToID         *string
	InReplyToAccountID  *string
	ReblogOfID          *string
	QuotedStatusID      *string
	QuoteApprovalPolicy string
	APID                string
	Sensitive           bool
	Local               bool
	CreatedAt           *time.Time // optional; defaults to NOW() in the database
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

// UpsertAccountConversationInput is the input for creating or updating an account_conversations row.
type UpsertAccountConversationInput struct {
	ID             string
	AccountID      string
	ConversationID string
	LastStatusID   string
	Unread         bool
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

// CreateFilterInput is the input for creating a keyword-based user filter.
type CreateFilterInput struct {
	ID           string
	AccountID    string
	Title        string
	Context      []string
	ExpiresAt    *time.Time
	FilterAction string
}

// UpdateFilterInput is the input for updating a keyword-based user filter.
type UpdateFilterInput struct {
	ID           string
	Title        string
	Context      []string
	ExpiresAt    *time.Time
	FilterAction string
}

// UpdateUserPreferencesInput is the input for updating a user's post preferences.
type UpdateUserPreferencesInput struct {
	UserID             string
	DefaultPrivacy     string
	DefaultSensitive   bool
	DefaultLanguage    string
	DefaultQuotePolicy string
}

// UpdateAccountInput is the input for updating an account (profile fields).
type UpdateAccountInput struct {
	ID            string
	DisplayName   *string
	Note          *string
	AvatarMediaID *string
	HeaderMediaID *string
	Bot           bool
	Locked        bool
	Fields        json.RawMessage // when not updating fields, pass current account.Fields
	URL           *string
	AvatarURL     *string
	HeaderURL     *string
}

// CreateMediaAttachmentInput is the input for creating a media attachment.
type CreateMediaAttachmentInput struct {
	ID          string
	AccountID   string
	Type        string
	ContentType *string
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

// CreateScheduledStatusInput is the input for creating a scheduled status.
type CreateScheduledStatusInput struct {
	ID          string
	AccountID   string
	Params      []byte
	ScheduledAt time.Time
}

// UpdateScheduledStatusInput is the input for updating a scheduled status (params and/or scheduled_at).
type UpdateScheduledStatusInput struct {
	ID          string
	Params      []byte
	ScheduledAt time.Time
}

// CreatePollInput is the input for creating a poll.
type CreatePollInput struct {
	ID        string
	StatusID  string
	ExpiresAt *time.Time
	Multiple  bool
}

// CreatePollOptionInput is the input for creating a poll option.
type CreatePollOptionInput struct {
	ID       string
	PollID   string
	Title    string
	Position int
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

// CreateAnnouncementInput is the input for creating an announcement.
type CreateAnnouncementInput struct {
	ID          string
	Content     string
	StartsAt    *time.Time
	EndsAt      *time.Time
	AllDay      bool
	PublishedAt time.Time
}

// UpdateAnnouncementInput is the input for updating an announcement.
type UpdateAnnouncementInput struct {
	ID          string
	Content     string
	StartsAt    *time.Time
	EndsAt      *time.Time
	AllDay      bool
	PublishedAt time.Time
}

// InsertOutboxEventInput is the input for inserting a domain event into the transactional outbox.
type InsertOutboxEventInput struct {
	ID            string
	EventType     string
	AggregateType string
	AggregateID   string
	Payload       json.RawMessage
}

// UpsertStatusCardInput holds data for upserting a status card row.
type UpsertStatusCardInput struct {
	StatusID        string
	ProcessingState string
	URL             string
	Title           string
	Description     string
	CardType        string
	ProviderName    string
	ProviderURL     string
	ImageURL        string
	Width           int
	Height          int
}

// UpdateNotificationPolicyInput is the input for updating a notification policy.
type UpdateNotificationPolicyInput struct {
	AccountID             string
	FilterNotFollowing    bool
	FilterNotFollowers    bool
	FilterNewAccounts     bool
	FilterPrivateMentions bool
}

// UpsertNotificationRequestInput is the input for upserting a notification request.
type UpsertNotificationRequestInput struct {
	ID            string
	AccountID     string
	FromAccountID string
	LastStatusID  *string
}

// CreatePushSubscriptionInput is the input for creating a push subscription.
type CreatePushSubscriptionInput struct {
	ID            string
	AccessTokenID string
	AccountID     string
	Endpoint      string
	KeyP256DH     string
	KeyAuth       string
	Alerts        domain.PushAlerts
	Policy        string
}
