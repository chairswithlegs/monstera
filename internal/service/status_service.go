package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// Quote-related methods implement Mastodon-style quotes: quoted_status_id, quote_approval_policy,
// quote approvals, revoke, and quoted_update notifications. See docs/mastodon-api-* for the plan.

// StatusFederationPublisher publishes status create/delete to federation (e.g. ap.Outbox).
type StatusFederationPublisher interface {
	PublishStatus(ctx context.Context, status *domain.Status) error
	DeleteStatus(ctx context.Context, status *domain.Status) error
	PublishUpdate(ctx context.Context, status *domain.Status) error
}

// NoopFederationPublisher is a StatusFederationPublisher that does nothing.
// Use when federation is disabled (e.g. no NATS) or in tests.
var NoopFederationPublisher StatusFederationPublisher = (*noopFederationPublisher)(nil)

type noopFederationPublisher struct{}

func (*noopFederationPublisher) PublishStatus(context.Context, *domain.Status) error { return nil }
func (*noopFederationPublisher) DeleteStatus(context.Context, *domain.Status) error  { return nil }
func (*noopFederationPublisher) PublishUpdate(context.Context, *domain.Status) error { return nil }

// StatusVisibilityChecker allows callers to check if a viewer can see a status (visibility + blocks).
// TimelineService depends on this narrow interface to filter list timelines.
type StatusVisibilityChecker interface {
	CanViewStatus(ctx context.Context, st *domain.Status, viewerAccountID *string) (bool, error)
}

