package store

import (
	"context"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// Store is the persistence abstraction composed of focused sub-interfaces.
// All methods use domain types so that the service layer and callers depend
// only on store and domain, not on any specific SQL implementation.
type Store interface {
	AccountStore
	UserStore
	StatusStore
	TimelineStore
	FollowStore
	InteractionStore
	ContentStore
	NotificationStore
	NotificationPolicyStore
	OAuthStore
	MediaStore
	ModerationStore
	AnnouncementStore
	InstanceStore
	ListStore
	FilterStore
	MarkerStore
	TrendingStore
	TrendingLinkFilterStore
	PollStore
	CardStore
	OutboxStore
	AccountDeletionStore
	MediaPurgeStore

	WithTx(ctx context.Context, fn func(Store) error) error
}

// AccountStore handles account persistence.
type AccountStore interface {
	CreateAccount(ctx context.Context, in CreateAccountInput) (*domain.Account, error)
	GetAccountByID(ctx context.Context, id string) (*domain.Account, error)
	// GetAccountByIDForUpdate is GetAccountByID with a row-level lock so the
	// caller's tx can safely read-then-mutate without racing concurrent writers
	// (primarily account deletion). Returns domain.ErrNotFound if the row has
	// already been deleted when the lock is acquired.
	GetAccountByIDForUpdate(ctx context.Context, id string) (*domain.Account, error)
	GetAccountsByIDs(ctx context.Context, ids []string) ([]*domain.Account, error)
	GetAccountByAPID(ctx context.Context, apID string) (*domain.Account, error)
	SearchAccounts(ctx context.Context, query string, limit, offset int) ([]*domain.Account, error)
	SearchAccountsFollowing(ctx context.Context, viewerID, query string, limit, offset int) ([]*domain.Account, error)
	GetLocalAccountByUsername(ctx context.Context, username string) (*domain.Account, error)
	GetRemoteAccountByUsername(ctx context.Context, username string, domain *string) (*domain.Account, error)
	CountLocalAccounts(ctx context.Context) (int64, error)
	CountRemoteAccounts(ctx context.Context) (int64, error)
	UpdateAccount(ctx context.Context, in UpdateAccountInput) error
	UpdateAccountKeys(ctx context.Context, id, publicKey string) error
	UpdateAccountURLs(ctx context.Context, id, inboxURL, outboxURL, followersURL, followingURL string) error
	UpdateRemoteAccountMeta(ctx context.Context, id, avatarURL, headerURL string, followersCount, followingCount, statusesCount int, featuredURL string) error
	SuspendAccount(ctx context.Context, id string) error
	UnsuspendAccount(ctx context.Context, id string) error
	SilenceAccount(ctx context.Context, id string) error
	UnsilenceAccount(ctx context.Context, id string) error
	// SetAccountsDomainSuspendedByDomain flips accounts.domain_suspended for
	// every account on the given domain. Called in the same tx as
	// CreateDomainBlock (suspended=true) and DeleteDomainBlock
	// (suspended=false) so visibility is consistent with the block row.
	// Returns the count of rows updated.
	SetAccountsDomainSuspendedByDomain(ctx context.Context, domain string, suspended bool) (int64, error)
	// DeleteAccount hard-deletes the account row and returns the deleted row.
	// Returns domain.ErrNotFound if no row matched — callers rely on this to
	// avoid emitting duplicate federation events when two concurrent deletes
	// race (Postgres serializes the DELETE via row lock, so the second caller
	// sees zero rows affected).
	DeleteAccount(ctx context.Context, id string) (*domain.Account, error)
	ListLocalAccounts(ctx context.Context, limit, offset int) ([]domain.Account, error)
	ListDirectoryAccounts(ctx context.Context, order string, localOnly bool, offset, limit int) ([]domain.Account, error)
	// ListRemoteAccountsByDomainPaginated returns the next page of remote
	// account ids on a domain, keyset-paginated by id > cursor. Pass
	// cursor="" to start at the beginning. Used by the domain-block purge
	// subscriber.
	ListRemoteAccountsByDomainPaginated(ctx context.Context, domain, cursor string, limit int) ([]string, error)
	UpdateAccountLastStatusAt(ctx context.Context, accountID string) error
	CreateAccountPin(ctx context.Context, accountID, statusID string) error
	DeleteAccountPin(ctx context.Context, accountID, statusID string) error
	DeleteAccountPinsByAccountID(ctx context.Context, accountID string) error
	ReplaceAccountPins(ctx context.Context, accountID string, statusIDs []string) error
	ListPinnedStatusIDs(ctx context.Context, accountID string) ([]string, error)
	CountAccountPins(ctx context.Context, accountID string) (int64, error)
	IncrementStatusesCount(ctx context.Context, accountID string) error
	DecrementStatusesCount(ctx context.Context, accountID string) error
	UpdateAccountLastBackfilledAt(ctx context.Context, id string, at time.Time) error
}

// UserStore handles user persistence and authentication.
type UserStore interface {
	CreateUser(ctx context.Context, in CreateUserInput) (*domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByAccountID(ctx context.Context, accountID string) (*domain.User, error)
	GetUserByID(ctx context.Context, id string) (*domain.User, error)
	ConfirmUser(ctx context.Context, userID string) error
	ListLocalUsers(ctx context.Context, limit, offset int) ([]domain.User, error)
	UpdateUserRole(ctx context.Context, userID string, role string) error
	UpdateUserDefaultQuotePolicy(ctx context.Context, accountID, policy string) error
	UpdateUserPreferences(ctx context.Context, in UpdateUserPreferencesInput) error
	UpdateUserEmail(ctx context.Context, userID, email string) error
	UpdateUserPassword(ctx context.Context, userID, passwordHash string) error
	GetPendingRegistrations(ctx context.Context) ([]domain.User, error)
	DeleteUser(ctx context.Context, id string) error
}

// StatusStore handles status persistence.
type StatusStore interface {
	CreateStatus(ctx context.Context, in CreateStatusInput) (*domain.Status, error)
	GetStatusByID(ctx context.Context, id string) (*domain.Status, error)
	GetStatusByAPID(ctx context.Context, apID string) (*domain.Status, error)
	GetAccountStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error)
	GetAccountPublicStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error)
	CountLocalStatuses(ctx context.Context) (int64, error)
	CountRemoteStatuses(ctx context.Context) (int64, error)
	CountAccountPublicStatuses(ctx context.Context, accountID string) (int64, error)
	SoftDeleteStatus(ctx context.Context, id string) error
	// DeleteStatusesByAccountIDBatched hard-deletes up to `limit` statuses
	// owned by accountID and returns the deleted ids. Intended to be called
	// in a loop until the returned slice is empty. DB-level CASCADE cleans
	// up dependent rows. Used by the domain-block purge subscriber.
	DeleteStatusesByAccountIDBatched(ctx context.Context, accountID string, limit int) ([]string, error)
	UpdateStatus(ctx context.Context, in UpdateStatusInput) error
	AttachMediaToStatus(ctx context.Context, mediaID, statusID, accountID string) error
	GetStatusAttachments(ctx context.Context, statusID string) ([]domain.MediaAttachment, error)
	CreateStatusEdit(ctx context.Context, in CreateStatusEditInput) error
	ListStatusEdits(ctx context.Context, statusID string) ([]domain.StatusEdit, error)
	CreateScheduledStatus(ctx context.Context, in CreateScheduledStatusInput) (*domain.ScheduledStatus, error)
	GetScheduledStatusByID(ctx context.Context, id string) (*domain.ScheduledStatus, error)
	ListScheduledStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.ScheduledStatus, error)
	UpdateScheduledStatus(ctx context.Context, in UpdateScheduledStatusInput) (*domain.ScheduledStatus, error)
	DeleteScheduledStatus(ctx context.Context, id string) error
	ListScheduledStatusesDue(ctx context.Context, limit int) ([]domain.ScheduledStatus, error)
}

