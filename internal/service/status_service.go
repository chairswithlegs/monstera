package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
)

// StatusVisibilityChecker allows callers to check if a viewer can see a status
// (visibility, blocks, and author suspension). The author argument is required
// because suspended-author hiding is part of the visibility decision; callers
// that already loaded the author for enrichment pass it through to avoid a
// duplicate lookup.
type StatusVisibilityChecker interface {
	CanViewStatus(ctx context.Context, st *domain.Status, author *domain.Account, viewerAccountID *string) (bool, error)
}

// EnrichOpts controls which optional fields are loaded when enriching statuses.
type EnrichOpts struct {
	IncludeCard   bool
	IncludePoll   bool
	ViewerID      *string
	FilterContext domain.FilterContext // When set (e.g. domain.FilterContextHome), statuses matching a "hide" filter for this context are excluded from results. This is an optimization — clients also handle hide filters via the filtered array.
	// Authors is an optional pre-fetched lookup keyed by account ID. When a
	// status's author is present here, EnrichStatuses uses it directly instead
	// of issuing another GetAccountByID. Callers that already fetched authors
	// for the visibility check (canViewStatus) should pass them through.
	Authors map[string]*domain.Account
}

// StatusService handles status lookup, enrichment, and read-only queries.
// Write operations with side effects (create, delete, reblog, favourite, bookmark, pin, etc.) live in StatusWriteService.
type StatusService interface {
	StatusVisibilityChecker

	// Core status reads.
	GetByID(ctx context.Context, id string) (*domain.Status, error)
	GetByAPID(ctx context.Context, apID string) (*domain.Status, error)
	GetByIDEnriched(ctx context.Context, id string, viewerAccountID *string) (EnrichedStatus, error)
	GetByIDsEnriched(ctx context.Context, ids []string, viewerAccountID *string, filterContext domain.FilterContext) ([]EnrichedStatus, error)
	EnrichStatuses(ctx context.Context, statuses []*domain.Status, opts EnrichOpts) ([]EnrichedStatus, error)
	GetContext(ctx context.Context, statusID string, viewerAccountID *string) (ContextResult, error)
	GetStatusHistory(ctx context.Context, statusID string, viewerAccountID *string) ([]domain.StatusEdit, error)
	GetStatusSource(ctx context.Context, statusID string, viewerAccountID *string) (text, spoilerText string, err error)

	// Favourite reads.
	GetFavouriteByAPID(ctx context.Context, apID string) (*domain.Favourite, error)
	GetFavouriteByAccountAndStatus(ctx context.Context, accountID, statusID string) (*domain.Favourite, error)
	GetFavouritedBy(ctx context.Context, statusID string, viewerAccountID *string, maxID *string, limit int) ([]*domain.Account, error)
	GetRebloggedBy(ctx context.Context, statusID string, viewerAccountID *string, maxID *string, limit int) ([]*domain.Account, error)

	// Pin reads.
	ListPinnedStatusIDs(ctx context.Context, accountID string) ([]string, error)
	PinnedStatusesEnriched(ctx context.Context, accountID string, viewerAccountID *string) ([]EnrichedStatus, error)

	// Scheduled status reads.
	GetScheduledStatus(ctx context.Context, id, accountID string) (*domain.ScheduledStatus, error)
	ListScheduledStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.ScheduledStatus, error)

	// Poll reads.
	GetPoll(ctx context.Context, pollID string, viewerAccountID *string) (*EnrichedPoll, error)

	// Quote reads.
	GetQuoteApproval(ctx context.Context, quotingStatusID string) (*domain.QuoteApprovalRecord, error)
	ListQuotesOfStatus(ctx context.Context, quotedStatusID string, maxID *string, limit int, viewerAccountID *string) ([]EnrichedStatus, error)

	// Conversation checks.
	IsConversationMutedForViewer(ctx context.Context, viewerAccountID, statusID string) (bool, error)
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

