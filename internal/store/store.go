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
	GetAccountByAPID(ctx context.Context, apID string) (*domain.Account, error)
	SearchAccounts(ctx context.Context, query string, limit int) ([]*domain.Account, error)
	GetLocalAccountByUsername(ctx context.Context, username string) (*domain.Account, error)
	GetRemoteAccountByUsername(ctx context.Context, username string, domain *string) (*domain.Account, error)
	CountLocalAccounts(ctx context.Context) (int64, error)
	WithTx(ctx context.Context, fn func(Store) error) error

	CreateUser(ctx context.Context, in CreateUserInput) (*domain.User, error)

	CreateStatus(ctx context.Context, in CreateStatusInput) (*domain.Status, error)
	GetStatusByID(ctx context.Context, id string) (*domain.Status, error)
	GetStatusByAPID(ctx context.Context, apID string) (*domain.Status, error)
	GetAccountStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error)
	GetAccountPublicStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error)
	CountLocalStatuses(ctx context.Context) (int64, error)
	CountAccountPublicStatuses(ctx context.Context, accountID string) (int64, error)
	DeleteStatus(ctx context.Context, id string) error
	IncrementStatusesCount(ctx context.Context, accountID string) error
	DecrementStatusesCount(ctx context.Context, accountID string) error

	GetHomeTimeline(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error)
	GetPublicTimeline(ctx context.Context, localOnly bool, maxID *string, limit int) ([]domain.Status, error)
	GetHashtagTimeline(ctx context.Context, tagName string, maxID *string, limit int) ([]domain.Status, error)
	GetStatusAncestors(ctx context.Context, statusID string) ([]domain.Status, error)
	GetStatusDescendants(ctx context.Context, statusID string) ([]domain.Status, error)
	GetStatusFavouritedBy(ctx context.Context, statusID string, maxID *string, limit int) ([]domain.Account, error)
	GetRebloggedBy(ctx context.Context, statusID string, maxID *string, limit int) ([]domain.Account, error)

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
	SearchHashtagsByPrefix(ctx context.Context, prefix string, limit int) ([]domain.Hashtag, error)
	AttachHashtagsToStatus(ctx context.Context, statusID string, hashtagIDs []string) error
	GetStatusHashtags(ctx context.Context, statusID string) ([]domain.Hashtag, error)
	CreateNotification(ctx context.Context, in CreateNotificationInput) (*domain.Notification, error)
	ListNotifications(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Notification, error)
	GetNotification(ctx context.Context, id, accountID string) (*domain.Notification, error)
	ClearNotifications(ctx context.Context, accountID string) error
	DismissNotification(ctx context.Context, id, accountID string) error
	GetStatusAttachments(ctx context.Context, statusID string) ([]domain.MediaAttachment, error)

	GetSetting(ctx context.Context, key string) (string, error)
	GetMediaAttachment(ctx context.Context, id string) (*domain.MediaAttachment, error)
	CountFollowers(ctx context.Context, accountID string) (int64, error)
	CountFollowing(ctx context.Context, accountID string) (int64, error)
	IncrementFollowersCount(ctx context.Context, accountID string) error
	DecrementFollowersCount(ctx context.Context, accountID string) error
	IncrementFollowingCount(ctx context.Context, accountID string) error
	DecrementFollowingCount(ctx context.Context, accountID string) error

	ListDomainBlocks(ctx context.Context) ([]domain.DomainBlock, error)
	GetRelationship(ctx context.Context, accountID, targetID string) (*domain.Relationship, error)

	GetFollow(ctx context.Context, accountID, targetID string) (*domain.Follow, error)
	GetFollowByAPID(ctx context.Context, apID string) (*domain.Follow, error)
	CreateFollow(ctx context.Context, in CreateFollowInput) (*domain.Follow, error)
	AcceptFollow(ctx context.Context, followID string) error
	DeleteFollow(ctx context.Context, accountID, targetID string) error
	GetFollowers(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error)
	GetFollowing(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error)

	SoftDeleteStatus(ctx context.Context, id string) error
	SuspendAccount(ctx context.Context, id string) error

	CreateBlock(ctx context.Context, in CreateBlockInput) error
	DeleteBlock(ctx context.Context, accountID, targetID string) error
	CreateMute(ctx context.Context, in CreateMuteInput) error
	DeleteMute(ctx context.Context, accountID, targetID string) error
	CreateFavourite(ctx context.Context, in CreateFavouriteInput) (*domain.Favourite, error)
	DeleteFavourite(ctx context.Context, accountID, statusID string) error
	GetFavouriteByAPID(ctx context.Context, apID string) (*domain.Favourite, error)
	GetFavouriteByAccountAndStatus(ctx context.Context, accountID, statusID string) (*domain.Favourite, error)
	IncrementFavouritesCount(ctx context.Context, statusID string) error
	DecrementFavouritesCount(ctx context.Context, statusID string) error
	IncrementReblogsCount(ctx context.Context, statusID string) error
	DecrementReblogsCount(ctx context.Context, statusID string) error
	IncrementRepliesCount(ctx context.Context, statusID string) error
	GetReblogByAccountAndTarget(ctx context.Context, accountID, statusID string) (*domain.Status, error)

	UpdateAccount(ctx context.Context, in UpdateAccountInput) error
	UpdateAccountKeys(ctx context.Context, id, publicKey string, apRaw []byte) error
	AttachMediaToStatus(ctx context.Context, mediaID, statusID, accountID string) error
	CreateMediaAttachment(ctx context.Context, in CreateMediaAttachmentInput) (*domain.MediaAttachment, error)
	CreateStatusEdit(ctx context.Context, in CreateStatusEditInput) error
	UpdateStatus(ctx context.Context, in UpdateStatusInput) error

	GetFollowerInboxURLs(ctx context.Context, accountID string) ([]string, error)
	GetDistinctFollowerInboxURLsPaginated(ctx context.Context, accountID string, cursor string, limit int) ([]string, error)
}
