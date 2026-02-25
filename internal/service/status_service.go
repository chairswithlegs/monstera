package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
)

// StatusService handles status creation, lookup, and soft delete.
type StatusService struct {
	store           store.Store
	instanceBaseURL string
}

// NewStatusService returns a StatusService that uses the given store and instance base URL.
func NewStatusService(s store.Store, instanceBaseURL string) *StatusService {
	base := strings.TrimSuffix(instanceBaseURL, "/")
	return &StatusService{store: s, instanceBaseURL: base}
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
func (svc *StatusService) Create(ctx context.Context, in CreateStatusInput) (*domain.Status, error) {
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
		return tx.IncrementStatusesCount(ctx, in.AccountID)
	})
	if err != nil {
		return nil, fmt.Errorf("CreateStatus: %w", err)
	}
	return st, nil
}

// GetByID returns the status by ID, or ErrNotFound.
func (svc *StatusService) GetByID(ctx context.Context, id string) (*domain.Status, error) {
	st, err := svc.store.GetStatusByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetStatusByID(%s): %w", id, err)
	}
	return st, nil
}

// Delete soft-deletes the status and decrements the account's statuses count atomically.
func (svc *StatusService) Delete(ctx context.Context, id string) error {
	st, err := svc.store.GetStatusByID(ctx, id)
	if err != nil {
		return fmt.Errorf("Delete(%s): %w", id, err)
	}
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.DeleteStatus(ctx, id); err != nil {
			return fmt.Errorf("DeleteStatus: %w", err)
		}
		return tx.DecrementStatusesCount(ctx, st.AccountID)
	})
	if err != nil {
		return fmt.Errorf("Delete(%s): %w", id, err)
	}
	return nil
}
