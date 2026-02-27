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

// BlockPublisher sends Block and Undo{Block} activities to remote inboxes.
// May be nil to skip federation.
type BlockPublisher interface {
	PublishBlock(ctx context.Context, actor, target *domain.Account) error
	PublishUndoBlock(ctx context.Context, actor, target *domain.Account) error
}

// FollowService orchestrates follow/unfollow, block/mute, and relationship lookups.
type FollowService struct {
	store store.Store
	pub   FollowPublisher
	block BlockPublisher
}

// NewFollowService returns a FollowService. pub and block may be nil.
func NewFollowService(s store.Store, pub FollowPublisher, block BlockPublisher) *FollowService {
	return &FollowService{store: s, pub: pub, block: block}
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

// Block creates a block from actor to target. Removes follow in either direction and any mute. Returns the relationship.
func (svc *FollowService) Block(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	if actorAccountID == targetAccountID {
		return nil, fmt.Errorf("Block: %w", domain.ErrValidation)
	}
	_, err := svc.store.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("Block target: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("GetAccountByID(target): %w", err)
	}
	actor, _ := svc.store.GetAccountByID(ctx, actorAccountID)
	target, _ := svc.store.GetAccountByID(ctx, targetAccountID)

	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if fw, _ := tx.GetFollow(ctx, actorAccountID, targetAccountID); fw != nil {
			if err := tx.DeleteFollow(ctx, actorAccountID, targetAccountID); err != nil {
				return fmt.Errorf("DeleteFollow: %w", err)
			}
			_ = tx.DecrementFollowingCount(ctx, actorAccountID)
			_ = tx.DecrementFollowersCount(ctx, targetAccountID)
		}
		if bw, _ := tx.GetFollow(ctx, targetAccountID, actorAccountID); bw != nil {
			if err := tx.DeleteFollow(ctx, targetAccountID, actorAccountID); err != nil {
				return fmt.Errorf("DeleteFollow reverse: %w", err)
			}
			_ = tx.DecrementFollowingCount(ctx, targetAccountID)
			_ = tx.DecrementFollowersCount(ctx, actorAccountID)
		}
		_ = tx.DeleteMute(ctx, actorAccountID, targetAccountID)
		return tx.CreateBlock(ctx, store.CreateBlockInput{
			ID:        uid.New(),
			AccountID: actorAccountID,
			TargetID:  targetAccountID,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("Block tx: %w", err)
	}
	if svc.block != nil && target != nil && actor != nil && target.InboxURL != "" {
		_ = svc.block.PublishBlock(ctx, actor, target)
	}
	rel, err := svc.store.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Block GetRelationship: %w", err)
	}
	return rel, nil
}

// Unblock removes the block from actor to target. Returns the relationship.
func (svc *FollowService) Unblock(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	if err := svc.store.DeleteBlock(ctx, actorAccountID, targetAccountID); err != nil {
		return nil, fmt.Errorf("DeleteBlock: %w", err)
	}
	if svc.block != nil {
		actor, _ := svc.store.GetAccountByID(ctx, actorAccountID)
		target, _ := svc.store.GetAccountByID(ctx, targetAccountID)
		if actor != nil && target != nil && target.InboxURL != "" {
			_ = svc.block.PublishUndoBlock(ctx, actor, target)
		}
	}
	rel, err := svc.store.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Unblock GetRelationship: %w", err)
	}
	return rel, nil
}

// Mute creates or updates a mute from actor to target. notifications controls hide_notifications.
func (svc *FollowService) Mute(ctx context.Context, actorAccountID, targetAccountID string, hideNotifications bool) (*domain.Relationship, error) {
	if actorAccountID == targetAccountID {
		return nil, fmt.Errorf("Mute: %w", domain.ErrValidation)
	}
	_, err := svc.store.GetAccountByID(ctx, targetAccountID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("Mute target: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("GetAccountByID(target): %w", err)
	}
	if err := svc.store.CreateMute(ctx, store.CreateMuteInput{
		ID:                uid.New(),
		AccountID:         actorAccountID,
		TargetID:          targetAccountID,
		HideNotifications: hideNotifications,
	}); err != nil {
		return nil, fmt.Errorf("CreateMute: %w", err)
	}
	rel, err := svc.store.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Mute GetRelationship: %w", err)
	}
	return rel, nil
}

// Unmute removes the mute from actor to target. Returns the relationship.
func (svc *FollowService) Unmute(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	_ = svc.store.DeleteMute(ctx, actorAccountID, targetAccountID)
	rel, err := svc.store.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Unmute GetRelationship: %w", err)
	}
	return rel, nil
}

// GetFollowers returns the list of followers for the given account (paginated).
func (svc *FollowService) GetFollowers(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error) {
	list, err := svc.store.GetFollowers(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, fmt.Errorf("GetFollowers: %w", err)
	}
	return list, nil
}

// GetFollowing returns the list of accounts the given account follows (paginated).
func (svc *FollowService) GetFollowing(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error) {
	list, err := svc.store.GetFollowing(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, fmt.Errorf("GetFollowing: %w", err)
	}
	return list, nil
}