// TimelineStore handles timeline queries.
type TimelineStore interface {
	GetHomeTimeline(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, error)
	GetFavouritesTimeline(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, *string, error)
	GetPublicTimeline(ctx context.Context, localOnly bool, maxID *string, limit int) ([]domain.Status, error)
	GetHashtagTimeline(ctx context.Context, tagName string, maxID *string, limit int) ([]domain.Status, error)
	GetStatusAncestors(ctx context.Context, statusID string) ([]domain.Status, error)
	GetStatusDescendants(ctx context.Context, statusID string) ([]domain.Status, error)
	GetStatusFavouritedBy(ctx context.Context, statusID string, maxID *string, limit int) ([]domain.Account, error)
	GetRebloggedBy(ctx context.Context, statusID string, maxID *string, limit int) ([]domain.Account, error)
}

// FollowStore handles follow relationships and follower queries.
type FollowStore interface {
	GetFollow(ctx context.Context, accountID, targetID string) (*domain.Follow, error)
	GetFollowByID(ctx context.Context, id string) (*domain.Follow, error)
	GetFollowByAPID(ctx context.Context, apID string) (*domain.Follow, error)
	CreateFollow(ctx context.Context, in CreateFollowInput) (*domain.Follow, error)
	AcceptFollow(ctx context.Context, followID string) error
	DeleteFollow(ctx context.Context, accountID, targetID string) error
	GetFollowers(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error)
	GetFollowing(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error)
	GetPendingFollowRequests(ctx context.Context, targetID string, maxID *string, limit int) ([]domain.Account, *string, error)
	CountFollowers(ctx context.Context, accountID string) (int64, error)
	CountFollowing(ctx context.Context, accountID string) (int64, error)
	IncrementFollowersCount(ctx context.Context, accountID string) error
	DecrementFollowersCount(ctx context.Context, accountID string) error
	IncrementFollowingCount(ctx context.Context, accountID string) error
	DecrementFollowingCount(ctx context.Context, accountID string) error
	GetFollowerInboxURLs(ctx context.Context, accountID string) ([]string, error)
	GetDistinctFollowerInboxURLsPaginated(ctx context.Context, accountID string, cursor string, limit int) ([]string, error)
	GetLocalFollowerAccountIDs(ctx context.Context, targetID string) ([]string, error)
	DeleteFollowsByDomain(ctx context.Context, domain string) error
	GetFamiliarFollowers(ctx context.Context, viewerID, targetID string, limit int) ([]domain.Account, error)
}

