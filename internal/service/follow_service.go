package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	CreateRemoteFollow(ctx context.Context, actorAccountID, targetAccountID string, state string, apID *string) (*domain.Follow, error)
	AcceptFollow(ctx context.Context, followID string) error
	DeleteRemoteFollow(ctx context.Context, actorAccountID, targetAccountID string) error
	GetFollowers(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error)
	GetFollowing(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, error)
	ListPendingFollowRequests(ctx context.Context, targetAccountID string, maxID *string, limit int) ([]domain.Account, *string, error)
	ListBlockedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error)
	ListMutedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error)
	AuthorizeFollowRequest(ctx context.Context, targetAccountID, requesterAccountID string) error
	RejectFollowRequest(ctx context.Context, targetAccountID, requesterAccountID string) error
	ListFollowedTags(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Hashtag, *string, error)
	FollowTag(ctx context.Context, accountID, tagName string) (*domain.Hashtag, error)
	UnfollowTag(ctx context.Context, accountID, tagID string) error
}

type followService struct {
	store    store.Store
	accounts AccountService
	pub      FollowPublisher
	block    BlockPublisher
}

// NewFollowService returns a FollowService. pub and block may be nil.
func NewFollowService(s store.Store, accounts AccountService, pub FollowPublisher, block BlockPublisher) FollowService {
	return &followService{store: s, accounts: accounts, pub: pub, block: block}
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
	rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
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
			rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
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

	rel, err = svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Follow GetRelationship after: %w", err)
	}
	return rel, nil
}

// Unfollow removes the follow from actor to target. Returns the relationship after the change.
func (svc *followService) Unfollow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	follow, err := svc.store.GetFollow(ctx, actorAccountID, targetAccountID)
	if err != nil {
		rel, _ := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
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

	rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
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
	rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
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
	rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
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
	rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
	if err != nil {
		return nil, fmt.Errorf("Mute GetRelationship: %w", err)
	}
	return rel, nil
}

