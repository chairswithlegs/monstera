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
	OAuthStore
	MediaStore
	ModerationStore
	AnnouncementStore
	InstanceStore
	ListStore
	FilterStore
	MarkerStore
	TrendingStore
	PollStore
	CardStore
	OutboxStore

	WithTx(ctx context.Context, fn func(Store) error) error
}

// AccountStore handles account persistence.
type AccountStore interface {
	CreateAccount(ctx context.Context, in CreateAccountInput) (*domain.Account, error)
	GetAccountByID(ctx context.Context, id string) (*domain.Account, error)
	GetAccountsByIDs(ctx context.Context, ids []string) ([]*domain.Account, error)
	GetAccountByAPID(ctx context.Context, apID string) (*domain.Account, error)
	SearchAccounts(ctx context.Context, query string, limit int) ([]*domain.Account, error)
	GetLocalAccountByUsername(ctx context.Context, username string) (*domain.Account, error)
	GetRemoteAccountByUsername(ctx context.Context, username string, domain *string) (*domain.Account, error)
	CountLocalAccounts(ctx context.Context) (int64, error)
	UpdateAccount(ctx context.Context, in UpdateAccountInput) error
	UpdateAccountKeys(ctx context.Context, id, publicKey string) error
	UpdateAccountURLs(ctx context.Context, id, inboxURL, outboxURL, followersURL, followingURL string) error
	UpdateRemoteAccountMeta(ctx context.Context, id, avatarURL, headerURL string, followersCount, followingCount, statusesCount int, featuredURL string) error
	SuspendAccount(ctx context.Context, id string) error
	UnsuspendAccount(ctx context.Context, id string) error
	SilenceAccount(ctx context.Context, id string) error
	UnsilenceAccount(ctx context.Context, id string) error
	DeleteAccount(ctx context.Context, id string) error
	ListLocalAccounts(ctx context.Context, limit, offset int) ([]domain.Account, error)
	GetRandomLocalAccount(ctx context.Context) (*domain.Account, error)
	ListDirectoryAccounts(ctx context.Context, order string, localOnly bool, offset, limit int) ([]domain.Account, error)
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
	CountAccountPublicStatuses(ctx context.Context, accountID string) (int64, error)
	SoftDeleteStatus(ctx context.Context, id string) error
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
	GetRandomFollowTarget(ctx context.Context, accountID string) (*domain.Account, error)
	GetUnbackfilledRemoteFollowing(ctx context.Context, accountID string, before time.Time, limit int) ([]domain.Account, error)
}

// InteractionStore handles blocks, mutes, favourites, bookmarks, reblogs, and quotes.
type InteractionStore interface {
	GetBlock(ctx context.Context, accountID, targetID string) (*domain.Block, error)
	CreateBlock(ctx context.Context, in CreateBlockInput) error
	DeleteBlock(ctx context.Context, accountID, targetID string) error
	IsBlockedEitherDirection(ctx context.Context, accountID, targetID string) (bool, error)
	ListBlockedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error)
	GetMute(ctx context.Context, accountID, targetID string) (*domain.Mute, error)
	CreateMute(ctx context.Context, in CreateMuteInput) error
	DeleteMute(ctx context.Context, accountID, targetID string) error
	ListMutedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error)
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
	ListNotifications(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Notification, error)
	GetNotification(ctx context.Context, id, accountID string) (*domain.Notification, error)
	ClearNotifications(ctx context.Context, accountID string) error
	DismissNotification(ctx context.Context, id, accountID string) error
}

// OAuthStore handles OAuth application, authorization code, and access token persistence.
type OAuthStore interface {
	CreateApplication(ctx context.Context, in CreateApplicationInput) (*domain.OAuthApplication, error)
	GetApplicationByClientID(ctx context.Context, clientID string) (*domain.OAuthApplication, error)
	CreateAuthorizationCode(ctx context.Context, in CreateAuthorizationCodeInput) (*domain.OAuthAuthorizationCode, error)
	GetAuthorizationCode(ctx context.Context, code string) (*domain.OAuthAuthorizationCode, error)
	DeleteAuthorizationCode(ctx context.Context, code string) error
	CreateAccessToken(ctx context.Context, in CreateAccessTokenInput) (*domain.OAuthAccessToken, error)
	GetAccessToken(ctx context.Context, token string) (*domain.OAuthAccessToken, error)
	RevokeAccessToken(ctx context.Context, token string) error
}

// MediaStore handles media attachment persistence.
type MediaStore interface {
	GetMediaAttachment(ctx context.Context, id string) (*domain.MediaAttachment, error)
	CreateMediaAttachment(ctx context.Context, in CreateMediaAttachmentInput) (*domain.MediaAttachment, error)
	UpdateMediaAttachment(ctx context.Context, in UpdateMediaAttachmentInput) (*domain.MediaAttachment, error)
}