// InteractionStore handles blocks, mutes, favourites, bookmarks, reblogs, and quotes.
type InteractionStore interface {
	GetBlock(ctx context.Context, accountID, targetID string) (*domain.Block, error)
	CreateBlock(ctx context.Context, in CreateBlockInput) error
	DeleteBlock(ctx context.Context, accountID, targetID string) error
	IsBlockedEitherDirection(ctx context.Context, accountID, targetID string) (bool, error)
	ListBlockedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error)
	GetMute(ctx context.Context, accountID, targetID string) (*domain.Mute, error)
	IsMuted(ctx context.Context, accountID, targetID string) (bool, error)
	CreateMute(ctx context.Context, in CreateMuteInput) error
	DeleteMute(ctx context.Context, accountID, targetID string) error
	ListMutedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error)
	CreateUserDomainBlock(ctx context.Context, in CreateUserDomainBlockInput) error
	DeleteUserDomainBlock(ctx context.Context, accountID, domain string) error
	ListUserDomainBlocks(ctx context.Context, accountID string, maxID *string, limit int) ([]string, *string, error)
	IsUserDomainBlocked(ctx context.Context, accountID, domain string) (bool, error)
	DeleteFollowersByDomain(ctx context.Context, targetAccountID, domain string) error
	CreateFavourite(ctx context.Context, in CreateFavouriteInput) (*domain.Favourite, error)
	DeleteFavourite(ctx context.Context, accountID, statusID string) error
	GetFavouriteByAPID(ctx context.Context, apID string) (*domain.Favourite, error)
	GetFavouriteByAccountAndStatus(ctx context.Context, accountID, statusID string) (*domain.Favourite, error)
	IncrementFavouritesCount(ctx context.Context, statusID string) error
	DecrementFavouritesCount(ctx context.Context, statusID string) error
	CreateBookmark(ctx context.Context, in CreateBookmarkInput) error
	DeleteBookmark(ctx context.Context, accountID, statusID string) error
	GetBookmarks(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, *string, error)
	IsBookmarked(ctx context.Context, accountID, statusID string) (bool, error)
	IncrementReblogsCount(ctx context.Context, statusID string) error
	DecrementReblogsCount(ctx context.Context, statusID string) error
	GetReblogByAccountAndTarget(ctx context.Context, accountID, statusID string) (*domain.Status, error)
	IncrementQuotesCount(ctx context.Context, quotedStatusID string) error
	DecrementQuotesCount(ctx context.Context, quotedStatusID string) error
	IncrementRepliesCount(ctx context.Context, statusID string) error
	CreateQuoteApproval(ctx context.Context, quotingStatusID, quotedStatusID string) error
	RevokeQuote(ctx context.Context, quotedStatusID, quotingStatusID string) error
	ListQuotesOfStatus(ctx context.Context, quotedStatusID string, maxID *string, limit int) ([]domain.Status, error)
	GetQuoteApproval(ctx context.Context, quotingStatusID string) (*domain.QuoteApprovalRecord, error)
	UpdateStatusQuoteApprovalPolicy(ctx context.Context, statusID, policy string) error
}