// canViewStatus returns whether the viewer can see the status (visibility, block,
// and author suspension rules). author must be the status's author; suspended or
// domain-suspended authors are hidden from everyone except the author themselves.
func (svc *statusService) canViewStatus(ctx context.Context, st *domain.Status, author *domain.Account, viewerAccountID *string) (bool, error) {
	if author != nil && author.IsHidden() {
		// Author themselves can still see their own statuses (so an undo-style
		// UI is possible during a suspension). Everyone else is blocked.
		if viewerAccountID == nil || *viewerAccountID != st.AccountID {
			return false, nil
		}
	}
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
		if slices.Contains(mentionIDs, *viewerAccountID) {
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
		if !slices.Contains(mentionIDs, *viewerAccountID) {
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

// CanViewStatus implements StatusVisibilityChecker.
func (svc *statusService) CanViewStatus(ctx context.Context, st *domain.Status, author *domain.Account, viewerAccountID *string) (bool, error) {
	return svc.canViewStatus(ctx, st, author, viewerAccountID)
}

// GetByIDEnriched returns the status with author, mentions, tags, and media for API response.
// If the viewer cannot see the status (visibility, block, or author suspension), returns domain.ErrNotFound.
func (svc *statusService) GetByIDEnriched(ctx context.Context, id string, viewerAccountID *string) (EnrichedStatus, error) {
	st, err := svc.store.GetStatusByID(ctx, id)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("GetStatusByID(%s): %w", id, err)
	}
	if st.DeletedAt != nil {
		return EnrichedStatus{}, fmt.Errorf("GetByIDEnriched(%s): %w", id, domain.ErrNotFound)
	}
	author, err := svc.store.GetAccountByID(ctx, st.AccountID)
	if err != nil {
		return EnrichedStatus{}, fmt.Errorf("GetByIDEnriched GetAccountByID(%s): %w", st.AccountID, err)
	}
	ok, err := svc.canViewStatus(ctx, st, author, viewerAccountID)
	if err != nil {
		return EnrichedStatus{}, err
	}
	if !ok {
		return EnrichedStatus{}, fmt.Errorf("GetByIDEnriched(%s): %w", id, domain.ErrNotFound)
	}
	enriched, err := svc.EnrichStatuses(ctx, []*domain.Status{st}, EnrichOpts{
		IncludeCard: true,
		IncludePoll: true,
		ViewerID:    viewerAccountID,
		Authors:     map[string]*domain.Account{author.ID: author},
	})
	if err != nil {
		return EnrichedStatus{}, err
	}
	return enriched[0], nil
}

// GetByIDsEnriched fetches multiple statuses, filters out deleted or invisible
// ones, and enriches the remainder in a single batch call. Statuses that cannot
// be found or cannot be viewed are silently skipped (order is preserved).
func (svc *statusService) GetByIDsEnriched(ctx context.Context, ids []string, viewerAccountID *string, filterContext domain.FilterContext) ([]EnrichedStatus, error) {
	var fetched []*domain.Status
	authorIDs := make(map[string]struct{})
	for _, id := range ids {
		st, err := svc.store.GetStatusByID(ctx, id)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("GetByIDsEnriched GetStatusByID(%s): %w", id, err)
		}
		if st.DeletedAt != nil {
			continue
		}
		fetched = append(fetched, st)
		authorIDs[st.AccountID] = struct{}{}
	}
	if len(fetched) == 0 {
		return nil, nil
	}
	authors, err := svc.loadAuthors(ctx, authorIDs)
	if err != nil {
		return nil, fmt.Errorf("GetByIDsEnriched: %w", err)
	}
	visible := make([]*domain.Status, 0, len(fetched))
	for _, st := range fetched {
		ok, err := svc.canViewStatus(ctx, st, authors[st.AccountID], viewerAccountID)
		if err != nil {
			return nil, fmt.Errorf("GetByIDsEnriched canViewStatus(%s): %w", st.ID, err)
		}
		if !ok {
			continue
		}
		visible = append(visible, st)
	}
	if len(visible) == 0 {
		return nil, nil
	}
	return svc.EnrichStatuses(ctx, visible, EnrichOpts{
		IncludeCard:   true,
		IncludePoll:   true,
		ViewerID:      viewerAccountID,
		FilterContext: filterContext,
		Authors:       authors,
	})
}

// loadAuthors batch-fetches accounts by the given set of IDs and returns a
// lookup map. Used by paths that need authors for both visibility checks and
// downstream enrichment so we issue a single GetAccountsByIDs query instead of
// O(n) GetAccountByID calls.
func (svc *statusService) loadAuthors(ctx context.Context, ids map[string]struct{}) (map[string]*domain.Account, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	idSlice := make([]string, 0, len(ids))
	for id := range ids {
		idSlice = append(idSlice, id)
	}
	accounts, err := svc.store.GetAccountsByIDs(ctx, idSlice)
	if err != nil {
		return nil, fmt.Errorf("GetAccountsByIDs: %w", err)
	}
	out := make(map[string]*domain.Account, len(accounts))
	for _, a := range accounts {
		out[a.ID] = a
	}
	return out, nil
}

// EnrichStatuses loads author, mentions, tags, media, and optionally card, poll, and viewer flags for each status.
func (svc *statusService) EnrichStatuses(ctx context.Context, statuses []*domain.Status, opts EnrichOpts) ([]EnrichedStatus, error) {
	// Load and compile viewer's active filters once for the whole batch.
	var compiledViewerFilters []compiledFilter
	if opts.ViewerID != nil {
		if f, err := svc.store.GetActiveFilters(ctx, *opts.ViewerID); err == nil {
			compiledViewerFilters = compileFilters(f)
		} else {
			slog.WarnContext(ctx, "enrich status: get active filters", slog.Any("error", err))
		}
	}

	out := make([]EnrichedStatus, 0, len(statuses))
	for _, st := range statuses {
		var author *domain.Account
		if a, ok := opts.Authors[st.AccountID]; ok && a != nil {
			author = a
		} else {
			a, err := svc.store.GetAccountByID(ctx, st.AccountID)
			if err != nil {
				return nil, fmt.Errorf("GetAccountByID(%s): %w", st.AccountID, err)
			}
			author = a
		}
		mentions, err := svc.store.GetStatusMentions(ctx, st.ID)
		if err != nil {
			return nil, fmt.Errorf("GetStatusMentions(%s): %w", st.ID, err)
		}
		tags, err := svc.store.GetStatusHashtags(ctx, st.ID)
		if err != nil {
			return nil, fmt.Errorf("GetStatusHashtags(%s): %w", st.ID, err)
		}
		media, err := svc.store.GetStatusAttachments(ctx, st.ID)
		if err != nil {
			return nil, fmt.Errorf("GetStatusAttachments(%s): %w", st.ID, err)
		}
		e := EnrichedStatus{
			Status:   st,
			Author:   author,
			Mentions: mentions,
			Tags:     tags,
			Media:    media,
		}
		if opts.IncludeCard {
			card, err := svc.store.GetStatusCard(ctx, st.ID)
			if err != nil && !errors.Is(err, domain.ErrNotFound) {
				return nil, fmt.Errorf("GetStatusCard(%s): %w", st.ID, err)
			}
			e.Card = card
		}
		if opts.IncludePoll {
			poll, pollErr := svc.store.GetPollByStatusID(ctx, st.ID)
			if pollErr != nil && !errors.Is(pollErr, domain.ErrNotFound) {
				slog.WarnContext(ctx, "enrich status: get poll", slog.Any("error", pollErr), slog.String("status_id", st.ID))
			}
			if pollErr == nil && poll != nil {
				enrichedPoll, enrichErr := svc.getPollEnriched(ctx, poll.ID, opts.ViewerID)
				if enrichErr != nil {
					slog.WarnContext(ctx, "enrich status: enrich poll", slog.Any("error", enrichErr), slog.String("poll_id", poll.ID), slog.String("status_id", st.ID))
				} else {
					e.Poll = enrichedPoll
				}
			}
		}
		if opts.ViewerID != nil {
			if _, err := svc.store.GetFavouriteByAccountAndStatus(ctx, *opts.ViewerID, st.ID); err == nil {
				e.Favourited = true
			} else if !errors.Is(err, domain.ErrNotFound) {
				slog.WarnContext(ctx, "enrich status: get favourite", slog.Any("error", err))
			}
			if _, err := svc.store.GetReblogByAccountAndTarget(ctx, *opts.ViewerID, st.ID); err == nil {
				e.Reblogged = true
			} else if !errors.Is(err, domain.ErrNotFound) {
				slog.WarnContext(ctx, "enrich status: get reblog", slog.Any("error", err))
			}
			if ok, err := svc.store.IsBookmarked(ctx, *opts.ViewerID, st.ID); err == nil {
				e.Bookmarked = ok
			} else if !errors.Is(err, domain.ErrNotFound) {
				slog.WarnContext(ctx, "enrich status: check bookmark", slog.Any("error", err))
			}
			if st.AccountID == *opts.ViewerID {
				pinnedIDs, err := svc.store.ListPinnedStatusIDs(ctx, *opts.ViewerID)
				if err == nil {
					for _, pid := range pinnedIDs {
						if pid == st.ID {
							e.Pinned = true
							break
						}
					}
				}
			}
			if muted, err := svc.IsConversationMutedForViewer(ctx, *opts.ViewerID, st.ID); err == nil {
				e.Muted = muted
			}
			if len(compiledViewerFilters) > 0 {
				content := ""
				if st.Content != nil {
					content = *st.Content
				}
				cw := ""
				if st.ContentWarning != nil {
					cw = *st.ContentWarning
				}
				e.FilterResults = matchCompiledFilters(compiledViewerFilters, st.ID, content, cw)
			}
		}
		// Server-side removal of hide-filtered statuses is an optimization — clients
		// also handle this via the filtered array. When FilterContext is set, we exclude
		// statuses that match a "hide" filter for the requested context so the client
		// never receives them.
		if opts.FilterContext != "" && shouldHideByFilter(e.FilterResults, opts.FilterContext) {
			continue
		}
		if st.ReblogOfID != nil {
			origSt, origErr := svc.store.GetStatusByID(ctx, *st.ReblogOfID)
			if origErr == nil && origSt.DeletedAt == nil {
				origEnriched, origErr := svc.EnrichStatuses(ctx, []*domain.Status{origSt}, opts)
				if origErr == nil && len(origEnriched) > 0 {
					e.ReblogOf = &origEnriched[0]
				}
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// shouldHideByFilter reports whether any filter result has filter_action "hide"
// and the filter's context includes the given filterContext.
func shouldHideByFilter(results []domain.FilterResult, filterContext domain.FilterContext) bool {
	for _, r := range results {
		if r.Filter.FilterAction == domain.FilterActionHide && slices.Contains(r.Filter.Context, filterContext) {
			return true
		}
	}
	return false
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
	author, err := svc.store.GetAccountByID(ctx, st.AccountID)
	if err != nil {
		return nil, fmt.Errorf("GetPoll GetAccountByID(%s): %w", st.AccountID, err)
	}
	ok, err := svc.canViewStatus(ctx, st, author, viewerAccountID)
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
	optionsWithCount := make([]PollOptionWithCount, 0, len(opts))
	for _, o := range opts {
		optionsWithCount = append(optionsWithCount, PollOptionWithCount{Title: o.Title, VotesCount: o.VotesCount})
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

func (svc *statusService) ListPinnedStatusIDs(ctx context.Context, accountID string) ([]string, error) {
	ids, err := svc.store.ListPinnedStatusIDs(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ListPinnedStatusIDs: %w", err)
	}
	return ids, nil
}

func (svc *statusService) PinnedStatusesEnriched(ctx context.Context, accountID string, viewerAccountID *string) ([]EnrichedStatus, error) {
	ids, err := svc.ListPinnedStatusIDs(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("PinnedStatusesEnriched: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	enriched, err := svc.GetByIDsEnriched(ctx, ids, viewerAccountID, domain.FilterContextAccount)
	if err != nil {
		return nil, fmt.Errorf("PinnedStatusesEnriched: %w", err)
	}
	for i := range enriched {
		enriched[i].Pinned = true
	}
	return enriched, nil
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
	author, err := svc.store.GetAccountByID(ctx, st.AccountID)
	if err != nil {
		return nil, fmt.Errorf("GetStatusHistory GetAccountByID(%s): %w", st.AccountID, err)
	}
	ok, err := svc.canViewStatus(ctx, st, author, viewerAccountID)
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
	author, err := svc.store.GetAccountByID(ctx, st.AccountID)
	if err != nil {
		return "", "", fmt.Errorf("GetStatusSource GetAccountByID(%s): %w", st.AccountID, err)
	}
	ok, err := svc.canViewStatus(ctx, st, author, viewerAccountID)
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

func (svc *statusService) GetScheduledStatus(ctx context.Context, id, accountID string) (*domain.ScheduledStatus, error) {
	s, err := svc.store.GetScheduledStatusByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetScheduledStatus: %w", err)
	}
	if s.AccountID != accountID {
		return nil, fmt.Errorf("GetScheduledStatus: %w", domain.ErrNotFound)
	}
	return s, nil
}

func (svc *statusService) ListScheduledStatuses(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.ScheduledStatus, error) {
	limit = ClampLimit(limit, 20, 40)
	list, err := svc.store.ListScheduledStatuses(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, fmt.Errorf("ListScheduledStatuses: %w", err)
	}
	return list, nil
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
	rootAuthor, err := svc.store.GetAccountByID(ctx, st.AccountID)
	if err != nil {
		return ContextResult{}, fmt.Errorf("GetContext GetAccountByID(%s): %w", st.AccountID, err)
	}
	ok, err := svc.canViewStatus(ctx, st, rootAuthor, viewerAccountID)
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
	authorIDs := make(map[string]struct{}, len(ancestors)+len(descendants))
	for i := range ancestors {
		authorIDs[ancestors[i].AccountID] = struct{}{}
	}
	for i := range descendants {
		authorIDs[descendants[i].AccountID] = struct{}{}
	}
	authors, err := svc.loadAuthors(ctx, authorIDs)
	if err != nil {
		return ContextResult{}, fmt.Errorf("GetContext: %w", err)
	}
	filteredAncestors := make([]domain.Status, 0, len(ancestors))
	for i := range ancestors {
		ok, err := svc.canViewStatus(ctx, &ancestors[i], authors[ancestors[i].AccountID], viewerAccountID)
		if err != nil {
			return ContextResult{}, err
		}
		if ok {
			filteredAncestors = append(filteredAncestors, ancestors[i])
		}
	}
	filteredDescendants := make([]domain.Status, 0, len(descendants))
	for i := range descendants {
		ok, err := svc.canViewStatus(ctx, &descendants[i], authors[descendants[i].AccountID], viewerAccountID)
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
	author, err := svc.store.GetAccountByID(ctx, st.AccountID)
	if err != nil {
		return nil, fmt.Errorf("GetFavouritedBy GetAccountByID(%s): %w", st.AccountID, err)
	}
	ok, err := svc.canViewStatus(ctx, st, author, viewerAccountID)
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
		if !accounts[i].IsHidden() {
			out = append(out, &accounts[i])
		}
	}
	if viewerAccountID != nil {
		filtered := make([]*domain.Account, 0, len(out))
		for _, a := range out {
			blocked, blockErr := svc.store.IsBlockedEitherDirection(ctx, *viewerAccountID, a.ID)
			if blockErr != nil {
				return nil, fmt.Errorf("GetFavouritedBy IsBlockedEitherDirection: %w", blockErr)
			}
			if !blocked {
				filtered = append(filtered, a)
			}
		}
		out = filtered
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
	author, err := svc.store.GetAccountByID(ctx, st.AccountID)
	if err != nil {
		return nil, fmt.Errorf("GetRebloggedBy GetAccountByID(%s): %w", st.AccountID, err)
	}
	ok, err := svc.canViewStatus(ctx, st, author, viewerAccountID)
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
		if !accounts[i].IsHidden() {
			out = append(out, &accounts[i])
		}
	}
	if viewerAccountID != nil {
		filtered := make([]*domain.Account, 0, len(out))
		for _, a := range out {
			blocked, blockErr := svc.store.IsBlockedEitherDirection(ctx, *viewerAccountID, a.ID)
			if blockErr != nil {
				return nil, fmt.Errorf("GetRebloggedBy IsBlockedEitherDirection: %w", blockErr)
			}
			if !blocked {
				filtered = append(filtered, a)
			}
		}
		out = filtered
	}
	return out, nil
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

// GetQuoteApproval returns the quote approval record for a quoting status, or ErrNotFound.
func (svc *statusService) GetQuoteApproval(ctx context.Context, quotingStatusID string) (*domain.QuoteApprovalRecord, error) {
	rec, err := svc.store.GetQuoteApproval(ctx, quotingStatusID)
	if err != nil {
		return nil, fmt.Errorf("GetQuoteApproval(%s): %w", quotingStatusID, err)
	}
	return rec, nil
}

// ListQuotesOfStatus returns enriched statuses that quote the given status (non-revoked).
// Viewer must be able to see the quoted status.
func (svc *statusService) ListQuotesOfStatus(ctx context.Context, quotedStatusID string, maxID *string, limit int, viewerAccountID *string) ([]EnrichedStatus, error) {
	quoted, err := svc.store.GetStatusByID(ctx, quotedStatusID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("ListQuotesOfStatus: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("ListQuotesOfStatus: %w", err)
	}
	quotedAuthor, err := svc.store.GetAccountByID(ctx, quoted.AccountID)
	if err != nil {
		return nil, fmt.Errorf("ListQuotesOfStatus GetAccountByID(%s): %w", quoted.AccountID, err)
	}
	ok, err := svc.canViewStatus(ctx, quoted, quotedAuthor, viewerAccountID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("ListQuotesOfStatus: %w", domain.ErrNotFound)
	}
	limit = ClampLimit(limit, 20, 80)
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
