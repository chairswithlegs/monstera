package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// ScheduledStatusService handles creation, update, deletion, and publication of scheduled statuses.
type ScheduledStatusService interface {
	CreateScheduledStatus(ctx context.Context, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error)
	UpdateScheduledStatus(ctx context.Context, id, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error)
	DeleteScheduledStatus(ctx context.Context, id, accountID string) error
	PublishDueStatuses(ctx context.Context, limit int) (int, error)
}

type scheduledStatusService struct {
	store        store.Store
	statusWriter StatusWriteService
}

// NewScheduledStatusService returns a ScheduledStatusService.
func NewScheduledStatusService(s store.Store, sw StatusWriteService) ScheduledStatusService {
	return &scheduledStatusService{store: s, statusWriter: sw}
}

func (svc *scheduledStatusService) CreateScheduledStatus(ctx context.Context, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error) {
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

func (svc *scheduledStatusService) UpdateScheduledStatus(ctx context.Context, id, accountID string, params []byte, scheduledAt time.Time) (*domain.ScheduledStatus, error) {
	s, err := svc.store.GetScheduledStatusByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("UpdateScheduledStatus: %w", err)
	}
	if s.AccountID != accountID {
		return nil, fmt.Errorf("UpdateScheduledStatus: %w", domain.ErrNotFound)
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

func (svc *scheduledStatusService) DeleteScheduledStatus(ctx context.Context, id, accountID string) error {
	s, err := svc.store.GetScheduledStatusByID(ctx, id)
	if err != nil {
		return fmt.Errorf("DeleteScheduledStatus: %w", err)
	}
	if s.AccountID != accountID {
		return fmt.Errorf("DeleteScheduledStatus: %w", domain.ErrNotFound)
	}
	if err := svc.store.DeleteScheduledStatus(ctx, id); err != nil {
		return fmt.Errorf("DeleteScheduledStatus: %w", err)
	}
	return nil
}

func (svc *scheduledStatusService) publishScheduled(ctx context.Context, scheduledID string) error {
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
	// Best-effort: user lookup provides default visibility. If it fails, we
	// fall back to empty string which the create path handles.
	user, err := svc.store.GetUserByAccountID(ctx, s.AccountID)
	if err != nil {
		slog.WarnContext(ctx, "publishScheduled: get user for default visibility", slog.Any("error", err), slog.String("account_id", s.AccountID))
	}
	defaultVisibility := ""
	if user != nil {
		defaultVisibility = user.DefaultPrivacy
	}
	var inReplyToID *string
	if p.InReplyToID != "" {
		inReplyToID = &p.InReplyToID
	}
	_, err = svc.statusWriter.Create(ctx, CreateStatusInput{
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
		return fmt.Errorf("PublishScheduled Create: %w", err)
	}
	if err := svc.store.DeleteScheduledStatus(ctx, scheduledID); err != nil {
		return fmt.Errorf("PublishScheduled DeleteScheduledStatus: %w", err)
	}
	return nil
}

func (svc *scheduledStatusService) PublishDueStatuses(ctx context.Context, limit int) (int, error) {
	due, err := svc.store.ListScheduledStatusesDue(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("list due: %w", err)
	}
	for i := range due {
		if err := svc.publishScheduled(ctx, due[i].ID); err != nil {
			slog.WarnContext(ctx, "scheduled status publish failed",
				slog.String("id", due[i].ID), slog.Any("error", err))
		}
	}
	return len(due), nil
}