// StatusService handles status creation, lookup, and soft delete.
type StatusService interface {
	StatusVisibilityChecker
	Create(ctx context.Context, in CreateStatusInput) (*domain.Status, error)
	GetByID(ctx context.Context, id string) (*domain.Status, error)
	GetByAPID(ctx context.Context, apID string) (*domain.Status, error)
	CreateFromInbox(ctx context.Context, in CreateStatusFromInboxInput) (*domain.Status, error)
	CreateBoostFromInbox(ctx context.Context, accountID string, activityAPID, objectStatusAPID string, apRaw []byte) (*domain.Status, error)
	UpdateFromInbox(ctx context.Context, statusID string, st *domain.Status, in UpdateStatusFromInboxInput) error
	SoftDelete(ctx context.Context, statusID string) error
	DecrementReblogsCount(ctx context.Context, statusID string) error
	GetReblogByAccountAndTarget(ctx context.Context, accountID, statusID string) (*domain.Status, error)
	IncrementRepliesCount(ctx context.Context, statusID string) error
	AttachMediaToStatus(ctx context.Context, mediaID, statusID, accountID string) error
	GetFavouriteByAPID(ctx context.Context, apID string) (*domain.Favourite, error)
	GetFavouriteByAccountAndStatus(ctx context.Context, accountID, statusID string) (*domain.Favourite, error)
	CreateFavouriteFromInbox(ctx context.Context, accountID, statusID string, apID *string) (*domain.Favourite, error)
	DeleteFavourite(ctx context.Context, accountID, statusID string) error
	DecrementFavouritesCount(ctx context.Context, statusID string) error
	GetByIDEnriched(ctx context.Context, id string, viewerAccountID *string) (EnrichedStatus, error)
	CreateWithContent(ctx context.Context, in CreateWithContentInput) (EnrichedStatus, error)
	Delete(ctx context.Context, id string) error
	Reblog(ctx context.Context, accountID, username, statusID string) (EnrichedStatus, error)
	Unreblog(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	Favourite(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	Unfavourite(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	GetContext(ctx context.Context, statusID string, viewerAccountID *string) (ContextResult, error)
	GetFavouritedBy(ctx context.Context, statusID string, viewerAccountID *string, maxID *string, limit int) ([]*domain.Account, error)
	GetRebloggedBy(ctx context.Context, statusID string, viewerAccountID *string, maxID *string, limit int) ([]*domain.Account, error)
	Bookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	Unbookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	IsBookmarked(ctx context.Context, accountID, statusID string) (bool, error)
	Pin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	Unpin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	ListPinnedStatusIDs(ctx context.Context, accountID string) ([]string, error)
	UpdateStatusFromAPI(ctx context.Context, accountID, statusID string, text, spoilerText string, sensitive bool) (EnrichedStatus, error)
	GetStatusHistory(ctx context.Context, statusID string, viewerAccountID *string) ([]domain.StatusEdit, error)
	GetStatusSource(ctx context.Context, statusID string, viewerAccountID *string) (text, spoilerText string, err error)

	CreateScheduledStatus(ctx context.Context, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error)
	GetScheduledStatus(ctx context.Context, id, accountID string) (*domain.ScheduledStatus, error)
	ListScheduledStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.ScheduledStatus, error)
	UpdateScheduledStatus(ctx context.Context, id, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error)
	DeleteScheduledStatus(ctx context.Context, id, accountID string) error
	PublishScheduled(ctx context.Context, scheduledID string) error

	GetPoll(ctx context.Context, pollID string, viewerAccountID *string) (*EnrichedPoll, error)
	RecordVote(ctx context.Context, pollID, accountID string, optionIndices []int) (*EnrichedPoll, error)

	MuteConversation(ctx context.Context, accountID, statusID string) error
	UnmuteConversation(ctx context.Context, accountID, statusID string) error
	GetConversationRoot(ctx context.Context, statusID string) (string, error)
	IsConversationMutedForViewer(ctx context.Context, viewerAccountID, statusID string) (bool, error)
	ListMutedConversationIDs(ctx context.Context, accountID string) ([]string, error)

	GetQuoteApproval(ctx context.Context, quotingStatusID string) (*domain.QuoteApprovalRecord, error)
	UpdateQuoteApprovalPolicy(ctx context.Context, accountID, statusID, policy string) error
	ListQuotesOfStatus(ctx context.Context, quotedStatusID string, maxID *string, limit int, viewerAccountID *string) ([]EnrichedStatus, error)
	RevokeQuote(ctx context.Context, accountID, quotedStatusID, quotingStatusID string) error

	// SetDirectStatusConversationUpdater sets the optional updater for direct statuses (used by wiring to break circular dependency).
	SetDirectStatusConversationUpdater(DirectStatusConversationUpdater)
}

// DirectStatusConversationUpdater is called when a direct-visibility status is created so the conversation list can be updated.
// Implemented by ConversationService; may be nil to skip.
type DirectStatusConversationUpdater interface {
	UpdateForDirectStatus(ctx context.Context, status *domain.Status, authorID string, mentionedAccountIDs []string) error
}

type statusService struct {
	store           store.Store
	fed             StatusFederationPublisher
	eventBus        events.EventBus
	convUpdater     DirectStatusConversationUpdater
	instanceBaseURL string
	instanceDomain  string
	maxStatusChars  int
	logger          *slog.Logger
}

// NewStatusService returns a StatusService that uses the given store and instance URLs.
// fed must be non-nil; use NoopFederationPublisher when federation is disabled.
// eventBus must be non-nil; use events.NoopEventBus in tests or when SSE is disabled. logger may be nil (federation failures will not be logged).
// convUpdater may be nil; when set, direct statuses will update the conversations list.
func NewStatusService(s store.Store, fed StatusFederationPublisher, eventBus events.EventBus, convUpdater DirectStatusConversationUpdater, instanceBaseURL, instanceDomain string, maxStatusChars int, logger *slog.Logger) StatusService {
	if fed == nil {
		panic("StatusService: fed must be non-nil. Use NoopFederationPublisher when federation is disabled")
	}
	base := strings.TrimSuffix(instanceBaseURL, "/")
	return &statusService{
		store:           s,
		fed:             fed,
		eventBus:        eventBus,
		convUpdater:     convUpdater,
		instanceBaseURL: base,
		instanceDomain:  instanceDomain,
		maxStatusChars:  maxStatusChars,
		logger:          logger,
	}
}

// SetDirectStatusConversationUpdater sets the optional conversation updater for direct statuses.
func (svc *statusService) SetDirectStatusConversationUpdater(u DirectStatusConversationUpdater) {
	svc.convUpdater = u
}

// CreateWithContentInput is the input for creating a status with plain text (content is rendered in-service).
// When DefaultVisibility or DefaultQuotePolicy are empty, the service looks up the account's user for defaults.
type CreateWithContentInput struct {
	AccountID           string
	Username            string
	Text                string
	Visibility          string
	DefaultVisibility   string // used when Visibility is empty or invalid; when empty, looked up from user
	DefaultQuotePolicy  string // public | followers | nobody; when empty, looked up from user
	ContentWarning      string
	Language            string
	Sensitive           bool
	InReplyToID         *string     // optional parent status ID for replies
	QuotedStatusID      *string     // optional status ID being quoted
	QuoteApprovalPolicy string      // public | followers | nobody (for the new status); service applies private/direct -> nobody
	MediaIDs            []string    // optional media attachment IDs (max 4; service caps at 4)
	Poll                *PollInput  // optional; when set, status is created with an attached poll
	PollLimits          *PollLimits // required when Poll is set (from instance configuration)
}

// PollInput is the poll payload when creating a status with a poll.
type PollInput struct {
	Options          []string
	ExpiresInSeconds int
	Multiple         bool
}

// PollLimits is instance configuration for poll validation (e.g. from GET /api/v2/instance).
type PollLimits struct {
	MaxOptions    int
	MinExpiration int
	MaxExpiration int
}

// CreateStatusInput is the input for creating a status.
type CreateStatusInput struct {
	AccountID      string
	Text           *string
	Content        *string
	ContentWarning *string
	Visibility     string
	Language       *string
	InReplyToID    *string
	ReblogOfID     *string
	Sensitive      bool
}

// Create creates a status and increments the account's statuses count atomically.
func (svc *statusService) Create(ctx context.Context, in CreateStatusInput) (*domain.Status, error) {
	if in.AccountID == "" {
		return nil, fmt.Errorf("CreateStatus: %w", domain.ErrValidation)
	}
	if in.Text == nil || *in.Text == "" {
		return nil, fmt.Errorf("CreateStatus: %w", domain.ErrValidation)
	}
	switch in.Visibility {
	case domain.VisibilityPublic, domain.VisibilityUnlisted, domain.VisibilityPrivate, domain.VisibilityDirect:
	default:
		return nil, fmt.Errorf("CreateStatus: %w", domain.ErrValidation)
	}
	id := uid.New()
	uri := fmt.Sprintf("%s/statuses/%s", svc.instanceBaseURL, id)
	storeIn := store.CreateStatusInput{
		ID:             id,
		URI:            uri,
		AccountID:      in.AccountID,
		Text:           in.Text,
		Content:        in.Content,
		ContentWarning: in.ContentWarning,
		Visibility:     in.Visibility,
		Language:       in.Language,
		InReplyToID:    in.InReplyToID,
		ReblogOfID:     in.ReblogOfID,
		APID:           uri,
		ApRaw:          nil,
		Sensitive:      in.Sensitive,
		Local:          true,
	}
	var st *domain.Status
	err := svc.store.WithTx(ctx, func(tx store.Store) error {
		var err error
		st, err = tx.CreateStatus(ctx, storeIn)
		if err != nil {
			return fmt.Errorf("CreateStatus: %w", err)
		}
		if err := tx.IncrementStatusesCount(ctx, in.AccountID); err != nil {
			return fmt.Errorf("IncrementStatusesCount: %w", err)
		}
		if err := tx.UpdateAccountLastStatusAt(ctx, in.AccountID); err != nil {
			return fmt.Errorf("UpdateAccountLastStatusAt: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("CreateStatus: %w", err)
	}
	if err := svc.fed.PublishStatus(ctx, st); err != nil && svc.logger != nil {
		svc.logger.WarnContext(ctx, "federation publish failed after status create", slog.Any("error", err), slog.String("status_id", st.ID))
	}
	return st, nil
}

// GetByID returns the status by ID, or ErrNotFound.
func (svc *statusService) GetByID(ctx context.Context, id string) (*domain.Status, error) {
	st, err := svc.store.GetStatusByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetStatusByID(%s): %w", id, err)
	}
	return st, nil
}

// GetByAPID returns the status by ActivityPub ID (URI), or ErrNotFound.
func (svc *statusService) GetByAPID(ctx context.Context, apID string) (*domain.Status, error) {
	st, err := svc.store.GetStatusByAPID(ctx, apID)
	if err != nil {
		return nil, fmt.Errorf("GetStatusByAPID(%s): %w", apID, err)
	}
	return st, nil
}

// CreateStatusFromInboxInput is the input for creating a status from an incoming Note (inbox).
type CreateStatusFromInboxInput struct {
	AccountID      string
	URI            string
	Text           *string
	Content        *string
	ContentWarning *string
	Visibility     string
	Language       *string
	InReplyToID    *string
	APID           string
	ApRaw          []byte
	Sensitive      bool
}

// CreateFromInbox creates a status from an incoming Note. Does not publish to federation or increment account statuses count.
func (svc *statusService) CreateFromInbox(ctx context.Context, in CreateStatusFromInboxInput) (*domain.Status, error) {
	st, err := svc.store.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  uid.New(),
		URI:                 in.URI,
		AccountID:           in.AccountID,
		Text:                in.Text,
		Content:             in.Content,
		ContentWarning:      in.ContentWarning,
		Visibility:          in.Visibility,
		Language:            in.Language,
		InReplyToID:         in.InReplyToID,
		APID:                in.APID,
		ApRaw:               in.ApRaw,
		Sensitive:           in.Sensitive,
		Local:               false,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateFromInbox: %w", err)
	}
	if in.Visibility == domain.VisibilityDirect && svc.convUpdater != nil {
		mentionedIDs, _ := svc.store.GetStatusMentionAccountIDs(ctx, st.ID)
		if updErr := svc.convUpdater.UpdateForDirectStatus(ctx, st, st.AccountID, mentionedIDs); updErr != nil && svc.logger != nil {
			svc.logger.WarnContext(ctx, "conversation update failed after direct status from inbox", slog.Any("error", updErr), slog.String("status_id", st.ID))
		}
	}
	return st, nil
}

// CreateBoostFromInbox creates a reblog status from an incoming Announce. Increments reblogs count on the original. Does not publish to federation.
func (svc *statusService) CreateBoostFromInbox(ctx context.Context, accountID string, activityAPID, objectStatusAPID string, apRaw []byte) (*domain.Status, error) {
	original, err := svc.store.GetStatusByAPID(ctx, objectStatusAPID)
	if err != nil {
		return nil, fmt.Errorf("CreateBoostFromInbox GetStatusByAPID: %w", err)
	}
	reblogOfID := original.ID
	st, err := svc.store.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  uid.New(),
		URI:                 activityAPID,
		AccountID:           accountID,
		Visibility:          domain.VisibilityPublic,
		ReblogOfID:          &reblogOfID,
		APID:                activityAPID,
		ApRaw:               apRaw,
		Local:               false,
		QuoteApprovalPolicy: domain.QuotePolicyPublic,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateBoostFromInbox CreateStatus: %w", err)
	}
	if err := svc.store.IncrementReblogsCount(ctx, original.ID); err != nil {
		return nil, fmt.Errorf("CreateBoostFromInbox IncrementReblogsCount: %w", err)
	}
	return st, nil
}

// UpdateStatusFromInboxInput is the input for updating a status from an incoming Update{Note}.
type UpdateStatusFromInboxInput struct {
	Text           *string
	Content        *string
	ContentWarning *string
	Sensitive      bool
}

// UpdateFromInbox creates a status edit record and updates the status (for incoming Update{Note}).
func (svc *statusService) UpdateFromInbox(ctx context.Context, statusID string, st *domain.Status, in UpdateStatusFromInboxInput) error {
	if err := svc.store.CreateStatusEdit(ctx, store.CreateStatusEditInput{
		ID:             uid.New(),
		StatusID:       statusID,
		AccountID:      st.AccountID,
		Text:           st.Text,
		Content:        st.Content,
		ContentWarning: st.ContentWarning,
		Sensitive:      st.Sensitive,
	}); err != nil {
		return fmt.Errorf("UpdateFromInbox CreateStatusEdit: %w", err)
	}
	if err := svc.store.UpdateStatus(ctx, store.UpdateStatusInput{
		ID:             statusID,
		Text:           in.Text,
		Content:        in.Content,
		ContentWarning: in.ContentWarning,
		Sensitive:      in.Sensitive,
	}); err != nil {
		return fmt.Errorf("UpdateFromInbox UpdateStatus: %w", err)
	}
	return nil
}

// SoftDelete soft-deletes the status (for Delete{Note} or Undo Announce). Does not decrement account statuses count or publish.
func (svc *statusService) SoftDelete(ctx context.Context, statusID string) error {
	if err := svc.store.SoftDeleteStatus(ctx, statusID); err != nil {
		return fmt.Errorf("SoftDelete(%s): %w", statusID, err)
	}
	return nil
}

// DecrementReblogsCount decrements the reblogs count on the status (for Undo Announce).
func (svc *statusService) DecrementReblogsCount(ctx context.Context, statusID string) error {
	if err := svc.store.DecrementReblogsCount(ctx, statusID); err != nil {
		return fmt.Errorf("DecrementReblogsCount(%s): %w", statusID, err)
	}
	return nil
}

// GetReblogByAccountAndTarget returns the boost status for the given account and original status (for Undo Announce).
func (svc *statusService) GetReblogByAccountAndTarget(ctx context.Context, accountID, statusID string) (*domain.Status, error) {
	st, err := svc.store.GetReblogByAccountAndTarget(ctx, accountID, statusID)
	if err != nil {
		return nil, fmt.Errorf("GetReblogByAccountAndTarget: %w", err)
	}
	return st, nil
}

// IncrementRepliesCount increments the replies count on the parent status (for Create note in reply).
func (svc *statusService) IncrementRepliesCount(ctx context.Context, statusID string) error {
	if err := svc.store.IncrementRepliesCount(ctx, statusID); err != nil {
		return fmt.Errorf("IncrementRepliesCount(%s): %w", statusID, err)
	}
	return nil
}

// AttachMediaToStatus attaches a media attachment to a status (for Create note with attachments).
func (svc *statusService) AttachMediaToStatus(ctx context.Context, mediaID, statusID, accountID string) error {
	if err := svc.store.AttachMediaToStatus(ctx, mediaID, statusID, accountID); err != nil {
		return fmt.Errorf("AttachMediaToStatus: %w", err)
	}
	return nil
}

// GetFavouriteByAPID returns the favourite by its ActivityPub ID (for Undo Like).
func (svc *statusService) GetFavouriteByAPID(ctx context.Context, apID string) (*domain.Favourite, error) {
	fav, err := svc.store.GetFavouriteByAPID(ctx, apID)
	if err != nil {
		return nil, fmt.Errorf("GetFavouriteByAPID: %w", err)
	}
	return fav, nil
}

// GetFavouriteByAccountAndStatus returns the favourite for the given account and status (for Undo Like).
func (svc *statusService) GetFavouriteByAccountAndStatus(ctx context.Context, accountID, statusID string) (*domain.Favourite, error) {
	fav, err := svc.store.GetFavouriteByAccountAndStatus(ctx, accountID, statusID)
	if err != nil {
		return nil, fmt.Errorf("GetFavouriteByAccountAndStatus: %w", err)
	}
	return fav, nil
}

// CreateFavouriteFromInbox creates a favourite from an incoming Like. Increments favourites count. Does not create notification (caller does).
func (svc *statusService) CreateFavouriteFromInbox(ctx context.Context, accountID, statusID string, apID *string) (*domain.Favourite, error) {
	fav, err := svc.store.CreateFavourite(ctx, store.CreateFavouriteInput{
		ID:        uid.New(),
		AccountID: accountID,
		StatusID:  statusID,
		APID:      apID,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateFavouriteFromInbox: %w", err)
	}
	if err := svc.store.IncrementFavouritesCount(ctx, statusID); err != nil {
		return nil, fmt.Errorf("CreateFavouriteFromInbox IncrementFavouritesCount: %w", err)
	}
	return fav, nil
}

// DeleteFavourite removes the favourite (for Undo Like).
func (svc *statusService) DeleteFavourite(ctx context.Context, accountID, statusID string) error {
	if err := svc.store.DeleteFavourite(ctx, accountID, statusID); err != nil {
		return fmt.Errorf("DeleteFavourite: %w", err)
	}
	return nil
}

// DecrementFavouritesCount decrements the favourites count on the status (for Undo Like).
func (svc *statusService) DecrementFavouritesCount(ctx context.Context, statusID string) error {
	if err := svc.store.DecrementFavouritesCount(ctx, statusID); err != nil {
		return fmt.Errorf("DecrementFavouritesCount(%s): %w", statusID, err)
	}
	return nil
}

// canViewStatus returns whether the viewer can see the status (visibility and block rules).
func (svc *statusService) canViewStatus(ctx context.Context, st *domain.Status, viewerAccountID *string) (bool, error) {
	switch st.Visibility {
	case domain.VisibilityPublic, domain.VisibilityUnlisted:
		// fall through to block check
	case domain.VisibilityPrivate:
		if viewerAccountID == nil {
			return false, nil
		}
		if *viewerAccountID == st.AccountID {
			break
		}
		follow, err := svc.store.GetFollow(ctx, *viewerAccountID, st.AccountID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return false, fmt.Errorf("GetFollow: %w", err)
		}
		if follow != nil && follow.State == domain.FollowStateAccepted {
			break
		}
		mentionIDs, err := svc.store.GetStatusMentionAccountIDs(ctx, st.ID)
		if err != nil {
			return false, fmt.Errorf("GetStatusMentionAccountIDs: %w", err)
		}
		if sliceContains(mentionIDs, *viewerAccountID) {
			break
		}
		return false, nil
	case domain.VisibilityDirect:
		if viewerAccountID == nil {
			return false, nil
		}
		if *viewerAccountID == st.AccountID {
			break
		}
		mentionIDs, err := svc.store.GetStatusMentionAccountIDs(ctx, st.ID)
		if err != nil {
			return false, fmt.Errorf("GetStatusMentionAccountIDs: %w", err)
		}
		if !sliceContains(mentionIDs, *viewerAccountID) {
			return false, nil
		}
	default:
		return false, nil
	}
	if viewerAccountID != nil {
		blocked, err := svc.store.IsBlockedEitherDirection(ctx, *viewerAccountID, st.AccountID)
		if err != nil {
			return false, fmt.Errorf("IsBlockedEitherDirection: %w", err)
		}
		if blocked {
			return false, nil
		}
	}
	return true, nil
}

func sliceContains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}

// CanViewStatus implements StatusVisibilityChecker.
func (svc *statusService) CanViewStatus(ctx context.Context, st *domain.Status, viewerAccountID *string) (bool, error) {
	return svc.canViewStatus(ctx, st, viewerAccountID)
}

// GetByIDEnriched returns the status with author, mentions, tags, and media for API response.
// If the viewer cannot see the status (visibility or block), returns domain.ErrNotFound.
func (svc *statusService) GetByIDEnriched(ctx context.Context, id string, viewerAccountID *string) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, id)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("GetStatusByID(%s): %w", id, err)
	}
	if st.DeletedAt != nil {
		return EnrichedStatus{}, fmt.Errorf("GetByIDEnriched(%s): %w", id, domain.ErrNotFound)
	}
	ok, err := svc.canViewStatus(ctx, st, viewerAccountID)
	if err != nil {
		return EnrichedStatus{}, err
	}
	if !ok {
		return EnrichedStatus{}, fmt.Errorf("GetByIDEnriched(%s): %w", id, domain.ErrNotFound)
	}
	author, err := svc.store.GetAccountByID(ctx, st.AccountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("GetAccountByID: %w", err)
	}
	mentions, err := svc.store.GetStatusMentions(ctx, st.ID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("GetStatusMentions: %w", err)
	}
	tags, err := svc.store.GetStatusHashtags(ctx, st.ID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("GetStatusHashtags: %w", err)
	}
	media, err := svc.store.GetStatusAttachments(ctx, st.ID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("GetStatusAttachments: %w", err)
	}
	out := EnrichedStatus{
		Status:   st,
		Author:   author,
		Mentions: mentions,
		Tags:     tags,
		Media:    media,
	}
	poll, pollErr := svc.store.GetPollByStatusID(ctx, st.ID)
	if pollErr == nil && poll != nil {
		enrichedPoll, enrichErr := svc.getPollEnriched(ctx, poll.ID, viewerAccountID)
		if enrichErr == nil {
			out.Poll = enrichedPoll
		}
	}
	if viewerAccountID != nil {
		if ok, err := svc.store.IsBookmarked(ctx, *viewerAccountID, st.ID); err == nil {
			out.Bookmarked = ok
		}
		if st.AccountID == *viewerAccountID {
			pinnedIDs, err := svc.store.ListPinnedStatusIDs(ctx, *viewerAccountID)
			if err == nil {
				for _, pid := range pinnedIDs {
					if pid == st.ID {
						out.Pinned = true
						break
					}
				}
			}
		}
		if muted, err := svc.IsConversationMutedForViewer(ctx, *viewerAccountID, st.ID); err == nil {
			out.Muted = muted
		}
	}
	return out, nil
}

