package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
)

// FollowPublisher sends Follow and Undo{Follow} activities to remote inboxes.
// Implemented by *ap.OutboxPublisher; may be nil to skip federation.
type FollowPublisher interface {
	PublishFollow(ctx context.Context, actor, target *domain.Account, followID string) error
	PublishUndoFollow(ctx context.Context, actor, target *domain.Account, followID string) error
}

// FollowService orchestrates follow/unfollow and relationship lookups.
type FollowService struct {
	store store.Store
	pub   FollowPublisher
}

// NewFollowService returns a FollowService. pub may be nil.
func NewFollowService(s store.Store, pub FollowPublisher) *FollowService {
	return &FollowService{store: s, pub: pub}
}

// Follow creates a follow from actor to target. Returns the relationship after the change.
// Errors: ErrValidation (self-follow), ErrNotFound (target), ErrForbidden (block in either direction).
func (svc *FollowService) Follow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	if actorAccountID == targetAccountID {
		return nil, fmt.Errorf("Follow: %w", domain.ErrValidation)
	}
	target, err := svc.store.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("Follow target: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("GetAccountByID(target): %w", err)
	}
	if target.Suspended {
		return nil, fmt.Errorf("Follow target: %w", domain.ErrNotFound)
	}
	rel, err := svc.store.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Follow GetRelationship: %w", err)
	}
	if rel.Blocking || rel.BlockedBy {
		return nil, fmt.Errorf("Follow: %w", domain.ErrForbidden)
	}
	actor, err := svc.store.GetAccountByID(ctx, actorAccountID)
	if err != nil {
		return nil, fmt.Errorf("GetAccountByID(actor): %w", err)
	}

	state := domain.FollowStateAccepted
	if target.Locked {
		state = domain.FollowStatePending
	}

	existing, _ := svc.store.GetFollow(ctx, actorAccountID, targetAccountID)
	if existing != nil {
		if existing.State == domain.FollowStateAccepted || existing.State == domain.FollowStatePending {
			rel, err := svc.store.GetRelationship(ctx, actorAccountID, targetAccountID)
			if err != nil {
				return nil, fmt.Errorf("Follow GetRelationship existing: %w", err)
			}
			return rel, nil
		}
	}

	var follow *domain.Follow
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		var txErr error
		follow, txErr = tx.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: actorAccountID,
			TargetID:  targetAccountID,
			State:     state,
			APID:      nil,
		})
		if txErr != nil {
			return fmt.Errorf("CreateFollow: %w", txErr)
		}
		if txErr := tx.IncrementFollowingCount(ctx, actorAccountID); txErr != nil {
			return fmt.Errorf("IncrementFollowingCount: %w", txErr)
		}
		if txErr := tx.IncrementFollowersCount(ctx, targetAccountID); txErr != nil {
			return fmt.Errorf("IncrementFollowersCount: %w", txErr)
		}
		notifType := domain.NotificationTypeFollow
		if state == domain.FollowStatePending {
			notifType = domain.NotificationTypeFollowRequest
		}
		_, txErr = tx.CreateNotification(ctx, store.CreateNotificationInput{
			ID:        uid.New(),
			AccountID: targetAccountID,
			FromID:    actorAccountID,
			Type:      notifType,
			StatusID:  nil,
		})
		if txErr != nil {
			return fmt.Errorf("CreateNotification: %w", txErr)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Follow tx: %w", err)
	}

	if svc.pub != nil && target.InboxURL != "" {
		_ = svc.pub.PublishFollow(ctx, actor, target, follow.ID)
	}

	rel, err = svc.store.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Follow GetRelationship after: %w", err)
	}
	return rel, nil
}

// Unfollow removes the follow from actor to target. Returns the relationship after the change.
func (svc *FollowService) Unfollow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	follow, err := svc.store.GetFollow(ctx, actorAccountID, targetAccountID)
	if err != nil {
		rel, _ := svc.store.GetRelationship(ctx, actorAccountID, targetAccountID)
		if rel != nil {
			return rel, nil
		}
		return nil, fmt.Errorf("GetFollow: %w", err)
	}
	target, _ := svc.store.GetAccountByID(ctx, targetAccountID)
	actor, _ := svc.store.GetAccountByID(ctx, actorAccountID)

	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if err := tx.DeleteFollow(ctx, actorAccountID, targetAccountID); err != nil {
			return fmt.Errorf("DeleteFollow: %w", err)
		}
		if err := tx.DecrementFollowingCount(ctx, actorAccountID); err != nil {
			return fmt.Errorf("DecrementFollowingCount: %w", err)
		}
		return tx.DecrementFollowersCount(ctx, targetAccountID)
	})
	if err != nil {
		return nil, fmt.Errorf("Unfollow tx: %w", err)
	}

	if svc.pub != nil && target != nil && actor != nil && target.InboxURL != "" {
		_ = svc.pub.PublishUndoFollow(ctx, actor, target, follow.ID)
	}

	rel, err := svc.store.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Unfollow GetRelationship: %w", err)
	}
	return rel, nil
}
