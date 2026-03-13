package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// FeaturedTagService handles featured tags on account profiles.
type FeaturedTagService interface {
	ListFeaturedTags(ctx context.Context, accountID string) ([]domain.FeaturedTag, error)
	CreateFeaturedTag(ctx context.Context, accountID, tagName string) (*domain.FeaturedTag, error)
	DeleteFeaturedTag(ctx context.Context, accountID, id string) error
	GetSuggestions(ctx context.Context, accountID string, limit int) ([]domain.Hashtag, []int64, error)
}

type featuredTagService struct {
	store store.Store
}

// NewFeaturedTagService returns a FeaturedTagService.
func NewFeaturedTagService(s store.Store) FeaturedTagService {
	return &featuredTagService{store: s}
}

func (svc *featuredTagService) ListFeaturedTags(ctx context.Context, accountID string) ([]domain.FeaturedTag, error) {
	list, err := svc.store.ListFeaturedTags(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ListFeaturedTags: %w", err)
	}
	return list, nil
}

func (svc *featuredTagService) CreateFeaturedTag(ctx context.Context, accountID, tagName string) (*domain.FeaturedTag, error) {
	tagName = strings.TrimSpace(tagName)
	if tagName == "" {
		return nil, fmt.Errorf("CreateFeaturedTag: %w", domain.ErrValidation)
	}
	tag, err := svc.store.GetOrCreateHashtag(ctx, tagName)
	if err != nil {
		return nil, fmt.Errorf("GetOrCreateHashtag: %w", err)
	}
	id := uid.New()
	if err := svc.store.CreateFeaturedTag(ctx, id, accountID, tag.ID); err != nil {
		return nil, fmt.Errorf("CreateFeaturedTag: %w", err)
	}
	list, err := svc.store.ListFeaturedTags(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("ListFeaturedTags: %w", err)
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return &domain.FeaturedTag{
		ID:            id,
		AccountID:     accountID,
		TagID:         tag.ID,
		Name:          tag.Name,
		StatusesCount: 0,
		LastStatusAt:  nil,
		CreatedAt:     tag.CreatedAt,
	}, nil
}

func (svc *featuredTagService) DeleteFeaturedTag(ctx context.Context, accountID, id string) error {
	_, err := svc.store.GetFeaturedTagByID(ctx, id, accountID)
	if err != nil {
		return fmt.Errorf("GetFeaturedTagByID: %w", err)
	}
	if err := svc.store.DeleteFeaturedTag(ctx, id, accountID); err != nil {
		return fmt.Errorf("DeleteFeaturedTag: %w", err)
	}
	return nil
}

func (svc *featuredTagService) GetSuggestions(ctx context.Context, accountID string, limit int) ([]domain.Hashtag, []int64, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 40 {
		limit = 40
	}
	tags, counts, err := svc.store.ListFeaturedTagSuggestions(ctx, accountID, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("ListFeaturedTagSuggestions: %w", err)
	}
	return tags, counts, nil
}