// ContentStore handles mentions, hashtags, featured tags, and conversations.
type ContentStore interface {
	CreateStatusMention(ctx context.Context, statusID, accountID string) error
	DeleteStatusMentions(ctx context.Context, statusID string) error
	GetStatusMentions(ctx context.Context, statusID string) ([]*domain.Account, error)
	GetStatusMentionAccountIDs(ctx context.Context, statusID string) ([]string, error)
	GetOrCreateHashtag(ctx context.Context, name string) (*domain.Hashtag, error)
	GetHashtagByName(ctx context.Context, name string) (*domain.Hashtag, error)
	SearchHashtagsByPrefix(ctx context.Context, prefix string, limit int) ([]domain.Hashtag, error)
	FollowTag(ctx context.Context, id, accountID, tagID string) error
	UnfollowTag(ctx context.Context, accountID, tagID string) error
	IsFollowingTag(ctx context.Context, accountID, tagID string) (bool, error)
	ListFollowedTags(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Hashtag, *string, error)
	AreFollowingTagsByName(ctx context.Context, accountID string, tagNames []string) (map[string]bool, error)
	CreatePushSubscription(ctx context.Context, in CreatePushSubscriptionInput) (*domain.PushSubscription, error)
	GetPushSubscription(ctx context.Context, accessTokenID string) (*domain.PushSubscription, error)
	UpdatePushSubscription(ctx context.Context, accessTokenID string, alerts domain.PushAlerts, policy string) (*domain.PushSubscription, error)
	DeletePushSubscription(ctx context.Context, accessTokenID string) error
	ListPushSubscriptionsByAccountID(ctx context.Context, accountID string) ([]domain.PushSubscription, error)

	CreateFeaturedTag(ctx context.Context, id, accountID, tagID string) error
	DeleteFeaturedTag(ctx context.Context, id, accountID string) error
	ListFeaturedTags(ctx context.Context, accountID string) ([]domain.FeaturedTag, error)
	GetFeaturedTagByID(ctx context.Context, id, accountID string) (*domain.FeaturedTag, error)
	ListFeaturedTagSuggestions(ctx context.Context, accountID string, limit int) ([]domain.Hashtag, []int64, error)
	AttachHashtagsToStatus(ctx context.Context, statusID string, hashtagIDs []string) error
	DeleteStatusHashtags(ctx context.Context, statusID string) error
	GetStatusHashtags(ctx context.Context, statusID string) ([]domain.Hashtag, error)
	GetConversationRoot(ctx context.Context, statusID string) (string, error)
	CreateConversationMute(ctx context.Context, accountID, conversationID string) error
	DeleteConversationMute(ctx context.Context, accountID, conversationID string) error
	IsConversationMuted(ctx context.Context, accountID, conversationID string) (bool, error)
	CreateConversation(ctx context.Context, id string) error
	SetStatusConversationID(ctx context.Context, statusID, conversationID string) error
	GetStatusConversationID(ctx context.Context, statusID string) (*string, error)
	UpsertAccountConversation(ctx context.Context, in UpsertAccountConversationInput) error
	ListAccountConversations(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.AccountConversation, *string, error)
	GetAccountConversation(ctx context.Context, accountID, conversationID string) (*domain.AccountConversation, error)
	MarkAccountConversationRead(ctx context.Context, accountID, conversationID string) error
	DeleteAccountConversation(ctx context.Context, accountID, conversationID string) error
}

// NotificationStore handles notification persistence.
type NotificationStore interface {
	CreateNotification(ctx context.Context, in CreateNotificationInput) (*domain.Notification, error)
	ListNotifications(ctx context.Context, accountID string, maxID *string, limit int, types, excludeTypes []string) ([]domain.Notification, error)
	GetNotification(ctx context.Context, id, accountID string) (*domain.Notification, error)
	ClearNotifications(ctx context.Context, accountID string) error
	DismissNotification(ctx context.Context, id, accountID string) error
	ListGroupedNotifications(ctx context.Context, accountID string, maxID *string, limit int, types, excludeTypes []string) ([]domain.NotificationGroup, error)
	GetNotificationGroup(ctx context.Context, accountID, groupKey string) ([]domain.Notification, error)
	DismissNotificationGroup(ctx context.Context, accountID, groupKey string) error
	CountUnreadGroupedNotifications(ctx context.Context, accountID string) (int64, error)
}

