package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
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
type FollowService interface {
	Follow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error)
	Unfollow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error)
	Block(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error)
	Unblock(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error)
	Mute(ctx context.Context, actorAccountID, targetAccountID string, hideNotifications bool) (*domain.Relationship, error)
	Unmute(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error)
	GetFollowByAPID(ctx context.Context, apID string) (*domain.Follow, error)
	GetFollow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Follow, error)
	CreateFollowFromInbox(ctx context.Context, actorAccountID, targetAccountID string, state string, apID *string) (*domain.Follow, error)
	AcceptFollow(ctx context.Context, followID string) error
	DeleteFollowFromInbox(ctx context.Context, actorAccountID, targetAccountID string) error
	GetFollowers(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error)
	GetFollowing(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error)
}

type followService struct {
	store store.Store
	pub   FollowPublisher
	block BlockPublisher
}

// NewFollowService returns a FollowService. pub and block may be nil.
func NewFollowService(s store.Store, pub FollowPublisher, block BlockPublisher) FollowService {
	return &followService{store: s, pub: pub, block: block}
}

// Follow creates a follow from actor to target. Returns the relationship after the change.
// Errors: ErrValidation (self-follow), ErrNotFound (target), ErrForbidden (block in either direction).
func (svc *followService) Follow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
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
func (svc *followService) Unfollow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
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
func (svc *followService) Block(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
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
func (svc *followService) Unblock(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
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
func (svc *followService) Mute(ctx context.Context, actorAccountID, targetAccountID string, hideNotifications bool) (*domain.Relationship, error) {
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
func (svc *followService) Unmute(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	_ = svc.store.DeleteMute(ctx, actorAccountID, targetAccountID)
	rel, err := svc.store.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Unmute GetRelationship: %w", err)
	}
	return rel, nil
}

// GetFollowByAPID returns the follow by its ActivityPub ID, or ErrNotFound.
func (svc *followService) GetFollowByAPID(ctx context.Context, apID string) (*domain.Follow, error) {
	f, err := svc.store.GetFollowByAPID(ctx, apID)
	if err != nil {
		return nil, fmt.Errorf("GetFollowByAPID: %w", err)
	}
	return f, nil
}

// GetFollow returns the follow relationship from actor to target, or ErrNotFound.
func (svc *followService) GetFollow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Follow, error) {
	f, err := svc.store.GetFollow(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("GetFollow: %w", err)
	}
	return f, nil
}

// CreateFollowFromInbox creates a follow from inbox (remote actor following local target). Does not increment follower/following counts.
func (svc *followService) CreateFollowFromInbox(ctx context.Context, actorAccountID, targetAccountID string, state string, apID *string) (*domain.Follow, error) {
	follow, err := svc.store.CreateFollow(ctx, store.CreateFollowInput{
		ID:        uid.New(),
		AccountID: actorAccountID,
		TargetID:  targetAccountID,
		State:     state,
		APID:      apID,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateFollowFromInbox: %w", err)
	}
	return follow, nil
}

// AcceptFollow marks the follow as accepted (for inbox Accept activity). Does not change counts.
func (svc *followService) AcceptFollow(ctx context.Context, followID string) error {
	if err := svc.store.AcceptFollow(ctx, followID); err != nil {
		return fmt.Errorf("AcceptFollow: %w", err)
	}
	return nil
}

// DeleteFollowFromInbox removes the follow (for inbox Reject/Undo). Does not decrement follower/following counts.
func (svc *followService) DeleteFollowFromInbox(ctx context.Context, actorAccountID, targetAccountID string) error {
	if err := svc.store.DeleteFollow(ctx, actorAccountID, targetAccountID); err != nil {
		return fmt.Errorf("DeleteFollowFromInbox: %w", err)
	}
	return nil
}

// GetFollowers returns the list of followers for the given account (paginated).
func (svc *followService) GetFollowers(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error) {
	list, err := svc.store.GetFollowers(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, fmt.Errorf("GetFollowers: %w", err)
	}
	return list, nil
}

// GetFollowing returns the list of accounts the given account follows (paginated).
func (svc *followService) GetFollowing(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error) {
	list, err := svc.store.GetFollowing(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, fmt.Errorf("GetFollowing: %w", err)
	}
	return list, nil
}
