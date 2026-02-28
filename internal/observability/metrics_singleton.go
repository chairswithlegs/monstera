package observability

// defaultMetrics is the process-wide metrics instance. Set via SetMetrics at startup.
// If nil, wrapper functions no-op so callers need not check.
var defaultMetrics *Metrics

// SetMetrics sets the global metrics instance used by IncNATSPublish and other
// singleton wrappers. Call once at startup after creating Metrics (e.g. in serve).
// Similar to slog.SetDefault for logging.
func SetMetrics(m *Metrics) {
	defaultMetrics = m
}

// IncNATSPublish increments the NATS publish counter for the given subject and result
// ("ok" or "error"). No-ops if SetMetrics has not been called.
func IncNATSPublish(subject, result string) {
	if defaultMetrics != nil {
		defaultMetrics.NATSPublishTotal.WithLabelValues(subject, result).Inc()
	}
}

// RecordHTTPRequest increments the HTTP request counter for the given method, path, and status.
// No-ops if SetMetrics has not been called.
func RecordHTTPRequest(method, path, status string) {
	if defaultMetrics != nil {
		defaultMetrics.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
	}
}

// ObserveHTTPRequestDuration records the request duration for the given method and path.
// No-ops if SetMetrics has not been called.
func ObserveHTTPRequestDuration(method, path string, durationSeconds float64) {
	if defaultMetrics != nil {
		defaultMetrics.HTTPRequestDurationSeconds.WithLabelValues(method, path).Observe(durationSeconds)
	}
}
