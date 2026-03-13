package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// Quote-related methods implement Mastodon-style quotes: quoted_status_id, quote_approval_policy,
// quote approvals, revoke, and quoted_update notifications. See docs/mastodon-api-* for the plan.

// StatusVisibilityChecker allows callers to check if a viewer can see a status (visibility + blocks).
// TimelineService depends on this narrow interface to filter list timelines.
type StatusVisibilityChecker interface {
	CanViewStatus(ctx context.Context, st *domain.Status, viewerAccountID *string) (bool, error)
}

// StatusService handles status lookup, enrichment, and local state mutations (bookmark, pin, mute, etc.).
// Write operations with cross-service side effects (create, delete, reblog, favourite, update) live in StatusWriteService.
type StatusService interface {
	StatusVisibilityChecker
	GetByID(ctx context.Context, id string) (*domain.Status, error)
	GetByAPID(ctx context.Context, apID string) (*domain.Status, error)
	GetFavouriteByAPID(ctx context.Context, apID string) (*domain.Favourite, error)
	GetFavouriteByAccountAndStatus(ctx context.Context, accountID, statusID string) (*domain.Favourite, error)
	GetByIDEnriched(ctx context.Context, id string, viewerAccountID *string) (EnrichedStatus, error)
	GetContext(ctx context.Context, statusID string, viewerAccountID *string) (ContextResult, error)
	GetFavouritedBy(ctx context.Context, statusID string, viewerAccountID *string, maxID *string, limit int) ([]*domain.Account, error)
	GetRebloggedBy(ctx context.Context, statusID string, viewerAccountID *string, maxID *string, limit int) ([]*domain.Account, error)
	Bookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	Unbookmark(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	IsBookmarked(ctx context.Context, accountID, statusID string) (bool, error)
	Pin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	Unpin(ctx context.Context, accountID, statusID string) (EnrichedStatus, error)
	ListPinnedStatusIDs(ctx context.Context, accountID string) ([]string, error)
	GetStatusHistory(ctx context.Context, statusID string, viewerAccountID *string) ([]domain.StatusEdit, error)
	GetStatusSource(ctx context.Context, statusID string, viewerAccountID *string) (text, spoilerText string, err error)

	CreateScheduledStatus(ctx context.Context, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error)
	GetScheduledStatus(ctx context.Context, id, accountID string) (*domain.ScheduledStatus, error)
	ListScheduledStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.ScheduledStatus, error)
	UpdateScheduledStatus(ctx context.Context, id, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error)
	DeleteScheduledStatus(ctx context.Context, id, accountID string) error

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
}

type statusService struct {
	store           store.Store
	instanceBaseURL string
	instanceDomain  string
	maxStatusChars  int
}

// NewStatusService returns a StatusService that uses the given store and instance URLs.
func NewStatusService(s store.Store, instanceBaseURL, instanceDomain string, maxStatusChars int) StatusService {
	base := strings.TrimSuffix(instanceBaseURL, "/")
	return &statusService{
		store:           s,
		instanceBaseURL: base,
		instanceDomain:  instanceDomain,
		maxStatusChars:  maxStatusChars,
	}
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

// CreateRemoteStatusInput is the input for creating a remote status (e.g. from federation).
type CreateRemoteStatusInput struct {
	AccountID      string
	URI            string
	Text           *string
	Content        *string
	ContentWarning *string
	Visibility     string
	Language       *string
	InReplyToID    *string
	MediaIDs       []string // optional; attached after status is created (max 4)
	APID           string
	ApRaw          []byte
	Sensitive      bool
}

// CreateRemoteReblogInput is the input for creating a remote reblog (e.g. from federation Announce).
type CreateRemoteReblogInput struct {
	AccountID        string
	ActivityAPID     string
	ObjectStatusAPID string
	ApRaw            []byte
}

// UpdateRemoteStatusInput is the input for updating a remote status (e.g. from federation Update{Note}).
type UpdateRemoteStatusInput struct {
	Text           *string
	Content        *string
	ContentWarning *string
	Sensitive      bool
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
	card, err := svc.store.GetStatusCard(ctx, st.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return EnrichedStatus{}, fmt.Errorf("GetStatusCard: %w", err)
	}
	out := EnrichedStatus{
		Status:   st,
		Author:   author,
		Mentions: mentions,
		Tags:     tags,
		Media:    media,
		Card:     card,
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
	return nil
}
