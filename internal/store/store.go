package store

import (
	"context"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
)

// Store is the persistence abstraction. All methods use domain types so that
// the service layer and callers depend only on store and domain, not on any
// specific SQL implementation (e.g. postgres).
type Store interface {
	CreateAccount(ctx context.Context, in CreateAccountInput) (*domain.Account, error)
	GetAccountByID(ctx context.Context, id string) (*domain.Account, error)
	GetLocalAccountByUsername(ctx context.Context, username string) (*domain.Account, error)
	GetRemoteAccountByUsername(ctx context.Context, username string, domain *string) (*domain.Account, error)
	WithTx(ctx context.Context, fn func(Store) error) error

	CreateUser(ctx context.Context, in CreateUserInput) (*domain.User, error)

	CreateStatus(ctx context.Context, in CreateStatusInput) (*domain.Status, error)
	GetStatusByID(ctx context.Context, id string) (*domain.Status, error)
	DeleteStatus(ctx context.Context, id string) error
	IncrementStatusesCount(ctx context.Context, accountID string) error
	DecrementStatusesCount(ctx context.Context, accountID string) error

	GetHomeTimeline(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error)
	GetPublicTimeline(ctx context.Context, localOnly bool, maxID *string, limit int) ([]domain.Status, error)

	CreateApplication(ctx context.Context, in CreateApplicationInput) (*domain.OAuthApplication, error)
	GetApplicationByClientID(ctx context.Context, clientID string) (*domain.OAuthApplication, error)

	CreateAuthorizationCode(ctx context.Context, in CreateAuthorizationCodeInput) (*domain.OAuthAuthorizationCode, error)
	GetAuthorizationCode(ctx context.Context, code string) (*domain.OAuthAuthorizationCode, error)
	DeleteAuthorizationCode(ctx context.Context, code string) error

	CreateAccessToken(ctx context.Context, in CreateAccessTokenInput) (*domain.OAuthAccessToken, error)
	GetAccessToken(ctx context.Context, token string) (*domain.OAuthAccessToken, error)
	RevokeAccessToken(ctx context.Context, token string) error

	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByAccountID(ctx context.Context, accountID string) (*domain.User, error)
	ConfirmUser(ctx context.Context, userID string) error

	CreateStatusMention(ctx context.Context, statusID, accountID string) error
	GetStatusMentions(ctx context.Context, statusID string) ([]*domain.Account, error)
	GetOrCreateHashtag(ctx context.Context, name string) (*domain.Hashtag, error)
	AttachHashtagsToStatus(ctx context.Context, statusID string, hashtagIDs []string) error
	GetStatusHashtags(ctx context.Context, statusID string) ([]domain.Hashtag, error)
	CreateNotification(ctx context.Context, in CreateNotificationInput) (*domain.Notification, error)
	GetStatusAttachments(ctx context.Context, statusID string) ([]domain.MediaAttachment, error)
}
