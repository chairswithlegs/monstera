package service

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera-fed/internal/store"
)

// NodeInfoStats holds instance-level counts and settings for NodeInfo discovery.
type NodeInfoStats struct {
	UserCount         int64
	LocalPostCount    int64
	OpenRegistrations bool
}

// InstanceService provides instance-level discovery data (NodeInfo).
type InstanceService struct {
	store store.Store
}

// NewInstanceService returns an InstanceService that uses the given store.
func NewInstanceService(s store.Store) *InstanceService {
	return &InstanceService{store: s}
}

// GetNodeInfoStats returns user count, local post count, and open registrations for NodeInfo 2.0.
func (svc *InstanceService) GetNodeInfoStats(ctx context.Context) (*NodeInfoStats, error) {
	userCount, err := svc.store.CountLocalAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("CountLocalAccounts: %w", err)
	}
	postCount, err := svc.store.CountLocalStatuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("CountLocalStatuses: %w", err)
	}
	regMode, _ := svc.store.GetSetting(ctx, "registration_mode")
	return &NodeInfoStats{
		UserCount:         userCount,
		LocalPostCount:    postCount,
		OpenRegistrations: regMode == "open",
	}, nil
}