// CreateWithContent creates a status from plain text: validates, renders content (mentions, hashtags),
// persists status with mentions, hashtags, and mention notifications in one transaction.
// Supports Mastodon-style quotes (quoted_status_id, quote approval, quote notification) when QuotedStatusID is set.
// Then loads author, mentions, tags, and media for the response.
func (svc *statusService) CreateWithContent(ctx context.Context, in CreateWithContentInput) (EnrichedStatus, error) {
	defaultVisibility := in.DefaultVisibility
	defaultQuotePolicy := in.DefaultQuotePolicy
	if defaultVisibility == "" || defaultQuotePolicy == "" {
		if user, err := svc.store.GetUserByAccountID(ctx, in.AccountID); err == nil {
			if defaultVisibility == "" {
				defaultVisibility = user.DefaultPrivacy
			}
			if defaultQuotePolicy == "" && user.DefaultQuotePolicy != "" {
				defaultQuotePolicy = user.DefaultQuotePolicy
			}
		}
		if defaultQuotePolicy == "" {
			defaultQuotePolicy = domain.QuotePolicyPublic
		}
	}
	mediaIDs := in.MediaIDs
	if len(mediaIDs) > 4 {
		mediaIDs = mediaIDs[:4]
	}
	text := strings.TrimSpace(in.Text)
	if text == "" && len(mediaIDs) == 0 {
		return EnrichedStatus{}, fmt.Errorf("CreateWithContent: %w", domain.ErrValidation)
	}
	visibility := resolveVisibilityService(in.Visibility, defaultVisibility)
	if text != "" && CountStatusCharacters(text) > svc.maxStatusChars {
		return EnrichedStatus{}, fmt.Errorf("CreateWithContent: %w", domain.ErrValidation)
	}
	var inReplyToAccountID *string
	if in.InReplyToID != nil && *in.InReplyToID != "" {
		parent, err := svc.store.GetStatusByID(ctx, *in.InReplyToID)
		if err != nil {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent in_reply_to: %w", err)
		}
		if parent.DeletedAt != nil {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent in_reply_to: %w", domain.ErrNotFound)
		}
		inReplyToAccountID = &parent.AccountID
	}
	for _, mid := range mediaIDs {
		att, err := svc.store.GetMediaAttachment(ctx, mid)
		if err != nil {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent media %s: %w", mid, err)
		}
		if att.AccountID != in.AccountID {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent media: %w", domain.ErrForbidden)
		}
	}
	var quotedAuthorID *string
	if in.QuotedStatusID != nil && *in.QuotedStatusID != "" {
		if (in.Poll != nil && len(in.Poll.Options) > 0) || len(mediaIDs) > 0 {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent: %w", domain.ErrValidation)
		}
		quoted, err := svc.store.GetStatusByID(ctx, *in.QuotedStatusID)
		if err != nil {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent quoted_status_id: %w", err)
		}
		if quoted.DeletedAt != nil {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent quoted_status_id: %w", domain.ErrNotFound)
		}
		visible, err := svc.canViewStatus(ctx, quoted, &in.AccountID)
		if err != nil {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent quoted_status_id: %w", err)
		}
		if !visible {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent quoted_status_id: %w", domain.ErrForbidden)
		}
		switch quoted.QuoteApprovalPolicy {
		case domain.QuotePolicyNobody:
			if quoted.AccountID != in.AccountID {
				return EnrichedStatus{}, fmt.Errorf("CreateWithContent quoted_status_id: %w", domain.ErrForbidden)
			}
		case domain.QuotePolicyFollowers:
			if quoted.AccountID != in.AccountID {
				follow, followErr := svc.store.GetFollow(ctx, in.AccountID, quoted.AccountID)
				if followErr != nil || follow == nil || follow.State != domain.FollowStateAccepted {
					return EnrichedStatus{}, fmt.Errorf("CreateWithContent quoted_status_id: %w", domain.ErrForbidden)
				}
			}
		default:
			// public or unknown: allow (block already checked in canViewStatus)
		}
		quotedAuthorID = &quoted.AccountID
	}
	renderResult := RenderResult{}
	if text != "" {
		resolver := svc.mentionResolver(ctx)
		var err error
		renderResult, err = Render(text, svc.instanceDomain, resolver)
		if err != nil {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent Render: %w", err)
		}
	}
	if in.Poll != nil {
		if in.PollLimits == nil {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent: %w", domain.ErrValidation)
		}
		if len(in.Poll.Options) < 2 || len(in.Poll.Options) > in.PollLimits.MaxOptions {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent poll options: %w", domain.ErrValidation)
		}
		if in.Poll.ExpiresInSeconds < in.PollLimits.MinExpiration || in.Poll.ExpiresInSeconds > in.PollLimits.MaxExpiration {
			return EnrichedStatus{}, fmt.Errorf("CreateWithContent poll expires_in: %w", domain.ErrValidation)
		}
	}
	// TODO: this should be a setting
	language := in.Language
	if language == "" {
		language = "en"
	}
	statusID := uid.New()
	statusURI := fmt.Sprintf("%s/users/%s/statuses/%s", svc.instanceBaseURL, in.Username, statusID)

	quotePolicy := in.QuoteApprovalPolicy
	if quotePolicy == "" {
		quotePolicy = defaultQuotePolicy
	}
	if visibility == domain.VisibilityPrivate || visibility == domain.VisibilityDirect {
		quotePolicy = domain.QuotePolicyNobody
	}

	var created *domain.Status
	var createdPollID string
	// Single transaction: status create, quote approval, quotes_count increment, and quote notification (Mastodon-style quotes) are atomic.
	err := svc.store.WithTx(ctx, func(tx store.Store) error {
		var txErr error
		created, txErr = createStatusWithContentTx(ctx, tx, in.AccountID, in.Username, statusID, statusURI, visibility, text, renderResult.HTML, in.ContentWarning, language, in.Sensitive, renderResult, in.InReplyToID, inReplyToAccountID, in.QuotedStatusID, quotePolicy, quotedAuthorID, mediaIDs)
		if txErr != nil {
			return txErr
		}
		if in.InReplyToID != nil && *in.InReplyToID != "" {
			if txErr = tx.IncrementRepliesCount(ctx, *in.InReplyToID); txErr != nil {
				return fmt.Errorf("IncrementRepliesCount: %w", txErr)
			}
		}
		for _, mid := range mediaIDs {
			if txErr = tx.AttachMediaToStatus(ctx, mid, statusID, in.AccountID); txErr != nil {
				return fmt.Errorf("AttachMediaToStatus: %w", txErr)
			}
		}
		if in.Poll != nil {
			pollID := uid.New()
			expiresAt := time.Now().Add(time.Duration(in.Poll.ExpiresInSeconds) * time.Second)
			if _, txErr = tx.CreatePoll(ctx, store.CreatePollInput{
				ID:        pollID,
				StatusID:  statusID,
				ExpiresAt: &expiresAt,
				Multiple:  in.Poll.Multiple,
			}); txErr != nil {
				return fmt.Errorf("CreatePoll: %w", txErr)
			}
			for i, title := range in.Poll.Options {
				optID := uid.New()
				if _, txErr = tx.CreatePollOption(ctx, store.CreatePollOptionInput{
					ID:       optID,
					PollID:   pollID,
					Title:    strings.TrimSpace(title),
					Position: i,
				}); txErr != nil {
					return fmt.Errorf("CreatePollOption: %w", txErr)
				}
			}
			createdPollID = pollID
		}
		return nil
	})
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateWithContent: %w", err)
	}

	author, err := svc.store.GetAccountByID(ctx, in.AccountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateWithContent GetAccountByID: %w", err)
	}
	mentions, err := svc.store.GetStatusMentions(ctx, statusID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateWithContent GetStatusMentions: %w", err)
	}
	tags, err := svc.store.GetStatusHashtags(ctx, statusID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateWithContent GetStatusHashtags: %w", err)
	}
	media, err := svc.store.GetStatusAttachments(ctx, statusID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateWithContent GetStatusAttachments: %w", err)
	}
	if err := svc.fed.PublishStatus(ctx, created); err != nil && svc.logger != nil {
		svc.logger.WarnContext(ctx, "federation publish failed after CreateWithContent", slog.Any("error", err), slog.String("status_id", created.ID))
	}
	mentionedAccountIDs := make([]string, 0, len(mentions))
	for _, m := range mentions {
		if m != nil {
			mentionedAccountIDs = append(mentionedAccountIDs, m.ID)
		}
	}
	svc.eventBus.PublishStatusCreated(ctx, events.StatusCreatedEvent{
		Status:              created,
		Author:              author,
		Mentions:            mentions,
		Tags:                tags,
		Media:               media,
		MentionedAccountIDs: mentionedAccountIDs,
	})
	if created.Visibility == domain.VisibilityDirect && svc.convUpdater != nil {
		if updErr := svc.convUpdater.UpdateForDirectStatus(ctx, created, in.AccountID, mentionedAccountIDs); updErr != nil && svc.logger != nil {
			svc.logger.WarnContext(ctx, "conversation update failed after direct status create", slog.Any("error", updErr), slog.String("status_id", created.ID))
		}
	}
	out := EnrichedStatus{
		Status:   created,
		Author:   author,
		Mentions: mentions,
		Tags:     tags,
		Media:    media,
	}
	if createdPollID != "" {
		enrichedPoll, getErr := svc.getPollEnriched(ctx, createdPollID, &in.AccountID)
		if getErr == nil {
			out.Poll = enrichedPoll
		}
	}
	return out, nil
}

