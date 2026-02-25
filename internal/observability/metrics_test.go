package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics_registersAllCollectors(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	require.NotNil(t, m)
	assert.NotNil(t, m.HTTPRequestsTotal)
	assert.NotNil(t, m.HTTPRequestDurationSeconds)
	assert.NotNil(t, m.FederationDeliveriesTotal)
	assert.NotNil(t, m.ActiveSSEConnections)
	assert.NotNil(t, m.NATSPublishTotal)
	assert.NotNil(t, m.DBQueryDurationSeconds)
	assert.NotNil(t, m.MediaUploadBytesTotal)
	assert.NotNil(t, m.AccountsTotal)
}

func TestMetricsMiddleware_incrementsCounter(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	r := chi.NewRouter()
	r.Use(MetricsMiddleware(m))
	r.Get("/api/v1/accounts/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/01ABC", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	metrics, err := reg.Gather()
	require.NoError(t, err)
	var found bool
	for _, mf := range metrics {
		if mf.GetName() == "monstera_fed_http_requests_total" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected http_requests_total metric to be recorded")
}