// ModerationStore handles reports, domain blocks, admin actions, server filters, invites, and known instances.
type ModerationStore interface {
	CreateReport(ctx context.Context, in CreateReportInput) (*domain.Report, error)
	GetReportByID(ctx context.Context, id string) (*domain.Report, error)
	ListReports(ctx context.Context, state string, limit, offset int) ([]domain.Report, error)
	AssignReport(ctx context.Context, reportID string, assigneeID *string) error
	ResolveReport(ctx context.Context, reportID string, actionTaken *string) error
	CreateDomainBlock(ctx context.Context, in CreateDomainBlockInput) (*domain.DomainBlock, error)
	DeleteDomainBlock(ctx context.Context, domain string) error
	ListDomainBlocks(ctx context.Context) ([]domain.DomainBlock, error)
	CreateAdminAction(ctx context.Context, in CreateAdminActionInput) error
	CreateServerFilter(ctx context.Context, in CreateServerFilterInput) (*domain.ServerFilter, error)
	ListServerFilters(ctx context.Context) ([]domain.ServerFilter, error)
	UpdateServerFilter(ctx context.Context, in UpdateServerFilterInput) (*domain.ServerFilter, error)
	DeleteServerFilter(ctx context.Context, id string) error
	CreateInvite(ctx context.Context, in CreateInviteInput) (*domain.Invite, error)
	GetInviteByCode(ctx context.Context, code string) (*domain.Invite, error)
	ListInvitesByCreator(ctx context.Context, createdByUserID string) ([]domain.Invite, error)
	DeleteInvite(ctx context.Context, id string) error
	IncrementInviteUses(ctx context.Context, code string) error
	UpsertKnownInstance(ctx context.Context, id, domain string) error
	ListKnownInstances(ctx context.Context, limit, offset int) ([]domain.KnownInstance, error)
	CountKnownInstances(ctx context.Context) (int64, error)
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
	AddAccountToList(ctx context.Context, listID, accountID string) error
	RemoveAccountFromList(ctx context.Context, listID, accountID string) error
	GetListTimeline(ctx context.Context, listID string, maxID *string, limit int) ([]domain.Status, error)
}

// FilterStore handles user filter persistence.
type FilterStore interface {
	CreateUserFilter(ctx context.Context, in CreateUserFilterInput) (*domain.UserFilter, error)
	GetUserFilterByID(ctx context.Context, id string) (*domain.UserFilter, error)
	ListUserFilters(ctx context.Context, accountID string) ([]domain.UserFilter, error)
	UpdateUserFilter(ctx context.Context, in UpdateUserFilterInput) (*domain.UserFilter, error)
	DeleteUserFilter(ctx context.Context, id string) error
	GetActiveUserFiltersByContext(ctx context.Context, accountID, context string) ([]domain.UserFilter, error)
}

// MarkerStore handles timeline marker persistence.
type MarkerStore interface {
	GetMarkers(ctx context.Context, accountID string, timelines []string) (map[string]domain.Marker, error)
	SetMarker(ctx context.Context, accountID, timeline, lastReadID string) error
}

// TrendingStore handles trending status and tag persistence.
type TrendingStore interface {
	GetTopScoredPublicStatuses(ctx context.Context, since time.Time, limit int) ([]domain.TrendingStatus, error)
	GetHashtagDailyStats(ctx context.Context, since time.Time) ([]domain.HashtagDailyStats, error)
	ReplaceTrendingStatuses(ctx context.Context, entries []domain.TrendingStatus) error
	UpsertTrendingTagHistory(ctx context.Context, entries []domain.TrendingTagHistory) error
	GetTrendingStatusIDs(ctx context.Context, limit int) ([]domain.TrendingStatus, error)
	GetTrendingTags(ctx context.Context, days int, limit int) ([]domain.TrendingTag, error)
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
	GetVoteCountsByPoll(ctx context.Context, pollID string) (map[string]int, error)
	HasVotedOnPoll(ctx context.Context, pollID, accountID string) (bool, error)
	GetOwnVoteOptionIDs(ctx context.Context, pollID, accountID string) ([]string, error)
}

// CardStore handles status card (link preview) persistence.
type CardStore interface {
	UpsertStatusCard(ctx context.Context, in UpsertStatusCardInput) error
	GetStatusCard(ctx context.Context, statusID string) (*domain.Card, error)
	ListStatusIDsNeedingCards(ctx context.Context, since time.Time, limit int) ([]string, error)
}

// OutboxStore handles transactional outbox event persistence.
type OutboxStore interface {
	InsertOutboxEvent(ctx context.Context, in InsertOutboxEventInput) error
	GetAndLockUnpublishedOutboxEvents(ctx context.Context, limit int) ([]domain.DomainEvent, error)
	MarkOutboxEventsPublished(ctx context.Context, ids []string) error
	DeletePublishedOutboxEventsBefore(ctx context.Context, before time.Time) error
}