// getPollEnriched loads a poll by ID, enforces visibility via the parent status, and attaches options, counts, voted, own_votes.
func (svc *statusService) getPollEnriched(ctx context.Context, pollID string, viewerAccountID *string) (*EnrichedPoll, error) {
	poll, err := svc.store.GetPollByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("getPollEnriched: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("GetPollByID: %w", err)
	}
	st, err := svc.store.GetStatusByID(ctx, poll.StatusID)
	if err != nil {
		return nil, fmt.Errorf("GetPoll GetStatusByID: %w", err)
	}
	ok, err := svc.canViewStatus(ctx, st, viewerAccountID)
	if err != nil {
		return nil, fmt.Errorf("GetPoll canViewStatus: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("GetPoll: %w", domain.ErrNotFound)
	}
	opts, err := svc.store.ListPollOptions(ctx, pollID)
	if err != nil {
		return nil, fmt.Errorf("GetPoll ListPollOptions: %w", err)
	}
	counts, err := svc.store.GetVoteCountsByPoll(ctx, pollID)
	if err != nil {
		return nil, fmt.Errorf("GetPoll GetVoteCountsByPoll: %w", err)
	}
	optionsWithCount := make([]PollOptionWithCount, 0, len(opts))
	for _, o := range opts {
		c := 0
		if n, ok := counts[o.ID]; ok {
			c = n
		}
		optionsWithCount = append(optionsWithCount, PollOptionWithCount{Title: o.Title, VotesCount: c})
	}
	var voted bool
	var ownVotes []int
	if viewerAccountID != nil && *viewerAccountID != "" {
		voted, err = svc.store.HasVotedOnPoll(ctx, pollID, *viewerAccountID)
		if err != nil {
			return nil, fmt.Errorf("GetPoll HasVotedOnPoll: %w", err)
		}
		ownIDs, err := svc.store.GetOwnVoteOptionIDs(ctx, pollID, *viewerAccountID)
		if err != nil {
			return nil, fmt.Errorf("GetPoll GetOwnVoteOptionIDs: %w", err)
		}
		ownVotes = make([]int, 0, len(ownIDs))
		for _, id := range ownIDs {
			for i := range opts {
				if opts[i].ID == id {
					ownVotes = append(ownVotes, i)
					break
				}
			}
		}
	}
	return &EnrichedPoll{
		Poll:     *poll,
		Options:  optionsWithCount,
		Voted:    voted,
		OwnVotes: ownVotes,
	}, nil
}

// GetPoll returns an enriched poll by ID. Returns ErrNotFound if the poll does not exist or the viewer cannot see the parent status.
func (svc *statusService) GetPoll(ctx context.Context, pollID string, viewerAccountID *string) (*EnrichedPoll, error) {
	enriched, err := svc.getPollEnriched(ctx, pollID, viewerAccountID)
	if err != nil {
		return nil, fmt.Errorf("GetPoll: %w", err)
	}
	return enriched, nil
}

// RecordVote records the viewer's vote on a poll (replacing any existing vote). Returns the updated EnrichedPoll.
func (svc *statusService) RecordVote(ctx context.Context, pollID, accountID string, optionIndices []int) (*EnrichedPoll, error) {
	poll, err := svc.store.GetPollByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("RecordVote: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("RecordVote: %w", err)
	}
	if poll.ExpiresAt != nil && poll.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("RecordVote: %w", domain.ErrUnprocessable)
	}
	st, err := svc.store.GetStatusByID(ctx, poll.StatusID)
	if err != nil {
		return nil, fmt.Errorf("RecordVote: %w", err)
	}
	viewerID := &accountID
	ok, err := svc.canViewStatus(ctx, st, viewerID)
	if err != nil {
		return nil, fmt.Errorf("RecordVote: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("RecordVote: %w", domain.ErrNotFound)
	}
	opts, err := svc.store.ListPollOptions(ctx, pollID)
	if err != nil {
		return nil, fmt.Errorf("RecordVote: %w", err)
	}
	if len(optionIndices) == 0 {
		return nil, fmt.Errorf("RecordVote: %w", domain.ErrValidation)
	}
	if !poll.Multiple && len(optionIndices) > 1 {
		return nil, fmt.Errorf("RecordVote: %w", domain.ErrValidation)
	}
	for _, idx := range optionIndices {
		if idx < 0 || idx >= len(opts) {
			return nil, fmt.Errorf("RecordVote: %w", domain.ErrValidation)
		}
	}
	if err := svc.store.DeletePollVotesByAccount(ctx, pollID, accountID); err != nil {
		return nil, fmt.Errorf("RecordVote: %w", err)
	}
	for _, idx := range optionIndices {
		optionID := opts[idx].ID
		voteID := uid.New()
		if err := svc.store.CreatePollVote(ctx, voteID, pollID, accountID, optionID); err != nil {
			return nil, fmt.Errorf("RecordVote: %w", err)
		}
	}
	return svc.getPollEnriched(ctx, pollID, &accountID)
}

func resolveVisibilityService(reqVis, defaultVis string) string {
	if reqVis != "" {
		switch reqVis {
		case domain.VisibilityPublic, domain.VisibilityUnlisted, domain.VisibilityPrivate, domain.VisibilityDirect:
			return reqVis
		}
	}
	if defaultVis != "" {
		switch defaultVis {
		case domain.VisibilityPublic, domain.VisibilityUnlisted, domain.VisibilityPrivate, domain.VisibilityDirect:
			return defaultVis
		}
	}
	return domain.VisibilityPublic
}

func (svc *statusService) mentionResolver(ctx context.Context) MentionResolver {
	return func(username string, domain *string) *domain.Account {
		if domain == nil || *domain == "" {
			a, _ := svc.store.GetLocalAccountByUsername(ctx, username)
			return a
		}
		a, _ := svc.store.GetRemoteAccountByUsername(ctx, username, domain)
		return a
	}
}

func createStatusWithContentTx(
	ctx context.Context,
	tx store.Store,
	accountID, _, statusID, statusURI, visibility, text, content, contentWarning, language string,
	sensitive bool,
	renderResult RenderResult,
	inReplyToID, inReplyToAccountID *string,
	quotedStatusID *string,
	quoteApprovalPolicy string,
	quotedAuthorID *string,
	_ []string, // mediaIDs are attached by caller after CreateStatus
) (*domain.Status, error) {
	var textPtr, contentPtr *string
	if text != "" {
		textPtr = &text
		contentPtr = &content
	}
	st, err := tx.CreateStatus(ctx, store.CreateStatusInput{
		ID:                  statusID,
		URI:                 statusURI,
		AccountID:           accountID,
		Text:                textPtr,
		Content:             contentPtr,
		ContentWarning:      &contentWarning,
		Visibility:          visibility,
		Language:            &language,
		InReplyToID:         inReplyToID,
		InReplyToAccountID:  inReplyToAccountID,
		QuotedStatusID:      quotedStatusID,
		QuoteApprovalPolicy: quoteApprovalPolicy,
		Sensitive:           sensitive,
		Local:               true,
		APID:                statusURI,
		ApRaw:               nil,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateStatus: %w", err)
	}
	if quotedStatusID != nil && *quotedStatusID != "" {
		if err := tx.CreateQuoteApproval(ctx, statusID, *quotedStatusID); err != nil {
			return nil, fmt.Errorf("CreateQuoteApproval: %w", err)
		}
		if err := tx.IncrementQuotesCount(ctx, *quotedStatusID); err != nil {
			return nil, fmt.Errorf("IncrementQuotesCount: %w", err)
		}
		if quotedAuthorID != nil && *quotedAuthorID != accountID {
			notifID := uid.New()
			_, err = tx.CreateNotification(ctx, store.CreateNotificationInput{
				ID:        notifID,
				AccountID: *quotedAuthorID,
				FromID:    accountID,
				Type:      domain.NotificationTypeQuote,
				StatusID:  &statusID,
			})
			if err != nil {
				return nil, fmt.Errorf("CreateNotification quote: %w", err)
			}
		}
	}
	for _, m := range renderResult.Mentions {
		if err := tx.CreateStatusMention(ctx, statusID, m.AccountID); err != nil {
			return nil, fmt.Errorf("CreateStatusMention: %w", err)
		}
	}
	var hashtagIDs []string
	for _, tagName := range renderResult.Tags {
		ht, err := tx.GetOrCreateHashtag(ctx, tagName)
		if err != nil {
			return nil, fmt.Errorf("GetOrCreateHashtag(%s): %w", tagName, err)
		}
		hashtagIDs = append(hashtagIDs, ht.ID)
	}
	if len(hashtagIDs) > 0 {
		if err := tx.AttachHashtagsToStatus(ctx, statusID, hashtagIDs); err != nil {
			return nil, fmt.Errorf("AttachHashtagsToStatus: %w", err)
		}
	}
	if err := tx.IncrementStatusesCount(ctx, accountID); err != nil {
		return nil, fmt.Errorf("IncrementStatusesCount: %w", err)
	}
	if err := tx.UpdateAccountLastStatusAt(ctx, accountID); err != nil {
		return nil, fmt.Errorf("UpdateAccountLastStatusAt: %w", err)
	}
	root, err := tx.GetConversationRoot(ctx, statusID)
	if err != nil {
		return nil, fmt.Errorf("GetConversationRoot: %w", err)
	}
	for _, m := range renderResult.Mentions {
		mentioned, _ := tx.GetAccountByID(ctx, m.AccountID)
		if mentioned == nil || (mentioned.Domain != nil && *mentioned.Domain != "") {
			continue
		}
		muted, err := tx.IsConversationMuted(ctx, mentioned.ID, root)
		if err != nil {
			return nil, fmt.Errorf("IsConversationMuted: %w", err)
		}
		if muted {
			continue
		}
		notifID := uid.New()
		_, err = tx.CreateNotification(ctx, store.CreateNotificationInput{
			ID:        notifID,
			AccountID: mentioned.ID,
			FromID:    accountID,
			Type:      domain.NotificationTypeMention,
			StatusID:  &statusID,
		})
		if err != nil {
			return nil, fmt.Errorf("CreateNotification: %w", err)
		}
	}
	return st, nil
}

// Delete soft-deletes the status and decrements the account's statuses count atomically.
func (svc *statusService) Delete(ctx context.Context, id string) error {
	st, err := svc.store.GetStatusByID(ctx, id)
	if err != nil {
		return fmt.Errorf("Delete(%s): %w", id, err)
	}
	var hashtagNames []string
	tags, _ := svc.store.GetStatusHashtags(ctx, id)
	for _, t := range tags {
		hashtagNames = append(hashtagNames, t.Name)
	}
	var mentionedAccountIDs []string
	if st.Visibility == domain.VisibilityDirect {
		mentionedAccountIDs, _ = svc.store.GetStatusMentionAccountIDs(ctx, id)
	}
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.SoftDeleteStatus(ctx, id); err != nil {
			return fmt.Errorf("SoftDeleteStatus: %w", err)
		}
		return tx.DecrementStatusesCount(ctx, st.AccountID)
	})
	if err != nil {
		return fmt.Errorf("Delete(%s): %w", id, err)
	}
	if err := svc.fed.DeleteStatus(ctx, st); err != nil && svc.logger != nil {
		svc.logger.WarnContext(ctx, "federation publish failed after status delete", slog.Any("error", err), slog.String("status_id", st.ID))
	}
	svc.eventBus.PublishStatusDeleted(ctx, events.StatusDeletedEvent{
		StatusID:            st.ID,
		AccountID:           st.AccountID,
		Visibility:          st.Visibility,
		Local:               st.Local,
		HashtagNames:        hashtagNames,
		MentionedAccountIDs: mentionedAccountIDs,
	})
	return nil
}