// Unmute removes the mute from actor to target. Returns the relationship.
func (svc *followService) Unmute(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Relationship, error) {
	_ = svc.store.DeleteMute(ctx, actorAccountID, targetAccountID)
	rel, err := svc.accounts.GetRelationship(ctx, actorAccountID, targetAccountID)
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

// CreateRemoteFollow creates a follow from a remote actor to a local target.
// When state is accepted (e.g. target not locked), increments follower/following counts.
func (svc *followService) CreateRemoteFollow(ctx context.Context, actorAccountID, targetAccountID string, state string, apID *string) (*domain.Follow, error) {
	var follow *domain.Follow
	err := svc.store.WithTx(ctx, func(tx store.Store) error {
		var txErr error
		follow, txErr = tx.CreateFollow(ctx, store.CreateFollowInput{
			ID:        uid.New(),
			AccountID: actorAccountID,
			TargetID:  targetAccountID,
			State:     state,
			APID:      apID,
		})
		if txErr != nil {
			return fmt.Errorf("CreateFollow: %w", txErr)
		}
		if state == domain.FollowStateAccepted {
			if txErr := tx.IncrementFollowersCount(ctx, targetAccountID); txErr != nil {
				return fmt.Errorf("IncrementFollowersCount: %w", txErr)
			}
			if txErr := tx.IncrementFollowingCount(ctx, actorAccountID); txErr != nil {
				return fmt.Errorf("IncrementFollowingCount: %w", txErr)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("CreateRemoteFollow: %w", err)
	}
	return follow, nil
}

// AcceptFollow marks the follow as accepted (for inbox Accept activity) and increments follower/following counts.
func (svc *followService) AcceptFollow(ctx context.Context, followID string) error {
	follow, err := svc.store.GetFollowByID(ctx, followID)
	if err != nil {
		return fmt.Errorf("AcceptFollow GetFollowByID: %w", err)
	}
	if follow.State == domain.FollowStateAccepted {
		return nil
	}
	err = svc.store.WithTx(ctx, func(tx store.Store) error {
		if txErr := tx.AcceptFollow(ctx, followID); txErr != nil {
			return fmt.Errorf("AcceptFollow: %w", txErr)
		}
		if txErr := tx.IncrementFollowersCount(ctx, follow.TargetID); txErr != nil {
			return fmt.Errorf("IncrementFollowersCount: %w", txErr)
		}
		if txErr := tx.IncrementFollowingCount(ctx, follow.AccountID); txErr != nil {
			return fmt.Errorf("IncrementFollowingCount: %w", txErr)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("AcceptFollow tx: %w", err)
	}
	return nil
}

// DeleteRemoteFollow removes the follow (e.g. Reject/Undo from federation). Does not decrement follower/following counts.
func (svc *followService) DeleteRemoteFollow(ctx context.Context, actorAccountID, targetAccountID string) error {
	if err := svc.store.DeleteFollow(ctx, actorAccountID, targetAccountID); err != nil {
		return fmt.Errorf("DeleteRemoteFollow: %w", err)
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

// ListPendingFollowRequests returns accounts that have requested to follow the target (paginated).
func (svc *followService) ListPendingFollowRequests(ctx context.Context, targetAccountID string, maxID *string, limit int) ([]domain.Account, *string, error) {
	accounts, nextCursor, err := svc.store.GetPendingFollowRequests(ctx, targetAccountID, maxID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("GetPendingFollowRequests: %w", err)
	}
	return accounts, nextCursor, nil
}

// ListBlockedAccounts returns blocked accounts for the given account (paginated by block id).
func (svc *followService) ListBlockedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error) {
	if limit <= 0 {
		limit = DefaultServiceListLimit
	}
	if limit > MaxServicePageLimit {
		limit = MaxServicePageLimit
	}
	accounts, nextCursor, err := svc.store.ListBlockedAccounts(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("ListBlockedAccounts: %w", err)
	}
	return accounts, nextCursor, nil
}

// ListMutedAccounts returns muted accounts for the given account (paginated by mute id).
func (svc *followService) ListMutedAccounts(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Account, *string, error) {
	if limit <= 0 {
		limit = DefaultServiceListLimit
	}
	if limit > MaxServicePageLimit {
		limit = MaxServicePageLimit
	}
	accounts, nextCursor, err := svc.store.ListMutedAccounts(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("ListMutedAccounts: %w", err)
	}
	return accounts, nextCursor, nil
}

// AuthorizeFollowRequest accepts a pending follow request (requesterAccountID requested to follow targetAccountID).
func (svc *followService) AuthorizeFollowRequest(ctx context.Context, targetAccountID, requesterAccountID string) error {
	follow, err := svc.store.GetFollow(ctx, requesterAccountID, targetAccountID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("follow request: %w", domain.ErrNotFound)
		}
		return fmt.Errorf("GetFollow: %w", err)
	}
	if follow.State != domain.FollowStatePending {
		return nil
	}
	if err := svc.AcceptFollow(ctx, follow.ID); err != nil {
		return fmt.Errorf("AcceptFollow: %w", err)
	}
	return nil
}

// RejectFollowRequest rejects a pending follow request.
func (svc *followService) RejectFollowRequest(ctx context.Context, targetAccountID, requesterAccountID string) error {
	follow, err := svc.store.GetFollow(ctx, requesterAccountID, targetAccountID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("follow request: %w", domain.ErrNotFound)
		}
		return fmt.Errorf("GetFollow: %w", err)
	}
	if follow.State != domain.FollowStatePending {
		return nil
	}
	if err := svc.store.DeleteFollow(ctx, requesterAccountID, targetAccountID); err != nil {
		return fmt.Errorf("DeleteFollow: %w", err)
	}
	return nil
}

// ListFollowedTags returns tags the account follows, paginated.
func (svc *followService) ListFollowedTags(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Hashtag, *string, error) {
	if limit <= 0 {
		limit = DefaultServiceListLimit
	}
	if limit > MaxServicePageLimit {
		limit = MaxServicePageLimit
	}
	tags, next, err := svc.store.ListFollowedTags(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("ListFollowedTags: %w", err)
	}
	return tags, next, nil
}

// FollowTag resolves the tag by name (creating it if needed), then records the follow.
func (svc *followService) FollowTag(ctx context.Context, accountID, tagName string) (*domain.Hashtag, error) {
	tagName = strings.TrimSpace(tagName)
	if tagName == "" {
		return nil, fmt.Errorf("FollowTag: %w", domain.ErrValidation)
	}
	tag, err := svc.store.GetOrCreateHashtag(ctx, tagName)
	if err != nil {
		return nil, fmt.Errorf("GetOrCreateHashtag: %w", err)
	}
	rowID := uid.New()
	if err := svc.store.FollowTag(ctx, rowID, accountID, tag.ID); err != nil {
		return nil, fmt.Errorf("FollowTag: %w", err)
	}
	return tag, nil
}

// UnfollowTag removes the follow for the given tag ID.
func (svc *followService) UnfollowTag(ctx context.Context, accountID, tagID string) error {
	if err := svc.store.UnfollowTag(ctx, accountID, tagID); err != nil {
		return fmt.Errorf("UnfollowTag: %w", err)
	}
	return nil
}
