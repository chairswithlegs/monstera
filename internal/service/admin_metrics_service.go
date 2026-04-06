package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"golang.org/x/sync/errgroup"

	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

const adminMetricsCacheKey = "admin:metrics"
const adminMetricsCacheTTL = 60 * time.Second

// AdminMetrics holds aggregated server metrics for the admin dashboard.
type AdminMetrics struct {
	LocalAccounts    int64 `json:"local_accounts"`
	RemoteAccounts   int64 `json:"remote_accounts"`
	LocalStatuses    int64 `json:"local_statuses"`
	RemoteStatuses   int64 `json:"remote_statuses"`
	KnownInstances   int64 `json:"known_instances"`
	OpenReports      int64 `json:"open_reports"`
	DeliveryDLQDepth int64 `json:"delivery_dlq_depth"`
	FanoutDLQDepth   int64 `json:"fanout_dlq_depth"`
}

// AdminMetricsService provides aggregated server metrics for admin visibility.
type AdminMetricsService interface {
	GetMetrics(ctx context.Context) (*AdminMetrics, error)
}

type adminMetricsService struct {
	store           store.Store
	js              jetstream.JetStream
	cache           cache.Store
	cacheKey        string
	deliveryDLQName string
	fanoutDLQName   string
}

// NewAdminMetricsService returns an AdminMetricsService backed by the given
// store, JetStream connection, and local cache.
func NewAdminMetricsService(s store.Store, js jetstream.JetStream, c cache.Store, deliveryDLQ, fanoutDLQ string) AdminMetricsService {
	return &adminMetricsService{
		store:           s,
		js:              js,
		cache:           c,
		cacheKey:        adminMetricsCacheKey + ":" + uid.New(), // Each instance has a unique cache key. This avoids cross-instance collisions in tests.
		deliveryDLQName: deliveryDLQ,
		fanoutDLQName:   fanoutDLQ,
	}
}

func (svc *adminMetricsService) GetMetrics(ctx context.Context) (*AdminMetrics, error) {
	return cache.GetOrSet(ctx, svc.cache, svc.cacheKey, adminMetricsCacheTTL, func() (*AdminMetrics, error) {
		return svc.fetchMetrics(ctx)
	})
}

func (svc *adminMetricsService) fetchMetrics(ctx context.Context) (*AdminMetrics, error) {
	var m AdminMetrics
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		n, err := svc.store.CountLocalAccounts(ctx)
		if err != nil {
			return fmt.Errorf("CountLocalAccounts: %w", err)
		}
		m.LocalAccounts = n
		return nil
	})
	g.Go(func() error {
		n, err := svc.store.CountRemoteAccounts(ctx)
		if err != nil {
			return fmt.Errorf("CountRemoteAccounts: %w", err)
		}
		m.RemoteAccounts = n
		return nil
	})
	g.Go(func() error {
		n, err := svc.store.CountLocalStatuses(ctx)
		if err != nil {
			return fmt.Errorf("CountLocalStatuses: %w", err)
		}
		m.LocalStatuses = n
		return nil
	})
	g.Go(func() error {
		n, err := svc.store.CountRemoteStatuses(ctx)
		if err != nil {
			return fmt.Errorf("CountRemoteStatuses: %w", err)
		}
		m.RemoteStatuses = n
		return nil
	})
	g.Go(func() error {
		n, err := svc.store.CountKnownInstances(ctx)
		if err != nil {
			return fmt.Errorf("CountKnownInstances: %w", err)
		}
		m.KnownInstances = n
		return nil
	})
	g.Go(func() error {
		n, err := svc.store.CountReportsByState(ctx, "open")
		if err != nil {
			return fmt.Errorf("CountReportsByState(open): %w", err)
		}
		m.OpenReports = n
		return nil
	})
	g.Go(func() error {
		n, err := svc.streamMsgCount(ctx, svc.deliveryDLQName)
		if err != nil {
			return fmt.Errorf("DLQ depth %s: %w", svc.deliveryDLQName, err)
		}
		m.DeliveryDLQDepth = n
		return nil
	})
	g.Go(func() error {
		n, err := svc.streamMsgCount(ctx, svc.fanoutDLQName)
		if err != nil {
			return fmt.Errorf("DLQ depth %s: %w", svc.fanoutDLQName, err)
		}
		m.FanoutDLQDepth = n
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (svc *adminMetricsService) streamMsgCount(ctx context.Context, name string) (int64, error) {
	stream, err := svc.js.Stream(ctx, name)
	if err != nil {
		return 0, fmt.Errorf("get stream %s: %w", name, err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return 0, fmt.Errorf("stream info %s: %w", name, err)
	}
	return int64(min(info.State.Msgs, uint64(math.MaxInt64))), nil //#nosec G115 -- capped at MaxInt64
}