// Reblog creates a boost status for the given status. Returns the new boost status (with nested reblog). Errors: ErrNotFound (cannot see status), ErrForbidden (private/direct), ErrConflict (already reblogged).
func (svc *statusService) Reblog(ctx context.Context, accountID, username, statusID string) (EnrichedStatus, error) {
	orig, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Reblog GetStatusByID: %w", err)
	}
	if orig.DeletedAt != nil {
		return EnrichedStatus{}, fmt.Errorf("Reblog: %w", domain.ErrNotFound)
	}
	viewerID := &accountID
	ok, err := svc.canViewStatus(ctx, orig, viewerID)
	if err != nil {
		return EnrichedStatus{}, err
	}
	if !ok {
		return EnrichedStatus{}, fmt.Errorf("Reblog: %w", domain.ErrNotFound)
	}
	if orig.Visibility != domain.VisibilityPublic && orig.Visibility != domain.VisibilityUnlisted {
		return EnrichedStatus{}, fmt.Errorf("Reblog: %w", domain.ErrForbidden)
	}
	existing, _ := svc.store.GetReblogByAccountAndTarget(ctx, accountID, statusID)
	if existing != nil {
		return EnrichedStatus{}, fmt.Errorf("Reblog: %w", domain.ErrConflict)
	}
	boostID := uid.New()
	boostURI := fmt.Sprintf("%s/users/%s/statuses/%s", svc.instanceBaseURL, username, boostID)
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		_, err := tx.CreateStatus(ctx, store.CreateStatusInput{
			ID:                  boostID,
			URI:                 boostURI,
			AccountID:           accountID,
			Text:                nil,
			Content:             nil,
			Visibility:          orig.Visibility,
			Language:            nil,
			InReplyToID:         nil,
			ReblogOfID:          &statusID,
			Sensitive:           orig.Sensitive,
			Local:               true,
			APID:                boostURI,
			ApRaw:               nil,
			QuoteApprovalPolicy: domain.QuotePolicyPublic,
		})
		if err != nil {
			return fmt.Errorf("CreateStatus: %w", err)
		}
		if err := tx.IncrementReblogsCount(ctx, statusID); err != nil {
			return fmt.Errorf("IncrementReblogsCount: %w", err)
		}
		origAuthor, _ := tx.GetAccountByID(ctx, orig.AccountID)
		if origAuthor != nil && (origAuthor.Domain == nil || *origAuthor.Domain == "") {
			_, _ = tx.CreateNotification(ctx, store.CreateNotificationInput{
				ID:        uid.New(),
				AccountID: orig.AccountID,
				FromID:    accountID,
				Type:      domain.NotificationTypeReblog,
				StatusID:  &statusID,
			})
		}
		return nil
	})
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Reblog tx: %w", err)
	}
	origAuthor, _ := svc.store.GetAccountByID(ctx, orig.AccountID)
	rebloggerAccount, _ := svc.store.GetAccountByID(ctx, accountID)
	if origAuthor != nil && (origAuthor.Domain == nil || *origAuthor.Domain == "") && rebloggerAccount != nil {
		notifs, _ := svc.store.ListNotifications(ctx, orig.AccountID, nil, 1)
		if len(notifs) > 0 && notifs[0].FromID == accountID && notifs[0].Type == domain.NotificationTypeReblog && notifs[0].StatusID != nil && *notifs[0].StatusID == statusID {
			svc.eventBus.PublishNotificationCreated(ctx, events.NotificationCreatedEvent{
				RecipientAccountID: orig.AccountID,
				Notification:       &notifs[0],
				FromAccount:        rebloggerAccount,
				StatusID:           &statusID,
			})
		}
	}
	return svc.GetByIDEnriched(ctx, boostID, &accountID)
}