// NotificationPolicyStore handles notification policy and request persistence.
type NotificationPolicyStore interface {
	UpsertNotificationPolicy(ctx context.Context, accountID string) (*domain.NotificationPolicy, error)
	GetNotificationPolicyByAccountID(ctx context.Context, accountID string) (*domain.NotificationPolicy, error)
	UpdateNotificationPolicy(ctx context.Context, in UpdateNotificationPolicyInput) (*domain.NotificationPolicy, error)
	CountPendingNotificationRequests(ctx context.Context, accountID string) (int64, error)
	CountPendingNotifications(ctx context.Context, accountID string) (int64, error)
	UpsertNotificationRequest(ctx context.Context, in UpsertNotificationRequestInput) (*domain.NotificationRequest, error)
	GetNotificationRequestByID(ctx context.Context, id, accountID string) (*domain.NotificationRequest, error)
	ListNotificationRequests(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.NotificationRequest, error)
	DeleteNotificationRequest(ctx context.Context, id, accountID string) error
	DeleteNotificationRequestsByIDs(ctx context.Context, accountID string, ids []string) error
}

// OAuthStore handles OAuth application, authorization code, and access token persistence.
type OAuthStore interface {
	CreateApplication(ctx context.Context, in CreateApplicationInput) (*domain.OAuthApplication, error)
	GetApplicationByClientID(ctx context.Context, clientID string) (*domain.OAuthApplication, error)
	GetApplicationByID(ctx context.Context, id string) (*domain.OAuthApplication, error)
	CreateAuthorizationCode(ctx context.Context, in CreateAuthorizationCodeInput) (*domain.OAuthAuthorizationCode, error)
	GetAuthorizationCode(ctx context.Context, code string) (*domain.OAuthAuthorizationCode, error)
	DeleteAuthorizationCode(ctx context.Context, code string) error
	CreateAccessToken(ctx context.Context, in CreateAccessTokenInput) (*domain.OAuthAccessToken, error)
	GetAccessToken(ctx context.Context, token string) (*domain.OAuthAccessToken, error)
	RevokeAccessToken(ctx context.Context, token string) error
	ListAuthorizedApplicationsForAccount(ctx context.Context, accountID string) ([]domain.AuthorizedApplication, error)
	// RevokeAccessTokensForAccountApp revokes every active access token for the
	// (accountID, applicationID) pair atomically and returns the raw token
	// strings of the rows it just revoked so callers can invalidate caches.
	RevokeAccessTokensForAccountApp(ctx context.Context, accountID, applicationID string) ([]string, error)
}

// MediaStore handles media attachment persistence.
type MediaStore interface {
	GetMediaAttachment(ctx context.Context, id string) (*domain.MediaAttachment, error)
	CreateMediaAttachment(ctx context.Context, in CreateMediaAttachmentInput) (*domain.MediaAttachment, error)
	UpdateMediaAttachment(ctx context.Context, in UpdateMediaAttachmentInput) (*domain.MediaAttachment, error)
}

// DomainBlockWithPurge bundles a domain block with its optional purge
// progress row. Purge is nil for silence-severity blocks (no purge flow)
// and for suspend-severity blocks that predate migration 000086.
type DomainBlockWithPurge struct {
	Block domain.DomainBlock
	Purge *domain.DomainBlockPurge
}

