package store

import (
	"context"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// Store is the persistence abstraction. All methods use domain types so that
// the service layer and callers depend only on store and domain, not on any
// specific SQL implementation (e.g. postgres).
type Store interface {
	CreateAccount(ctx context.Context, in CreateAccountInput) (*domain.Account, error)
	GetAccountByID(ctx context.Context, id string) (*domain.Account, error)
	GetAccountsByIDs(ctx context.Context, ids []string) ([]*domain.Account, error)
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
	GetFavouritesTimeline(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, *string, error)
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
	DeleteStatusMentions(ctx context.Context, statusID string) error
	GetStatusMentions(ctx context.Context, statusID string) ([]*domain.Account, error)
	GetOrCreateHashtag(ctx context.Context, name string) (*domain.Hashtag, error)
	SearchHashtagsByPrefix(ctx context.Context, prefix string, limit int) ([]domain.Hashtag, error)
	FollowTag(ctx context.Context, id, accountID, tagID string) error
	UnfollowTag(ctx context.Context, accountID, tagID string) error
	ListFollowedTags(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Hashtag, *string, error)
	CreateFeaturedTag(ctx context.Context, id, accountID, tagID string) error
	DeleteFeaturedTag(ctx context.Context, id, accountID string) error
	ListFeaturedTags(ctx context.Context, accountID string) ([]domain.FeaturedTag, error)
	GetFeaturedTagByID(ctx context.Context, id, accountID string) (*domain.FeaturedTag, error)
	ListFeaturedTagSuggestions(ctx context.Context, accountID string, limit int) ([]domain.Hashtag, []int64, error)
	GetConversationRoot(ctx context.Context, statusID string) (string, error)
	CreateConversationMute(ctx context.Context, accountID, conversationID string) error
	DeleteConversationMute(ctx context.Context, accountID, conversationID string) error
	IsConversationMuted(ctx context.Context, accountID, conversationID string) (bool, error)
	ListMutedConversationIDs(ctx context.Context, accountID string) ([]string, error)
	CreateConversation(ctx context.Context, id string) error
	SetStatusConversationID(ctx context.Context, statusID, conversationID string) error
	GetStatusConversationID(ctx context.Context, statusID string) (*string, error)
	UpsertAccountConversation(ctx context.Context, in UpsertAccountConversationInput) error
	ListAccountConversations(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.AccountConversation, *string, error)
	GetAccountConversation(ctx context.Context, accountID, conversationID string) (*domain.AccountConversation, error)
	MarkAccountConversationRead(ctx context.Context, accountID, conversationID string) error
	DeleteAccountConversation(ctx context.Context, accountID, conversationID string) error
	AttachHashtagsToStatus(ctx context.Context, statusID string, hashtagIDs []string) error
	DeleteStatusHashtags(ctx context.Context, statusID string) error
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
	GetFollowByID(ctx context.Context, id string) (*domain.Follow, error)
	GetFollowByAPID(ctx context.Context, apID string) (*domain.Follow, error)
	CreateFollow(ctx context.Context, in CreateFollowInput) (*domain.Follow, error)
	AcceptFollow(ctx context.Context, followID string) error
	DeleteFollow(ctx context.Context, accountID, targetID string) error
	GetFollowers(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error)
	GetFollowing(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error)
	GetPendingFollowRequests(ctx context.Context, targetID string, maxID *string, limit int) ([]domain.Account, *string, error)

	SoftDeleteStatus(ctx context.Context, id string) error
	SuspendAccount(ctx context.Context, id string) error

	CreateBlock(ctx context.Context, in CreateBlockInput) error
	DeleteBlock(ctx context.Context, accountID, targetID string) error
	IsBlockedEitherDirection(ctx context.Context, accountID, targetID string) (bool, error)
	ListBlockedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error)
	CreateMute(ctx context.Context, in CreateMuteInput) error
	DeleteMute(ctx context.Context, accountID, targetID string) error
	ListMutedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error)
	CreateFavourite(ctx context.Context, in CreateFavouriteInput) (*domain.Favourite, error)
	DeleteFavourite(ctx context.Context, accountID, statusID string) error
	GetFavouriteByAPID(ctx context.Context, apID string) (*domain.Favourite, error)
	GetFavouriteByAccountAndStatus(ctx context.Context, accountID, statusID string) (*domain.Favourite, error)
	CreateBookmark(ctx context.Context, in CreateBookmarkInput) error
	DeleteBookmark(ctx context.Context, accountID, statusID string) error
	GetBookmarks(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Status, *string, error)
	IsBookmarked(ctx context.Context, accountID, statusID string) (bool, error)
	IncrementFavouritesCount(ctx context.Context, statusID string) error
	DecrementFavouritesCount(ctx context.Context, statusID string) error
	IncrementReblogsCount(ctx context.Context, statusID string) error
	DecrementReblogsCount(ctx context.Context, statusID string) error
	IncrementRepliesCount(ctx context.Context, statusID string) error
	GetReblogByAccountAndTarget(ctx context.Context, accountID, statusID string) (*domain.Status, error)

	CreateAccountPin(ctx context.Context, accountID, statusID string) error
	DeleteAccountPin(ctx context.Context, accountID, statusID string) error
	ListPinnedStatusIDs(ctx context.Context, accountID string) ([]string, error)
	CountAccountPins(ctx context.Context, accountID string) (int64, error)

	UpdateAccount(ctx context.Context, in UpdateAccountInput) error
	UpdateAccountKeys(ctx context.Context, id, publicKey string, apRaw []byte) error
	AttachMediaToStatus(ctx context.Context, mediaID, statusID, accountID string) error
	CreateMediaAttachment(ctx context.Context, in CreateMediaAttachmentInput) (*domain.MediaAttachment, error)
	UpdateMediaAttachment(ctx context.Context, in UpdateMediaAttachmentInput) (*domain.MediaAttachment, error)
	CreateStatusEdit(ctx context.Context, in CreateStatusEditInput) error
	ListStatusEdits(ctx context.Context, statusID string) ([]domain.StatusEdit, error)
	UpdateStatus(ctx context.Context, in UpdateStatusInput) error

	CreateScheduledStatus(ctx context.Context, in CreateScheduledStatusInput) (*domain.ScheduledStatus, error)
	GetScheduledStatusByID(ctx context.Context, id string) (*domain.ScheduledStatus, error)
	ListScheduledStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.ScheduledStatus, error)
	UpdateScheduledStatus(ctx context.Context, in UpdateScheduledStatusInput) (*domain.ScheduledStatus, error)
	DeleteScheduledStatus(ctx context.Context, id string) error
	ListScheduledStatusesDue(ctx context.Context, limit int) ([]domain.ScheduledStatus, error)

	GetFollowerInboxURLs(ctx context.Context, accountID string) ([]string, error)
	GetDistinctFollowerInboxURLsPaginated(ctx context.Context, accountID string, cursor string, limit int) ([]string, error)
	GetLocalFollowerAccountIDs(ctx context.Context, targetID string) ([]string, error)
	GetStatusMentionAccountIDs(ctx context.Context, statusID string) ([]string, error)

	CreateReport(ctx context.Context, in CreateReportInput) (*domain.Report, error)
	GetReportByID(ctx context.Context, id string) (*domain.Report, error)
	ListReports(ctx context.Context, state string, limit, offset int) ([]domain.Report, error)
	AssignReport(ctx context.Context, reportID string, assigneeID *string) error
	ResolveReport(ctx context.Context, reportID string, actionTaken *string) error

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

	CreateDomainBlock(ctx context.Context, in CreateDomainBlockInput) (*domain.DomainBlock, error)
	GetDomainBlock(ctx context.Context, domain string) (*domain.DomainBlock, error)
	UpdateDomainBlock(ctx context.Context, domain string, severity string, reason *string) (*domain.DomainBlock, error)
	DeleteDomainBlock(ctx context.Context, domain string) error

	CreateAdminAction(ctx context.Context, in CreateAdminActionInput) error
	ListAdminActionsByTarget(ctx context.Context, targetAccountID string) ([]domain.AdminAction, error)

	CreateInvite(ctx context.Context, in CreateInviteInput) (*domain.Invite, error)
	GetInviteByCode(ctx context.Context, code string) (*domain.Invite, error)
	ListInvitesByCreator(ctx context.Context, createdByUserID string) ([]domain.Invite, error)
	DeleteInvite(ctx context.Context, id string) error
	IncrementInviteUses(ctx context.Context, code string) error

	SetSetting(ctx context.Context, key, value string) error
	ListSettings(ctx context.Context) (map[string]string, error)

	UpsertKnownInstance(ctx context.Context, id, domain string) error
	ListKnownInstances(ctx context.Context, limit, offset int) ([]domain.KnownInstance, error)
	CountKnownInstances(ctx context.Context) (int64, error)

	CreateServerFilter(ctx context.Context, in CreateServerFilterInput) (*domain.ServerFilter, error)
	GetServerFilter(ctx context.Context, id string) (*domain.ServerFilter, error)
	ListServerFilters(ctx context.Context) ([]domain.ServerFilter, error)
	UpdateServerFilter(ctx context.Context, in UpdateServerFilterInput) (*domain.ServerFilter, error)
	DeleteServerFilter(ctx context.Context, id string) error

	ListLocalUsers(ctx context.Context, limit, offset int) ([]domain.User, error)
	GetUserByID(ctx context.Context, id string) (*domain.User, error)
	UpdateUserRole(ctx context.Context, userID string, role string) error
	GetPendingRegistrations(ctx context.Context) ([]domain.User, error)
	DeleteUser(ctx context.Context, id string) error

	SilenceAccount(ctx context.Context, id string) error
	UnsuspendAccount(ctx context.Context, id string) error
	UnsilenceAccount(ctx context.Context, id string) error
	DeleteAccount(ctx context.Context, id string) error
	ListLocalAccounts(ctx context.Context, limit, offset int) ([]domain.Account, error)
	ListDirectoryAccounts(ctx context.Context, order string, localOnly bool, offset, limit int) ([]domain.Account, error)
	UpdateAccountLastStatusAt(ctx context.Context, accountID string) error

	DeleteFollowsByDomain(ctx context.Context, domain string) error

	CreateList(ctx context.Context, in CreateListInput) (*domain.List, error)
	GetListByID(ctx context.Context, id string) (*domain.List, error)
	ListLists(ctx context.Context, accountID string) ([]domain.List, error)
	UpdateList(ctx context.Context, in UpdateListInput) (*domain.List, error)
	DeleteList(ctx context.Context, id string) error
	ListListAccountIDs(ctx context.Context, listID string) ([]string, error)
	AddAccountToList(ctx context.Context, listID, accountID string) error
	RemoveAccountFromList(ctx context.Context, listID, accountID string) error
	GetListTimeline(ctx context.Context, listID string, maxID *string, limit int) ([]domain.Status, error)

	CreateUserFilter(ctx context.Context, in CreateUserFilterInput) (*domain.UserFilter, error)
	GetUserFilterByID(ctx context.Context, id string) (*domain.UserFilter, error)
	ListUserFilters(ctx context.Context, accountID string) ([]domain.UserFilter, error)
	UpdateUserFilter(ctx context.Context, in UpdateUserFilterInput) (*domain.UserFilter, error)
	DeleteUserFilter(ctx context.Context, id string) error
	GetActiveUserFiltersByContext(ctx context.Context, accountID, context string) ([]domain.UserFilter, error)

	GetMarkers(ctx context.Context, accountID string, timelines []string) (map[string]domain.Marker, error)
	SetMarker(ctx context.Context, accountID, timeline, lastReadID string) error

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
