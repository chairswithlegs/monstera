package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "monstera_fed"

type Metrics struct {
	HTTPRequestsTotal                 *prometheus.CounterVec
	HTTPRequestDurationSeconds        *prometheus.HistogramVec
	FederationDeliveriesTotal         *prometheus.CounterVec
	FederationDeliveryDurationSeconds prometheus.Histogram
	ActiveSSEConnections              *prometheus.GaugeVec
	NATSPublishTotal                  *prometheus.CounterVec
	DBQueryDurationSeconds            *prometheus.HistogramVec
	MediaUploadBytesTotal             *prometheus.CounterVec
	AccountsTotal                     *prometheus.GaugeVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Namespace: namespace, Name: "http_requests_total", Help: "Total HTTP requests"},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Namespace: namespace, Name: "http_request_duration_seconds", Help: "HTTP request duration in seconds"},
			[]string{"method", "path"},
		),
		FederationDeliveriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Namespace: namespace, Name: "federation_deliveries_total", Help: "Total federation delivery attempts"},
			[]string{"result"},
		),
		FederationDeliveryDurationSeconds: prometheus.NewHistogram(
			prometheus.HistogramOpts{Namespace: namespace, Name: "federation_delivery_duration_seconds", Help: "Federation delivery duration in seconds"},
		),
		ActiveSSEConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Namespace: namespace, Name: "active_sse_connections", Help: "Active SSE connections"},
			[]string{"stream"},
		),
		NATSPublishTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Namespace: namespace, Name: "nats_publish_total", Help: "Total NATS publish attempts"},
			[]string{"subject", "result"},
		),
		DBQueryDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{Namespace: namespace, Name: "db_query_duration_seconds", Help: "Database query duration in seconds"},
			[]string{"query_name"},
		),
		MediaUploadBytesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{Namespace: namespace, Name: "media_upload_bytes_total", Help: "Total media upload bytes"},
			[]string{"driver"},
		),
		AccountsTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Namespace: namespace, Name: "accounts_total", Help: "Total accounts"},
			[]string{"type"},
		),
	}

	reg.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDurationSeconds,
		m.FederationDeliveriesTotal,
		m.FederationDeliveryDurationSeconds,
		m.ActiveSSEConnections,
		m.NATSPublishTotal,
		m.DBQueryDurationSeconds,
		m.MediaUploadBytesTotal,
		m.AccountsTotal,
	)

	return m
}

func MetricsMiddleware(m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			duration := time.Since(start).Seconds()
			method := r.Method
			path := r.URL.Path
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				path = rc.RoutePattern()
			}
			status := strconv.Itoa(rec.status)
			m.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
			m.HTTPRequestDurationSeconds.WithLabelValues(method, path).Observe(duration)
		})
	}
}