// Unreblog removes the viewer's reblog of the given status. Returns the original status (no nested reblog).
func (svc *statusService) Unreblog(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	boost, err := svc.store.GetReblogByAccountAndTarget(ctx, accountID, statusID)
	if err != nil || boost == nil {
		result, getErr := svc.GetByIDEnriched(ctx, statusID, &accountID)
		if getErr != nil {
			return EnrichedStatus{}, fmt.Errorf("Unreblog GetByIDEnriched: %w", getErr)
		}
		return result, nil
	}
	if err := svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.SoftDeleteStatus(ctx, boost.ID); err != nil {
			return fmt.Errorf("SoftDeleteStatus: %w", err)
		}
		if err := tx.DecrementReblogsCount(ctx, statusID); err != nil {
			return fmt.Errorf("DecrementReblogsCount: %w", err)
		}
		return nil
	}); err != nil {
		return EnrichedStatus{}, fmt.Errorf("Unreblog: %w", err)
	}
	return svc.GetByIDEnriched(ctx, statusID, &accountID)
}

// Favourite adds a favourite for the viewer on the status. Returns the status with favourited true.
func (svc *statusService) Favourite(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil || st.DeletedAt != nil {
		return EnrichedStatus{}, fmt.Errorf("Favourite: %w", domain.ErrNotFound)
	}
	viewerID := &accountID
	ok, err := svc.canViewStatus(ctx, st, viewerID)
	if err != nil {
		return EnrichedStatus{}, err
	}
	if !ok {
		return EnrichedStatus{}, fmt.Errorf("Favourite: %w", domain.ErrNotFound)
	}
	_, err = svc.store.CreateFavourite(ctx, store.CreateFavouriteInput{
		ID:        uid.New(),
		AccountID: accountID,
		StatusID:  statusID,
		APID:      nil,
	})
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("CreateFavourite: %w", err)
	}
	if err := svc.store.IncrementFavouritesCount(ctx, statusID); err != nil {
		return EnrichedStatus{}, fmt.Errorf("IncrementFavouritesCount: %w", err)
	}
	author, _ := svc.store.GetAccountByID(ctx, st.AccountID)
	var createdNotif *domain.Notification
	if author != nil && (author.Domain == nil || *author.Domain == "") {
		createdNotif, _ = svc.store.CreateNotification(ctx, store.CreateNotificationInput{
			ID:        uid.New(),
			AccountID: st.AccountID,
			FromID:    accountID,
			Type:      domain.NotificationTypeFavourite,
			StatusID:  &statusID,
		})
	}
	if createdNotif != nil {
		favouriterAccount, _ := svc.store.GetAccountByID(ctx, accountID)
		if favouriterAccount != nil {
			svc.eventBus.PublishNotificationCreated(ctx, events.NotificationCreatedEvent{
				RecipientAccountID: st.AccountID,
				Notification:       createdNotif,
				FromAccount:        favouriterAccount,
				StatusID:           &statusID,
			})
		}
	}
	return svc.GetByIDEnriched(ctx, statusID, &accountID)
}

// Unfavourite removes the viewer's favourite. Returns the status with favourited false.
func (svc *statusService) Unfavourite(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	_ = svc.store.DeleteFavourite(ctx, accountID, statusID)
	_ = svc.store.DecrementFavouritesCount(ctx, statusID)
	return svc.GetByIDEnriched(ctx, statusID, &accountID)
}

// Bookmark adds the status to the account's bookmarks. Idempotent if already bookmarked.
func (svc *statusService) Bookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil || st.DeletedAt != nil {
		return EnrichedStatus{}, fmt.Errorf("Bookmark: %w", domain.ErrNotFound)
	}
	viewerID := &accountID
	ok, err := svc.canViewStatus(ctx, st, viewerID)
	if err != nil {
		return EnrichedStatus{}, err
	}
	if !ok {
		return EnrichedStatus{}, fmt.Errorf("Bookmark: %w", domain.ErrNotFound)
	}
	err = svc.store.CreateBookmark(ctx, store.CreateBookmarkInput{
		ID:        uid.New(),
		AccountID: accountID,
		StatusID:  statusID,
	})
	if err != nil && !errors.Is(err, domain.ErrConflict) {
		return EnrichedStatus{}, fmt.Errorf("CreateBookmark: %w", err)
	}
	return svc.GetByIDEnriched(ctx, statusID, &accountID)
}

// Unbookmark removes the status from the account's bookmarks.
func (svc *statusService) Unbookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	_ = svc.store.DeleteBookmark(ctx, accountID, statusID)
	return svc.GetByIDEnriched(ctx, statusID, &accountID)
}

// IsBookmarked returns whether the account has bookmarked the status.
func (svc *statusService) IsBookmarked(ctx context.Context, accountID, statusID string) (bool, error) {
	ok, err := svc.store.IsBookmarked(ctx, accountID, statusID)
	if err != nil {
		return false, fmt.Errorf("IsBookmarked: %w", err)
	}
	return ok, nil
}

const maxPinsPerAccount = 5

func (svc *statusService) Pin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return EnrichedStatus{}, fmt.Errorf("Pin: %w", domain.ErrNotFound)
		}
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", err)
	}
	if st.AccountID != accountID {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", domain.ErrForbidden)
	}
	if st.Visibility != "public" && st.Visibility != "unlisted" {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", domain.ErrUnprocessable)
	}
	if st.ReblogOfID != nil && *st.ReblogOfID != "" {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", domain.ErrUnprocessable)
	}
	count, err := svc.store.CountAccountPins(ctx, accountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", err)
	}
	if count >= maxPinsPerAccount {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", domain.ErrUnprocessable)
	}
	if err := svc.store.CreateAccountPin(ctx, accountID, statusID); err != nil {
		return EnrichedStatus{}, fmt.Errorf("Pin: %w", err)
	}
	return svc.GetByIDEnriched(ctx, statusID, &accountID)
}

func (svc *statusService) Unpin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return EnrichedStatus{}, fmt.Errorf("Unpin: %w", domain.ErrNotFound)
		}
		return EnrichedStatus{}, fmt.Errorf("Unpin: %w", err)
	}
	if st.AccountID != accountID {
		return EnrichedStatus{}, fmt.Errorf("Unpin: %w", domain.ErrForbidden)
	}
	if err := svc.store.DeleteAccountPin(ctx, accountID, statusID); err != nil {
		return EnrichedStatus{}, fmt.Errorf("Unpin: %w", err)
	}
	return svc.GetByIDEnriched(ctx, statusID, &accountID)
}

func (svc *statusService) ListPinnedStatusIDs(ctx context.Context, accountID string) ([]string, error) {
	ids, err := svc.store.ListPinnedStatusIDs(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ListPinnedStatusIDs: %w", err)
	}
	return ids, nil
}

