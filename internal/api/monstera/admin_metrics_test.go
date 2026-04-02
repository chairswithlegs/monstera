package monstera

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAdminMetricsService struct {
	metrics *service.AdminMetrics
	err     error
}

func (f *fakeAdminMetricsService) GetMetrics(ctx context.Context) (*service.AdminMetrics, error) {
	return f.metrics, f.err
}

func TestAdminMetricsHandler_GETMetrics(t *testing.T) {
	t.Parallel()

	t.Run("returns 200 with metrics", func(t *testing.T) {
		t.Parallel()
		svc := &fakeAdminMetricsService{
			metrics: &service.AdminMetrics{
				LocalAccounts:    10,
				RemoteAccounts:   20,
				LocalStatuses:    100,
				RemoteStatuses:   200,
				KnownInstances:   5,
				OpenReports:      3,
				DeliveryDLQDepth: 1,
				FanoutDLQDepth:   2,
			},
		}
		handler := NewAdminMetricsHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/admin/metrics", nil)
		rec := httptest.NewRecorder()
		handler.GETMetrics(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var body apimodel.AdminMetrics
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
		assert.Equal(t, int64(10), body.LocalAccounts)
		assert.Equal(t, int64(20), body.RemoteAccounts)
		assert.Equal(t, int64(100), body.LocalStatuses)
		assert.Equal(t, int64(200), body.RemoteStatuses)
		assert.Equal(t, int64(5), body.KnownInstances)
		assert.Equal(t, int64(3), body.OpenReports)
		assert.Equal(t, int64(1), body.DeliveryDLQDepth)
		assert.Equal(t, int64(2), body.FanoutDLQDepth)
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		t.Parallel()
		svc := &fakeAdminMetricsService{err: errors.New("database down")}
		handler := NewAdminMetricsHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/admin/metrics", nil)
		rec := httptest.NewRecorder()
		handler.GETMetrics(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