// ModerationStore handles reports, domain blocks, admin actions, invites, and known instances.
type ModerationStore interface {
	CreateReport(ctx context.Context, in CreateReportInput) (*domain.Report, error)
	GetReportByID(ctx context.Context, id string) (*domain.Report, error)
	ListReports(ctx context.Context, state string, limit, offset int) ([]domain.Report, error)
	AssignReport(ctx context.Context, reportID string, assigneeID *string) error
	ResolveReport(ctx context.Context, reportID string, actionTaken *string) error
	CreateDomainBlock(ctx context.Context, in CreateDomainBlockInput) (*domain.DomainBlock, error)
	DeleteDomainBlock(ctx context.Context, domain string) error
	ListDomainBlocks(ctx context.Context) ([]domain.DomainBlock, error)
	// ListDomainBlocksWithPurge returns every domain block joined with its
	// domain_block_purges row. Used by the admin list endpoint to surface
	// purge progress without an extra round trip.
	ListDomainBlocksWithPurge(ctx context.Context) ([]DomainBlockWithPurge, error)
	// CreateDomainBlockPurge inserts a tracker row for a severity=suspend
	// block. Called inside CreateDomainBlock's tx.
	CreateDomainBlockPurge(ctx context.Context, blockID, domain string) error
	// GetDomainBlockPurge returns the purge tracker for a block, or
	// ErrNotFound if the block has no purge row (silence severity or
	// pre-issue-#104).
	GetDomainBlockPurge(ctx context.Context, blockID string) (*domain.DomainBlockPurge, error)
	// UpdateDomainBlockPurgeCursor persists the last-processed account id
	// after a batch so redelivery resumes from the right place.
	UpdateDomainBlockPurgeCursor(ctx context.Context, blockID, cursor string) error
	// MarkDomainBlockPurgeComplete sets completed_at = NOW(). Idempotent.
	MarkDomainBlockPurgeComplete(ctx context.Context, blockID string) error
	// CountRemoteAccountsByDomainAfterCursor returns the number of remote
	// accounts on the domain with id > cursor. Used by the admin API to
	// compute "accounts remaining" for an in-progress purge.
	CountRemoteAccountsByDomainAfterCursor(ctx context.Context, domain, cursor string) (int64, error)
	CreateAdminAction(ctx context.Context, in CreateAdminActionInput) error
	CreateInvite(ctx context.Context, in CreateInviteInput) (*domain.Invite, error)
	GetInviteByCode(ctx context.Context, code string) (*domain.Invite, error)
	ListInvitesByCreator(ctx context.Context, createdByUserID string) ([]domain.Invite, error)
	DeleteInvite(ctx context.Context, id string) error
	IncrementInviteUses(ctx context.Context, code string) error
	UpsertKnownInstance(ctx context.Context, id, domain string) error
	ListKnownInstances(ctx context.Context, limit, offset int) ([]domain.KnownInstance, error)
	CountKnownInstances(ctx context.Context) (int64, error)
	CountReportsByState(ctx context.Context, state string) (int64, error)
}

// AnnouncementStore handles announcement persistence.
type AnnouncementStore interface {
	CreateAnnouncement(ctx context.Context, in CreateAnnouncementInput) (*domain.Announcement, error)
	UpdateAnnouncement(ctx context.Context, in UpdateAnnouncementInput) error
	GetAnnouncementByID(ctx context.Context, id string) (*domain.Announcement, error)
	ListActiveAnnouncements(ctx context.Context) ([]domain.Announcement, error)
	ListAllAnnouncements(ctx context.Context) ([]domain.Announcement, error)
	DismissAnnouncement(ctx context.Context, accountID, announcementID string) error
	ListReadAnnouncementIDs(ctx context.Context, accountID string) ([]string, error)
	AddAnnouncementReaction(ctx context.Context, announcementID, accountID, name string) error
	RemoveAnnouncementReaction(ctx context.Context, announcementID, accountID, name string) error
	ListAnnouncementReactionCounts(ctx context.Context, announcementID string) ([]domain.AnnouncementReactionCount, error)
	ListAccountAnnouncementReactionNames(ctx context.Context, announcementID, accountID string) ([]string, error)
}

// InstanceStore handles instance-level settings.
type InstanceStore interface {
	GetMonsteraSettings(ctx context.Context) (*domain.MonsteraSettings, error)
	UpdateMonsteraSettings(ctx context.Context, in *domain.MonsteraSettings) error
}

// ListStore handles list persistence.
type ListStore interface {
	CreateList(ctx context.Context, in CreateListInput) (*domain.List, error)
	GetListByID(ctx context.Context, id string) (*domain.List, error)
	ListLists(ctx context.Context, accountID string) ([]domain.List, error)
	UpdateList(ctx context.Context, in UpdateListInput) (*domain.List, error)
	DeleteList(ctx context.Context, id string) error
	ListListAccountIDs(ctx context.Context, listID string) ([]string, error)
	GetListIDsByMemberAccountID(ctx context.Context, accountID string) ([]string, error)
	GetListsByMemberAccountID(ctx context.Context, accountID string) ([]domain.List, error)
	AddAccountToList(ctx context.Context, listID, accountID string) error
	RemoveAccountFromList(ctx context.Context, listID, accountID string) error
	GetListTimeline(ctx context.Context, listID string, maxID *string, limit int) ([]domain.Status, error)
}