// UpdateStatusFromAPI updates a status by its owner: snapshots current to status_edits, re-renders content, updates mentions/hashtags, persists, and federates Update(Note).
func (svc *statusService) UpdateStatusFromAPI(ctx context.Context, accountID, statusID string, text, spoilerText string, sensitive bool) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return EnrichedStatus{}, fmt.Errorf("UpdateStatusFromAPI: %w", domain.ErrNotFound)
		}
		return EnrichedStatus{}, fmt.Errorf("UpdateStatusFromAPI: %w", err)
	}
	if st.AccountID != accountID {
		return EnrichedStatus{}, fmt.Errorf("UpdateStatusFromAPI: %w", domain.ErrForbidden)
	}
	if st.ReblogOfID != nil && *st.ReblogOfID != "" {
		return EnrichedStatus{}, fmt.Errorf("UpdateStatusFromAPI: %w", domain.ErrUnprocessable)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return EnrichedStatus{}, fmt.Errorf("UpdateStatusFromAPI: %w", domain.ErrValidation)
	}
	if CountStatusCharacters(text) > svc.maxStatusChars {
		return EnrichedStatus{}, fmt.Errorf("UpdateStatusFromAPI: %w", domain.ErrValidation)
	}
	resolver := svc.mentionResolver(ctx)
	renderResult, err := Render(text, svc.instanceDomain, resolver)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("UpdateStatusFromAPI Render: %w", err)
	}
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.CreateStatusEdit(ctx, store.CreateStatusEditInput{
			ID:             uid.New(),
			StatusID:       statusID,
			AccountID:      accountID,
			Text:           st.Text,
			Content:        st.Content,
			ContentWarning: st.ContentWarning,
			Sensitive:      st.Sensitive,
		}); err != nil {
			return fmt.Errorf("CreateStatusEdit: %w", err)
		}
		contentWarningPtr := &spoilerText
		if spoilerText == "" {
			contentWarningPtr = nil
		}
		if err := tx.UpdateStatus(ctx, store.UpdateStatusInput{
			ID:             statusID,
			Text:           &text,
			Content:        &renderResult.HTML,
			ContentWarning: contentWarningPtr,
			Sensitive:      sensitive,
		}); err != nil {
			return fmt.Errorf("UpdateStatus: %w", err)
		}
		if err := tx.DeleteStatusMentions(ctx, statusID); err != nil {
			return fmt.Errorf("DeleteStatusMentions: %w", err)
		}
		for _, m := range renderResult.Mentions {
			if err := tx.CreateStatusMention(ctx, statusID, m.AccountID); err != nil {
				return fmt.Errorf("CreateStatusMention: %w", err)
			}
		}
		if err := tx.DeleteStatusHashtags(ctx, statusID); err != nil {
			return fmt.Errorf("DeleteStatusHashtags: %w", err)
		}
		var hashtagIDs []string
		for _, tagName := range renderResult.Tags {
			ht, err := tx.GetOrCreateHashtag(ctx, tagName)
			if err != nil {
				return fmt.Errorf("GetOrCreateHashtag: %w", err)
			}
			hashtagIDs = append(hashtagIDs, ht.ID)
		}
		if len(hashtagIDs) > 0 {
			if err := tx.AttachHashtagsToStatus(ctx, statusID, hashtagIDs); err != nil {
				return fmt.Errorf("AttachHashtagsToStatus: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("UpdateStatusFromAPI: %w", err)
	}
	updated, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("UpdateStatusFromAPI GetStatusByID: %w", err)
	}
	if err := svc.fed.PublishUpdate(ctx, updated); err != nil && svc.logger != nil {
		svc.logger.WarnContext(ctx, "federation publish update failed", slog.Any("error", err), slog.String("status_id", statusID))
	}
	quotes, err := svc.store.ListQuotesOfStatus(ctx, statusID, nil, 500)
	if err == nil {
		for i := range quotes {
			quotingAuthorID := quotes[i].AccountID
			if quotingAuthorID == accountID {
				continue
			}
			quotingStatusID := quotes[i].ID
			_, _ = svc.store.CreateNotification(ctx, store.CreateNotificationInput{
				ID:        uid.New(),
				AccountID: quotingAuthorID,
				FromID:    accountID,
				Type:      domain.NotificationTypeQuotedUpdate,
				StatusID:  &quotingStatusID,
			})
		}
	}
	return svc.GetByIDEnriched(ctx, statusID, &accountID)
}

// GetStatusHistory returns edit history for a status. Applies same visibility as GET status.
func (svc *statusService) GetStatusHistory(ctx context.Context, statusID string, viewerAccountID *string) ([]domain.StatusEdit, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("GetStatusHistory: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("GetStatusHistory: %w", err)
	}
	ok, err := svc.canViewStatus(ctx, st, viewerAccountID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("GetStatusHistory: %w", domain.ErrNotFound)
	}
	edits, err := svc.store.ListStatusEdits(ctx, statusID)
	if err != nil {
		return nil, fmt.Errorf("GetStatusHistory: %w", err)
	}
	return edits, nil
}

// GetStatusSource returns the plain text and spoiler for a status. Applies same visibility as GET status.
func (svc *statusService) GetStatusSource(ctx context.Context, statusID string, viewerAccountID *string) (text, spoilerText string, err error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", "", fmt.Errorf("GetStatusSource: %w", domain.ErrNotFound)
		}
		return "", "", fmt.Errorf("GetStatusSource: %w", err)
	}
	ok, err := svc.canViewStatus(ctx, st, viewerAccountID)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", fmt.Errorf("GetStatusSource: %w", domain.ErrNotFound)
	}
	t := ""
	if st.Text != nil {
		t = *st.Text
	}
	spoiler := ""
	if st.ContentWarning != nil {
		spoiler = *st.ContentWarning
	}
	return t, spoiler, nil
}

func (svc *statusService) CreateScheduledStatus(ctx context.Context, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error) {
	now := time.Now()
	if !scheduledAt.After(now) {
		return nil, fmt.Errorf("CreateScheduledStatus scheduled_at must be in the future: %w", domain.ErrValidation)
	}
	var p domain.ScheduledStatusParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("CreateScheduledStatus invalid params: %w", domain.ErrValidation)
	}
	id := uid.New()
	s, err := svc.store.CreateScheduledStatus(ctx, store.CreateScheduledStatusInput{
		ID:          id,
		AccountID:   accountID,
		Params:      params,
		ScheduledAt: scheduledAt,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateScheduledStatus: %w", err)
	}
	return s, nil
}

func (svc *statusService) GetScheduledStatus(ctx context.Context, id, accountID string) (*domain.ScheduledStatus, error) {
	s, err := svc.store.GetScheduledStatusByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("GetScheduledStatus: %w", err)
		}
		return nil, fmt.Errorf("GetScheduledStatus: %w", err)
	}
	if s.AccountID != accountID {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func (svc *statusService) ListScheduledStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.ScheduledStatus, error) {
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	list, err := svc.store.ListScheduledStatuses(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, fmt.Errorf("ListScheduledStatuses: %w", err)
	}
	return list, nil
}

func (svc *statusService) UpdateScheduledStatus(ctx context.Context, id, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error) {
	s, err := svc.store.GetScheduledStatusByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("UpdateScheduledStatus: %w", err)
		}
		return nil, fmt.Errorf("UpdateScheduledStatus: %w", err)
	}
	if s.AccountID != accountID {
		return nil, domain.ErrNotFound
	}
	var p domain.ScheduledStatusParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("UpdateScheduledStatus invalid params: %w", domain.ErrValidation)
	}
	now := time.Now()
	if !scheduledAt.After(now) {
		return nil, fmt.Errorf("UpdateScheduledStatus scheduled_at must be in the future: %w", domain.ErrValidation)
	}
	updated, err := svc.store.UpdateScheduledStatus(ctx, store.UpdateScheduledStatusInput{
		ID:          id,
		Params:      params,
		ScheduledAt: scheduledAt,
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateScheduledStatus: %w", err)
	}
	return updated, nil
}

func (svc *statusService) DeleteScheduledStatus(ctx context.Context, id, accountID string) error {
	s, err := svc.store.GetScheduledStatusByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("DeleteScheduledStatus: %w", err)
		}
		return fmt.Errorf("DeleteScheduledStatus: %w", err)
	}
	if s.AccountID != accountID {
		return domain.ErrNotFound
	}
	if err := svc.store.DeleteScheduledStatus(ctx, id); err != nil {
		return fmt.Errorf("DeleteScheduledStatus: %w", err)
	}
	return nil
}

func (svc *statusService) PublishScheduled(ctx context.Context, scheduledID string) error {
	s, err := svc.store.GetScheduledStatusByID(ctx, scheduledID)
	if err != nil {
		return fmt.Errorf("PublishScheduled GetScheduledStatusByID: %w", err)
	}
	var p domain.ScheduledStatusParams
	if err := json.Unmarshal(s.Params, &p); err != nil {
		return fmt.Errorf("PublishScheduled invalid params: %w", err)
	}
	acc, err := svc.store.GetAccountByID(ctx, s.AccountID)
	if err != nil {
		return fmt.Errorf("PublishScheduled GetAccountByID: %w", err)
	}
	user, _ := svc.store.GetUserByAccountID(ctx, s.AccountID)
	defaultVisibility := ""
	if user != nil {
		defaultVisibility = user.DefaultPrivacy
	}
	var inReplyToID *string
	if p.InReplyToID != "" {
		inReplyToID = &p.InReplyToID
	}
	_, err = svc.CreateWithContent(ctx, CreateWithContentInput{
		AccountID:         s.AccountID,
		Username:          acc.Username,
		Text:              p.Text,
		Visibility:        p.Visibility,
		DefaultVisibility: defaultVisibility,
		ContentWarning:    p.SpoilerText,
		Language:          p.Language,
		Sensitive:         p.Sensitive,
		InReplyToID:       inReplyToID,
		MediaIDs:          p.MediaIDs,
	})
	if err != nil {
		return fmt.Errorf("PublishScheduled CreateWithContent: %w", err)
	}
	if err := svc.store.DeleteScheduledStatus(ctx, scheduledID); err != nil {
		return fmt.Errorf("PublishScheduled DeleteScheduledStatus: %w", err)
	}
	return nil
}

// ContextResult holds ancestors and descendants for a status thread.
type ContextResult struct {
	Ancestors   []domain.Status
	Descendants []domain.Status
}

// GetContext returns the reply-chain ancestors and descendants for the status. Visibility filtering is applied.
func (svc *statusService) GetContext(ctx context.Context, statusID string, viewerAccountID *string) (ContextResult, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		return ContextResult{}, fmt.Errorf("GetContext GetStatusByID: %w", err)
	}
	if st.DeletedAt != nil {
		return ContextResult{}, fmt.Errorf("GetContext(%s): %w", statusID, domain.ErrNotFound)
	}
	ok, err := svc.canViewStatus(ctx, st, viewerAccountID)
	if err != nil {
		return ContextResult{}, err
	}
	if !ok {
		return ContextResult{}, fmt.Errorf("GetContext(%s): %w", statusID, domain.ErrNotFound)
	}
	ancestors, err := svc.store.GetStatusAncestors(ctx, statusID)
	if err != nil {
		return ContextResult{}, fmt.Errorf("GetStatusAncestors: %w", err)
	}
	descendants, err := svc.store.GetStatusDescendants(ctx, statusID)
	if err != nil {
		return ContextResult{}, fmt.Errorf("GetStatusDescendants: %w", err)
	}
	filteredAncestors := make([]domain.Status, 0, len(ancestors))
	for i := range ancestors {
		ok, err := svc.canViewStatus(ctx, &ancestors[i], viewerAccountID)
		if err != nil {
			return ContextResult{}, err
		}
		if ok {
			filteredAncestors = append(filteredAncestors, ancestors[i])
		}
	}
	filteredDescendants := make([]domain.Status, 0, len(descendants))
	for i := range descendants {
		ok, err := svc.canViewStatus(ctx, &descendants[i], viewerAccountID)
		if err != nil {
			return ContextResult{}, err
		}
		if ok {
			filteredDescendants = append(filteredDescendants, descendants[i])
		}
	}
	return ContextResult{Ancestors: filteredAncestors, Descendants: filteredDescendants}, nil
}

