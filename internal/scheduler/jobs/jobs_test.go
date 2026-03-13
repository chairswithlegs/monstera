package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// fakeStatusWriteService stubs service.StatusWriteService for job tests.
type fakeStatusWriteService struct {
	service.StatusWriteService
	publishDueErr   error
	publishDueCalls int
}

func (f *fakeStatusWriteService) PublishDueStatuses(_ context.Context, _ int) error {
	f.publishDueCalls++
	return f.publishDueErr
}

// fakeTrendsService stubs service.TrendsService for job tests.
type fakeTrendsService struct {
	service.TrendsService
	refreshErr   error
	refreshCalls int
}

func (f *fakeTrendsService) RefreshIndexes(_ context.Context) error {
	f.refreshCalls++
	return f.refreshErr
}

func (f *fakeTrendsService) TrendingStatuses(_ context.Context, _ int) ([]service.EnrichedStatus, error) {
	return nil, nil
}

func (f *fakeTrendsService) TrendingTags(_ context.Context, _ int) ([]domain.TrendingTag, error) {
	return nil, nil
}

func TestScheduledStatuses_success(t *testing.T) {
	t.Parallel()

	svc := &fakeStatusWriteService{}
	handler := ScheduledStatuses(svc)
	err := handler(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, svc.publishDueCalls)
}

func TestScheduledStatuses_propagatesError(t *testing.T) {
	t.Parallel()

	svc := &fakeStatusWriteService{publishDueErr: errors.New("store down")}
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

// fakeCardService stubs service.CardService for job tests.
type fakeCardService struct {
	fetchErr   error
	fetchCalls int
}

func (f *fakeCardService) ProcessPendingCards(_ context.Context, _ int) (int, error) {
	f.fetchCalls++
	return 0, f.fetchErr
}

func TestProcessPendingCards_success(t *testing.T) {
	t.Parallel()

	svc := &fakeCardService{}
	handler := ProcessPendingCards(svc)
	err := handler(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, svc.fetchCalls)
}

func TestProcessPendingCards_propagatesError(t *testing.T) {
	t.Parallel()

	svc := &fakeCardService{fetchErr: errors.New("store down")}
	handler := ProcessPendingCards(svc)
	err := handler(context.Background())
	require.Error(t, err)
}