// FilterStore handles user filter persistence.
type FilterStore interface {
	CreateFilter(ctx context.Context, in CreateFilterInput) (*domain.UserFilter, error)
	GetFilterByID(ctx context.Context, id string) (*domain.UserFilter, error)
	ListFilters(ctx context.Context, accountID string) ([]domain.UserFilter, error)
	UpdateFilter(ctx context.Context, in UpdateFilterInput) (*domain.UserFilter, error)
	DeleteFilter(ctx context.Context, id string) error
	GetActiveFilters(ctx context.Context, accountID string) ([]domain.UserFilter, error)

	// filter keywords
	AddFilterKeyword(ctx context.Context, filterID, id, keyword string, wholeWord bool) (*domain.FilterKeyword, error)
	GetFilterKeywordByID(ctx context.Context, id string) (*domain.FilterKeyword, error)
	ListFilterKeywords(ctx context.Context, filterID string) ([]domain.FilterKeyword, error)
	UpdateFilterKeyword(ctx context.Context, id, keyword string, wholeWord bool) (*domain.FilterKeyword, error)
	DeleteFilterKeyword(ctx context.Context, id string) error

	// filter statuses
	AddFilterStatus(ctx context.Context, id, filterID, statusID string) (*domain.FilterStatus, error)
	GetFilterStatusByID(ctx context.Context, id string) (*domain.FilterStatus, error)
	ListFilterStatuses(ctx context.Context, filterID string) ([]domain.FilterStatus, error)
	DeleteFilterStatus(ctx context.Context, id string) error
}

// MarkerStore handles timeline marker persistence.
type MarkerStore interface {
	GetMarkers(ctx context.Context, accountID string, timelines []string) (map[string]domain.Marker, error)
	SetMarker(ctx context.Context, accountID, timeline, lastReadID string) error
}

// TrendingStore handles trending status, tag, and link persistence.
type TrendingStore interface {
	GetTopScoredPublicStatuses(ctx context.Context, since time.Time, limit int, localOnly bool) ([]domain.TrendingStatus, error)
	GetHashtagDailyStats(ctx context.Context, since time.Time, localOnly bool) ([]domain.HashtagDailyStats, error)
	TruncateTrendingTagHistory(ctx context.Context) error
	ReplaceTrendingStatuses(ctx context.Context, entries []domain.TrendingStatus) error
	UpsertTrendingTagHistory(ctx context.Context, entries []domain.TrendingTagHistory) error
	GetTrendingStatusIDs(ctx context.Context, limit int) ([]domain.TrendingStatus, error)
	GetTrendingTags(ctx context.Context, days int, limit int) ([]domain.TrendingTag, error)

	GetLinkDailyStats(ctx context.Context, days int, localOnly bool) ([]domain.TrendingLinkStats, error)
	UpsertTrendingLinkHistory(ctx context.Context, entries []domain.TrendingLinkStats) error
	ReplaceTrendingLinks(ctx context.Context, entries []domain.TrendingLink) error
	GetTrendingLinks(ctx context.Context, days int, limit int) ([]domain.TrendingLink, error)
}

// TrendingLinkFilterStore handles filter persistence for trending links.
type TrendingLinkFilterStore interface {
	AddTrendingLinkFilter(ctx context.Context, url string) error
	RemoveTrendingLinkFilter(ctx context.Context, url string) error
	ListTrendingLinkFilters(ctx context.Context) ([]string, error)
}

// PollStore handles poll persistence.
type PollStore interface {
	CreatePoll(ctx context.Context, in CreatePollInput) (*domain.Poll, error)
	CreatePollOption(ctx context.Context, in CreatePollOptionInput) (*domain.PollOption, error)
	GetPollByID(ctx context.Context, id string) (*domain.Poll, error)
	GetPollByStatusID(ctx context.Context, statusID string) (*domain.Poll, error)
	ListPollOptions(ctx context.Context, pollID string) ([]domain.PollOption, error)
	DeletePollVotesByAccount(ctx context.Context, pollID, accountID string) error
	CreatePollVote(ctx context.Context, id, pollID, accountID, optionID string) error
	SetPollOptionVoteCount(ctx context.Context, pollID string, position, count int) error
	ListExpiredOpenPollStatusIDs(ctx context.Context, limit int) ([]string, error)
	ClosePoll(ctx context.Context, pollID string) error
	GetVoteCountsByPoll(ctx context.Context, pollID string) (map[string]int, error)
	CountDistinctVoters(ctx context.Context, pollID string) (int, error)
	HasVotedOnPoll(ctx context.Context, pollID, accountID string) (bool, error)
	GetOwnVoteOptionIDs(ctx context.Context, pollID, accountID string) ([]string, error)
}

// CardStore handles status card (link preview) persistence.
type CardStore interface {
	UpsertStatusCard(ctx context.Context, in UpsertStatusCardInput) error
	GetStatusCard(ctx context.Context, statusID string) (*domain.Card, error)
}

