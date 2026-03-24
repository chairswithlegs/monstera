package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

type fakeScheduledStatusService struct {
	publishDueErr    error
	publishDueCalls  int
	publishDueCounts []int
}

func (f *fakeScheduledStatusService) CreateScheduledStatus(context.Context, string, []byte, time.Time) (*domain.ScheduledStatus, error) {
	return nil, nil
}

func (f *fakeScheduledStatusService) UpdateScheduledStatus(context.Context, string, string, []byte, time.Time) (*domain.ScheduledStatus, error) {
	return nil, nil
}

func (f *fakeScheduledStatusService) DeleteScheduledStatus(context.Context, string, string) error {
	return nil
}

func (f *fakeScheduledStatusService) PublishDueStatuses(_ context.Context, _ int) (int, error) {
	idx := f.publishDueCalls
	f.publishDueCalls++
	if f.publishDueErr != nil {
		return 0, f.publishDueErr
	}
	if idx < len(f.publishDueCounts) {
		return f.publishDueCounts[idx], nil
	}
	return 0, nil
}

type fakeTrendsService struct {
	service.TrendsService
	refreshErr   error
	refreshCalls int
}

func (f *fakeTrendsService) RefreshIndexes(_ context.Context) error {
	f.refreshCalls++
	return f.refreshErr
}

func (f *fakeTrendsService) TrendingStatuses(_ context.Context, _, _ int) ([]service.EnrichedStatus, error) {
	return nil, nil
}

func (f *fakeTrendsService) TrendingTags(_ context.Context, _, _ int) ([]domain.TrendingTag, error) {
	return nil, nil
}

func TestScheduledStatuses_drains_queue(t *testing.T) {
	t.Parallel()

	svc := &fakeScheduledStatusService{
		publishDueCounts: []int{100, 100, 42},
	}
	handler := ScheduledStatuses(svc)
	err := handler(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, svc.publishDueCalls)
}

func TestScheduledStatuses_single_batch(t *testing.T) {
	t.Parallel()

	svc := &fakeScheduledStatusService{
		publishDueCounts: []int{5},
	}
	handler := ScheduledStatuses(svc)
	err := handler(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, svc.publishDueCalls)
}

func TestScheduledStatuses_propagatesError(t *testing.T) {
	t.Parallel()

	svc := &fakeScheduledStatusService{publishDueErr: errors.New("store down")}
	handler := ScheduledStatuses(svc)
	err := handler(context.Background())
	require.Error(t, err)
}

func TestUpdateTrendingIndexes_success(t *testing.T) {
	t.Parallel()

	svc := &fakeTrendsService{}
	handler := UpdateTrendingIndexes(svc)
	err := handler(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, svc.refreshCalls)
}

func TestUpdateTrendingIndexes_propagatesError(t *testing.T) {
	t.Parallel()

	svc := &fakeTrendsService{refreshErr: errors.New("db error")}
	handler := UpdateTrendingIndexes(svc)
	err := handler(context.Background())
	require.Error(t, err)
}

type fakeCardService struct {
	fetchErr    error
	fetchCalls  int
	fetchCounts []int
}

func (f *fakeCardService) ProcessPendingCards(_ context.Context, _ int) (int, error) {
	idx := f.fetchCalls
	f.fetchCalls++
	if f.fetchErr != nil {
		return 0, f.fetchErr
	}
	if idx < len(f.fetchCounts) {
		return f.fetchCounts[idx], nil
	}
	return 0, nil
}

func TestProcessPendingCards_drains_queue(t *testing.T) {
	t.Parallel()

	svc := &fakeCardService{
		fetchCounts: []int{100, 50},
	}
	handler := ProcessPendingCards(svc)
	err := handler(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, svc.fetchCalls)
}

func TestProcessPendingCards_propagatesError(t *testing.T) {
	t.Parallel()

	svc := &fakeCardService{fetchErr: errors.New("store down")}
	handler := ProcessPendingCards(svc)
	err := handler(context.Background())
	require.Error(t, err)
}
