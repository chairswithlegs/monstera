package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// TagFollowService handles hashtag follow/unfollow operations.
type TagFollowService interface {
	GetTagByName(ctx context.Context, name string) (*domain.Hashtag, error)
	IsFollowingTag(ctx context.Context, accountID, tagID string) (bool, error)
	ListFollowedTags(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Hashtag, *string, error)
	AreFollowingTagsByName(ctx context.Context, accountID string, tagNames []string) (map[string]bool, error)
	FollowTag(ctx context.Context, accountID, tagName string) (*domain.Hashtag, error)
	UnfollowTag(ctx context.Context, accountID, tagID string) error
	UnfollowTagByName(ctx context.Context, accountID, tagName string) (*domain.Hashtag, error)
}

type tagFollowService struct {
	store store.Store
}

func NewTagFollowService(s store.Store) TagFollowService {
	return &tagFollowService{store: s}
}

func (svc *tagFollowService) GetTagByName(ctx context.Context, name string) (*domain.Hashtag, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return nil, fmt.Errorf("GetTagByName: %w", domain.ErrValidation)
	}
	tag, err := svc.store.GetHashtagByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("GetTagByName: %w", err)
	}
	return tag, nil
}

func (svc *tagFollowService) IsFollowingTag(ctx context.Context, accountID, tagID string) (bool, error) {
	following, err := svc.store.IsFollowingTag(ctx, accountID, tagID)
	if err != nil {
		return false, fmt.Errorf("IsFollowingTag: %w", err)
	}
	return following, nil
}

func (svc *tagFollowService) ListFollowedTags(ctx context.Context, accountID string, maxID *string, limit int) ([]domain.Hashtag, *string, error) {
	limit = ClampLimit(limit, DefaultServiceListLimit, MaxServicePageLimit)
	tags, next, err := svc.store.ListFollowedTags(ctx, accountID, maxID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("ListFollowedTags: %w", err)
	}
	return tags, next, nil
}

func (svc *tagFollowService) AreFollowingTagsByName(ctx context.Context, accountID string, tagNames []string) (map[string]bool, error) {
	result, err := svc.store.AreFollowingTagsByName(ctx, accountID, tagNames)
	if err != nil {
		return nil, fmt.Errorf("AreFollowingTagsByName: %w", err)
	}
	return result, nil
}

func (svc *tagFollowService) FollowTag(ctx context.Context, accountID, tagName string) (*domain.Hashtag, error) {
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

func (svc *tagFollowService) UnfollowTag(ctx context.Context, accountID, tagID string) error {
	if err := svc.store.UnfollowTag(ctx, accountID, tagID); err != nil {
		return fmt.Errorf("UnfollowTag: %w", err)
	}
	return nil
}

func (svc *tagFollowService) UnfollowTagByName(ctx context.Context, accountID, tagName string) (*domain.Hashtag, error) {
	tagName = strings.TrimSpace(strings.ToLower(tagName))
	if tagName == "" {
		return nil, fmt.Errorf("UnfollowTagByName: %w", domain.ErrValidation)
	}
	tag, err := svc.store.GetHashtagByName(ctx, tagName)
	if err != nil {
		return nil, fmt.Errorf("UnfollowTagByName: %w", err)
	}
	if err := svc.store.UnfollowTag(ctx, accountID, tag.ID); err != nil {
		return nil, fmt.Errorf("UnfollowTagByName: %w", err)
	}
	return tag, nil
}
