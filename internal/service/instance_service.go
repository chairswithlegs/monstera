package service

import (
	"context"
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/store"
)

// NodeInfoStats holds instance-level counts and settings for NodeInfo discovery.
type NodeInfoStats struct {
	UserCount         int64
	LocalPostCount    int64
	OpenRegistrations bool
}

// InstanceStats holds counts for the Mastodon instance API (v1 stats block).
type InstanceStats struct {
	UserCount   int64
	StatusCount int64
	DomainCount int64
}

// InstanceService provides instance-level discovery data (NodeInfo) and instance settings.
type InstanceService interface {
	GetNodeInfoStats(ctx context.Context) (*NodeInfoStats, error)
	GetInstanceStats(ctx context.Context) (*InstanceStats, error)
	GetAllSettings(ctx context.Context) (map[string]string, error)
	SetSetting(ctx context.Context, key, value string) error
	ListKnownInstances(ctx context.Context, limit, offset int) ([]domain.KnownInstance, error)
}

type instanceService struct {
	store store.Store
}

// NewInstanceService returns an InstanceService that uses the given store.
func NewInstanceService(s store.Store) InstanceService {
	return &instanceService{store: s}
}

// GetNodeInfoStats returns user count, local post count, and open registrations for NodeInfo 2.0.
func (svc *instanceService) GetNodeInfoStats(ctx context.Context) (*NodeInfoStats, error) {
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

// GetInstanceStats returns user, status, and domain counts for the Mastodon instance API.
func (svc *instanceService) GetInstanceStats(ctx context.Context) (*InstanceStats, error) {
	userCount, err := svc.store.CountLocalAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("CountLocalAccounts: %w", err)
	}
	statusCount, err := svc.store.CountLocalStatuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("CountLocalStatuses: %w", err)
	}
	domainCount, err := svc.store.CountKnownInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("CountKnownInstances: %w", err)
	}
	return &InstanceStats{
		UserCount:   userCount,
		StatusCount: statusCount,
		DomainCount: domainCount,
	}, nil
}

// GetAllSettings returns all instance settings as a map from the store.
func (svc *instanceService) GetAllSettings(ctx context.Context) (map[string]string, error) {
	m, err := svc.store.ListSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListSettings: %w", err)
	}
	return m, nil
}

// SetSetting writes a single setting to the store.
func (svc *instanceService) SetSetting(ctx context.Context, key, value string) error {
	if err := svc.store.SetSetting(ctx, key, value); err != nil {
		return fmt.Errorf("SetSetting(%s): %w", key, err)
	}
	return nil
}

// ListKnownInstances returns known federated instances for admin.
func (svc *instanceService) ListKnownInstances(ctx context.Context, limit, offset int) ([]domain.KnownInstance, error) {
	instances, err := svc.store.ListKnownInstances(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("ListKnownInstances: %w", err)
	}
	return instances, nil
}