// OutboxStore handles transactional outbox event persistence.
type OutboxStore interface {
	InsertOutboxEvent(ctx context.Context, in InsertOutboxEventInput) error
	GetAndLockUnpublishedOutboxEvents(ctx context.Context, limit int) ([]domain.DomainEvent, error)
	MarkOutboxEventsPublished(ctx context.Context, ids []string) error
	DeletePublishedOutboxEventsBefore(ctx context.Context, before time.Time) error
}

// AccountDeletionSnapshot is a frozen copy of the signing material and actor
// IRI of a deleted local account. It lives in the DB long enough for the
// federation subscriber and delivery worker to fan out Delete{Actor} to remote
// followers after the accounts row (and its follows) has been hard-deleted.
type AccountDeletionSnapshot struct {
	ID            string // deletion_id (ULID)
	APID          string // actor IRI
	PrivateKeyPEM string // PEM-encoded RSA private key, used only to sign the Delete{Actor}
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// CreateAccountDeletionSnapshotInput is the input for CreateAccountDeletionSnapshot.
type CreateAccountDeletionSnapshotInput struct {
	ID            string
	APID          string
	PrivateKeyPEM string
	ExpiresAt     time.Time
}

// AccountDeletionStore handles side tables that outlive a hard-deleted account
// row so the Delete{Actor} federation flow can run after CASCADE.
//
// Lifecycle:
//  1. Inside the account-delete tx (before the DELETE fires), the service
//     creates a snapshot and materializes the per-inbox targets from the
//     still-live follows rows.
//  2. The fanout worker paginates targets for a deletion_id and enqueues a
//     delivery for each inbox, marking each target delivered.
//  3. The delivery worker signs with the snapshot's private key (the accounts
//     row is gone by now).
//  4. A scheduler job purges snapshots past expires_at; CASCADE drops the
//     federation-delivery targets.
type AccountDeletionStore interface {
	CreateAccountDeletionSnapshot(ctx context.Context, in CreateAccountDeletionSnapshotInput) error
	GetAccountDeletionSnapshot(ctx context.Context, id string) (*AccountDeletionSnapshot, error)
	// InsertAccountDeletionTargetsForAccount joins follows + accounts to
	// snapshot the distinct remote follower inbox URLs for accountID, keyed by
	// deletionID. Must run BEFORE the accounts row is deleted (otherwise the
	// CASCADE on follows.target_id wipes the source rows).
	InsertAccountDeletionTargetsForAccount(ctx context.Context, deletionID, accountID string) error
	// ListPendingAccountDeletionTargets returns the next page of undelivered
	// inbox URLs for deletionID, keyset-paginated by inbox_url > cursor.
	ListPendingAccountDeletionTargets(ctx context.Context, deletionID, cursor string, limit int) ([]string, error)
	MarkAccountDeletionTargetDelivered(ctx context.Context, deletionID, inboxURL string) error
	// DeleteExpiredAccountDeletionSnapshots drops snapshots past their TTL and
	// returns the count deleted. CASCADE drops any remaining target rows.
	DeleteExpiredAccountDeletionSnapshots(ctx context.Context, before time.Time) (int64, error)
}

// MediaPurgeStore handles the shared media_purge_targets table used by both
// account deletion and domain-block suspend. purge_id is an opaque identifier
// owned by whichever flow emitted the row; there is no FK, so rows are GC'd
// by DeleteDeliveredMediaPurgeTargets after the MediaPurgeSubscriber marks
// them delivered.
type MediaPurgeStore interface {
	// InsertMediaPurgeTargetsForAccount snapshots every storage_key owned by
	// accountID into media_purge_targets, keyed by purgeID. Must run BEFORE
	// any CASCADE that would drop media_attachments for the account (e.g.
	// account deletion, or the status-hard-delete loop in domain purge).
	InsertMediaPurgeTargetsForAccount(ctx context.Context, purgeID, accountID string) error
	// ListPendingMediaPurgeTargets returns the next page of undelivered
	// storage keys for purgeID, keyset-paginated by storage_key > cursor.
	ListPendingMediaPurgeTargets(ctx context.Context, purgeID, cursor string, limit int) ([]string, error)
	MarkMediaPurgeTargetDelivered(ctx context.Context, purgeID, storageKey string) error
	// DeleteDeliveredMediaPurgeTargets sweeps rows whose blobs have been
	// deleted and are older than the cutoff. Returns count deleted.
	DeleteDeliveredMediaPurgeTargets(ctx context.Context, before time.Time) (int64, error)
}
