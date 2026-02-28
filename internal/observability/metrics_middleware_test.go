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

func TestMetricsMiddleware_incrementsCounter(t *testing.T) {
	t.Parallel()

	prev := defaultMetrics
	t.Cleanup(func() { defaultMetrics = prev })
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	SetMetrics(m)
	r := chi.NewRouter()
	r.Use(MetricsMiddleware())
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
