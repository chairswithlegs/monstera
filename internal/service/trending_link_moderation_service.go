package service

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/store"
)

// TrendingLinkDenylistService manages the denylist for trending links.
type TrendingLinkDenylistService interface {
	GetDenylist(ctx context.Context) ([]string, error)
	AddDenylist(ctx context.Context, url string) error
	RemoveDenylist(ctx context.Context, url string) error
}

type trendingLinkDenylistService struct {
	store store.Store
}

// NewTrendingLinkDenylistService returns a TrendingLinkDenylistService backed by the given store.
func NewTrendingLinkDenylistService(s store.Store) TrendingLinkDenylistService {
	return &trendingLinkDenylistService{store: s}
}

func (svc *trendingLinkDenylistService) GetDenylist(ctx context.Context) ([]string, error) {
	urls, err := svc.store.ListTrendingLinkDenylist(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListTrendingLinkDenylist: %w", err)
	}
	return urls, nil
}

func (svc *trendingLinkDenylistService) AddDenylist(ctx context.Context, url string) error {
	if err := svc.store.AddTrendingLinkDenylist(ctx, url); err != nil {
		return fmt.Errorf("AddTrendingLinkDenylist: %w", err)
	}
	return nil
}

func (svc *trendingLinkDenylistService) RemoveDenylist(ctx context.Context, url string) error {
	if err := svc.store.RemoveTrendingLinkDenylist(ctx, url); err != nil {
		return fmt.Errorf("RemoveTrendingLinkDenylist: %w", err)
	}
	return nil
}
