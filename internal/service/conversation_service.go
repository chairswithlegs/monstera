package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// ConversationService handles listing, marking read, and removing DM conversations,
// and updating conversation state when direct statuses are created.
type ConversationService interface {
	ListConversations(ctx context.Context, accountID string, maxID *string, limit int) ([]ConversationResult, *string, error)
	MarkConversationRead(ctx context.Context, accountID, conversationID string) (*ConversationResult, error)
	RemoveConversation(ctx context.Context, accountID, conversationID string) error
	UpdateForDirectStatus(ctx context.Context, status *domain.Status, authorID string, mentionedAccountIDs []string) error
	MuteConversation(ctx context.Context, accountID, statusID string) error
	UnmuteConversation(ctx context.Context, accountID, statusID string) error
}

// ConversationResult is a single conversation with resolved last status and participants.
type ConversationResult struct {
	AccountConversation domain.AccountConversation
	LastStatus          *EnrichedStatus
	Participants        []*domain.Account
}

type conversationService struct {
	store         store.Store
	statusService StatusService
}

// NewConversationService returns a ConversationService.
func NewConversationService(s store.Store, statusService StatusService) ConversationService {
	return &conversationService{store: s, statusService: statusService}
}

func (svc *conversationService) ListConversations(ctx context.Context, accountID string, maxID *string, limit int) ([]ConversationResult, *string, error) {
	limit = ClampLimit(limit, DefaultServiceListLimit, MaxServicePageLimit)
	rows, nextCursor, err := svc.store.ListAccountConversations(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("ListAccountConversations: %w", err)
	}
	results := make([]ConversationResult, 0, len(rows))
	for _, ac := range rows {
		if ac.LastStatusID == nil || *ac.LastStatusID == "" {
			continue
		}
		enriched, err := svc.statusService.GetByIDEnriched(ctx, *ac.LastStatusID, &accountID)
		if err != nil {
			continue
		}
		if enriched.Status.DeletedAt != nil {
			continue
		}
		participants := participantsFromEnriched(&enriched, accountID)
		results = append(results, ConversationResult{
			AccountConversation: ac,
			LastStatus:          &enriched,
			Participants:        participants,
		})
	}
	return results, nextCursor, nil
}

func (svc *conversationService) MarkConversationRead(ctx context.Context, accountID, conversationID string) (*ConversationResult, error) {
	ac, err := svc.store.GetAccountConversation(ctx, accountID, conversationID)
	if err != nil {
		return nil, fmt.Errorf("GetAccountConversation: %w", err)
	}
	if err := svc.store.MarkAccountConversationRead(ctx, accountID, conversationID); err != nil {
		return nil, fmt.Errorf("MarkAccountConversationRead: %w", err)
	}
	ac.Unread = false
	var lastStatus *EnrichedStatus
	var participants []*domain.Account
	if ac.LastStatusID != nil && *ac.LastStatusID != "" {
		enriched, getErr := svc.statusService.GetByIDEnriched(ctx, *ac.LastStatusID, &accountID)
		if getErr == nil && enriched.Status.DeletedAt == nil {
			lastStatus = &enriched
			participants = participantsFromEnriched(&enriched, accountID)
		}
	}
	return &ConversationResult{
		AccountConversation: *ac,
		LastStatus:          lastStatus,
		Participants:        participants,
	}, nil
}

func (svc *conversationService) RemoveConversation(ctx context.Context, accountID, conversationID string) error {
	if err := svc.store.DeleteAccountConversation(ctx, accountID, conversationID); err != nil {
		return fmt.Errorf("DeleteAccountConversation: %w", err)
	}
	return nil
}

func (svc *conversationService) UpdateForDirectStatus(ctx context.Context, status *domain.Status, authorID string, mentionedAccountIDs []string) error {
	var conversationID string
	if status.InReplyToID != nil && *status.InReplyToID != "" {
		parentCID, err := svc.store.GetStatusConversationID(ctx, *status.InReplyToID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("GetStatusConversationID(parent): %w", err)
		}
		if parentCID != nil && *parentCID != "" {
			conversationID = *parentCID
		}
	}
	if conversationID == "" {
		conversationID = uid.New()
		if err := svc.store.CreateConversation(ctx, conversationID); err != nil {
			return fmt.Errorf("CreateConversation: %w", err)
		}
	}
	if err := svc.store.SetStatusConversationID(ctx, status.ID, conversationID); err != nil {
		return fmt.Errorf("SetStatusConversationID: %w", err)
	}
	participantIDs := make(map[string]struct{})
	participantIDs[authorID] = struct{}{}
	for _, id := range mentionedAccountIDs {
		participantIDs[id] = struct{}{}
	}
	for pid := range participantIDs {
		unread := pid != authorID
		acID := uid.New()
		if err := svc.store.UpsertAccountConversation(ctx, store.UpsertAccountConversationInput{
			ID:             acID,
			AccountID:      pid,
			ConversationID: conversationID,
			LastStatusID:   status.ID,
			Unread:         unread,
		}); err != nil {
			return fmt.Errorf("UpsertAccountConversation: %w", err)
		}
	}
	return nil
}

func (svc *conversationService) MuteConversation(ctx context.Context, accountID, statusID string) error {
	root, err := svc.store.GetConversationRoot(ctx, statusID)
	if err != nil {
		return fmt.Errorf("MuteConversation GetConversationRoot: %w", err)
	}
	if err := svc.store.CreateConversationMute(ctx, accountID, root); err != nil {
		return fmt.Errorf("CreateConversationMute: %w", err)
	}
	return nil
}

func (svc *conversationService) UnmuteConversation(ctx context.Context, accountID, statusID string) error {
	root, err := svc.store.GetConversationRoot(ctx, statusID)
	if err != nil {
		return fmt.Errorf("UnmuteConversation GetConversationRoot: %w", err)
	}
	if err := svc.store.DeleteConversationMute(ctx, accountID, root); err != nil {
		return fmt.Errorf("DeleteConversationMute: %w", err)
	}
	return nil
}

func participantsFromEnriched(e *EnrichedStatus, viewerAccountID string) []*domain.Account {
	seen := make(map[string]struct{})
	seen[viewerAccountID] = struct{}{}
	var out []*domain.Account
	if e.Author != nil && e.Author.ID != viewerAccountID {
		if _, ok := seen[e.Author.ID]; !ok {
			seen[e.Author.ID] = struct{}{}
			out = append(out, e.Author)
		}
	}
	for _, m := range e.Mentions {
		if m != nil && m.ID != viewerAccountID {
			if _, ok := seen[m.ID]; !ok {
				seen[m.ID] = struct{}{}
				out = append(out, m)
			}
		}
	}
	return out
}