// GetFavouritedBy returns the accounts that favourited the status (paginated).
// Returns ErrNotFound if the viewer cannot see the status.
func (svc *statusService) GetFavouritedBy(ctx context.Context, statusID string, viewerAccountID *string, maxID *string, limit int) ([]*domain.Account, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		return nil, fmt.Errorf("GetFavouritedBy GetStatusByID: %w", err)
	}
	if st.DeletedAt != nil {
		return nil, fmt.Errorf("GetFavouritedBy(%s): %w", statusID, domain.ErrNotFound)
	}
	ok, err := svc.canViewStatus(ctx, st, viewerAccountID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("GetFavouritedBy(%s): %w", statusID, domain.ErrNotFound)
	}
	accounts, err := svc.store.GetStatusFavouritedBy(ctx, statusID, maxID, limit)
	if err != nil {
		return nil, fmt.Errorf("GetStatusFavouritedBy: %w", err)
	}
	out := make([]*domain.Account, 0, len(accounts))
	for i := range accounts {
		out = append(out, &accounts[i])
	}
	return out, nil
}

// GetRebloggedBy returns the accounts that reblogged the status (paginated).
// Returns ErrNotFound if the viewer cannot see the status.
func (svc *statusService) GetRebloggedBy(ctx context.Context, statusID string, viewerAccountID *string, maxID *string, limit int) ([]*domain.Account, error) {
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		return nil, fmt.Errorf("GetRebloggedBy GetStatusByID: %w", err)
	}
	if st.DeletedAt != nil {
		return nil, fmt.Errorf("GetRebloggedBy(%s): %w", statusID, domain.ErrNotFound)
	}
	ok, err := svc.canViewStatus(ctx, st, viewerAccountID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("GetRebloggedBy(%s): %w", statusID, domain.ErrNotFound)
	}
	accounts, err := svc.store.GetRebloggedBy(ctx, statusID, maxID, limit)
	if err != nil {
		return nil, fmt.Errorf("GetRebloggedBy: %w", err)
	}
	out := make([]*domain.Account, 0, len(accounts))
	for i := range accounts {
		out = append(out, &accounts[i])
	}
	return out, nil
}

// MuteConversation mutes the conversation (thread) containing the given status for the account.
func (svc *statusService) MuteConversation(ctx context.Context, accountID, statusID string) error {
	root, err := svc.store.GetConversationRoot(ctx, statusID)
	if err != nil {
		return fmt.Errorf("MuteConversation GetConversationRoot: %w", err)
	}
	if err := svc.store.CreateConversationMute(ctx, accountID, root); err != nil {
		return fmt.Errorf("CreateConversationMute: %w", err)
	}
	return nil
}

// UnmuteConversation unmutes the conversation containing the given status for the account.
func (svc *statusService) UnmuteConversation(ctx context.Context, accountID, statusID string) error {
	root, err := svc.store.GetConversationRoot(ctx, statusID)
	if err != nil {
		return fmt.Errorf("UnmuteConversation GetConversationRoot: %w", err)
	}
	if err := svc.store.DeleteConversationMute(ctx, accountID, root); err != nil {
		return fmt.Errorf("DeleteConversationMute: %w", err)
	}
	return nil
}

// GetConversationRoot returns the root status ID of the conversation (thread) containing the given status.
func (svc *statusService) GetConversationRoot(ctx context.Context, statusID string) (string, error) {
	root, err := svc.store.GetConversationRoot(ctx, statusID)
	if err != nil {
		return "", fmt.Errorf("GetConversationRoot: %w", err)
	}
	return root, nil
}

// IsConversationMutedForViewer returns whether the viewer has muted the conversation containing the given status.
func (svc *statusService) IsConversationMutedForViewer(ctx context.Context, viewerAccountID, statusID string) (bool, error) {
	root, err := svc.store.GetConversationRoot(ctx, statusID)
	if err != nil {
		return false, fmt.Errorf("GetConversationRoot: %w", err)
	}
	ok, err := svc.store.IsConversationMuted(ctx, viewerAccountID, root)
	if err != nil {
		return false, fmt.Errorf("IsConversationMuted: %w", err)
	}
	return ok, nil
}

// ListMutedConversationIDs returns the list of conversation (root) IDs the account has muted.
func (svc *statusService) ListMutedConversationIDs(ctx context.Context, accountID string) ([]string, error) {
	ids, err := svc.store.ListMutedConversationIDs(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ListMutedConversationIDs: %w", err)
	}
	return ids, nil
}

// GetQuoteApproval returns the quote approval record for a quoting status, or ErrNotFound.
func (svc *statusService) GetQuoteApproval(ctx context.Context, quotingStatusID string) (*domain.QuoteApprovalRecord, error) {
	rec, err := svc.store.GetQuoteApproval(ctx, quotingStatusID)
	if err != nil {
		return nil, fmt.Errorf("GetQuoteApproval(%s): %w", quotingStatusID, err)
	}
	return rec, nil
}

// UpdateQuoteApprovalPolicy updates the quote_approval_policy of a status (Mastodon-style quotes).
// Caller must be the status owner. Policy must be non-empty; use domain.QuotePolicy* constants.
func (svc *statusService) UpdateQuoteApprovalPolicy(ctx context.Context, accountID, statusID, policy string) error {
	if strings.TrimSpace(policy) == "" {
		return fmt.Errorf("UpdateQuoteApprovalPolicy: %w", domain.ErrValidation)
	}
	st, err := svc.store.GetStatusByID(ctx, statusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("UpdateQuoteApprovalPolicy: %w", domain.ErrNotFound)
		}
		return fmt.Errorf("UpdateQuoteApprovalPolicy: %w", err)
	}
	if st.AccountID != accountID {
		return fmt.Errorf("UpdateQuoteApprovalPolicy: %w", domain.ErrForbidden)
	}
	if st.Visibility == domain.VisibilityPrivate || st.Visibility == domain.VisibilityDirect {
		policy = domain.QuotePolicyNobody
	} else {
		switch policy {
		case domain.QuotePolicyPublic, domain.QuotePolicyFollowers, domain.QuotePolicyNobody:
			// valid
		default:
			return fmt.Errorf("UpdateQuoteApprovalPolicy: %w", domain.ErrValidation)
		}
	}
	if err := svc.store.UpdateStatusQuoteApprovalPolicy(ctx, statusID, policy); err != nil {
		return fmt.Errorf("UpdateQuoteApprovalPolicy: %w", err)
	}
	if svc.logger != nil {
		svc.logger.InfoContext(ctx, "quote approval policy updated", slog.String("status_id", statusID), slog.String("account_id", accountID), slog.String("policy", policy))
	}
	return nil
}

// ListQuotesOfStatus returns enriched statuses that quote the given status (Mastodon-style quotes, non-revoked).
// Viewer must be able to see the quoted status.
func (svc *statusService) ListQuotesOfStatus(ctx context.Context, quotedStatusID string, maxID *string, limit int, viewerAccountID *string) ([]EnrichedStatus, error) {
	quoted, err := svc.store.GetStatusByID(ctx, quotedStatusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("ListQuotesOfStatus: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("ListQuotesOfStatus: %w", err)
	}
	ok, err := svc.canViewStatus(ctx, quoted, viewerAccountID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("ListQuotesOfStatus: %w", domain.ErrNotFound)
	}
	if limit <= 0 || limit > 80 {
		limit = 20
	}
	list, err := svc.store.ListQuotesOfStatus(ctx, quotedStatusID, maxID, limit)
	if err != nil {
		return nil, fmt.Errorf("ListQuotesOfStatus: %w", err)
	}
	out := make([]EnrichedStatus, 0, len(list))
	for i := range list {
		enriched, err := svc.GetByIDEnriched(ctx, list[i].ID, viewerAccountID)
		if err != nil {
			continue
		}
		out = append(out, enriched)
	}
	return out, nil
}

// RevokeQuote revokes a quote of the given status by the quoting status (Mastodon-style quotes).
// Caller must be the author of the quoted status.
func (svc *statusService) RevokeQuote(ctx context.Context, accountID, quotedStatusID, quotingStatusID string) error {
	quoted, err := svc.store.GetStatusByID(ctx, quotedStatusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("RevokeQuote: %w", domain.ErrNotFound)
		}
		return fmt.Errorf("RevokeQuote: %w", err)
	}
	if quoted.AccountID != accountID {
		return fmt.Errorf("RevokeQuote: %w", domain.ErrForbidden)
	}
	if err := svc.store.RevokeQuote(ctx, quotedStatusID, quotingStatusID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("RevokeQuote: %w", domain.ErrNotFound)
		}
		return fmt.Errorf("RevokeQuote: %w", err)
	}
	if err := svc.store.DecrementQuotesCount(ctx, quotedStatusID); err != nil {
		return fmt.Errorf("RevokeQuote DecrementQuotesCount: %w", err)
	}
	if svc.logger != nil {
		svc.logger.InfoContext(ctx, "quote revoked", slog.String("quoted_status_id", quotedStatusID), slog.String("quoting_status_id", quotingStatusID), slog.String("account_id", accountID))
	}
	return nil
}
