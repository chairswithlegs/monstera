package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Stats collects per-request latency and status code counts in a thread-safe manner.
type Stats struct {
	mu        sync.Mutex
	latencies []time.Duration

	total     atomic.Int64
	success   atomic.Int64
	client4xx atomic.Int64
	server5xx atomic.Int64
}

// Record adds a single request result.
func (s *Stats) Record(latency time.Duration, statusCode int) {
	s.total.Add(1)
	switch {
	case statusCode >= 200 && statusCode < 300:
		s.success.Add(1)
	case statusCode >= 400 && statusCode < 500:
		s.client4xx.Add(1)
	case statusCode >= 500:
		s.server5xx.Add(1)
	}
	s.mu.Lock()
	s.latencies = append(s.latencies, latency)
	s.mu.Unlock()
}

// Percentile returns the p-th percentile latency (0–100). Returns 0 if no data.
func (s *Stats) Percentile(p int) time.Duration {
	s.mu.Lock()
	lats := make([]time.Duration, len(s.latencies))
	copy(lats, s.latencies)
	s.mu.Unlock()

	if len(lats) == 0 {
		return 0
	}
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	idx := p * (len(lats) - 1) / 100
	return lats[idx]
}

// InboxResult is the summary for the inbox flood scenario.
type InboxResult struct {
	Requests  int64   `json:"requests"`
	Successes int64   `json:"successes"`
	C4xx      int64   `json:"4xx"`
	C5xx      int64   `json:"5xx"`
	RPS       float64 `json:"rps"`
	Duration  float64 `json:"duration_s"`
	P50Ms     int64   `json:"p50_ms"`
	P95Ms     int64   `json:"p95_ms"`
	P99Ms     int64   `json:"p99_ms"`
}

// PrintInboxResult writes the inbox results as a table or JSON.
func PrintInboxResult(w io.Writer, r InboxResult, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}
	_, _ = fmt.Fprintf(w, "\nInbox Flood Results\n")
	_, _ = fmt.Fprintf(w, "  Requests    %d\n", r.Requests)
	_, _ = fmt.Fprintf(w, "  Successes   %d\n", r.Successes)
	_, _ = fmt.Fprintf(w, "  4xx         %d\n", r.C4xx)
	_, _ = fmt.Fprintf(w, "  5xx         %d\n", r.C5xx)
	_, _ = fmt.Fprintf(w, "  RPS         %.1f\n", r.RPS)
	_, _ = fmt.Fprintf(w, "  Duration    %.1fs\n", r.Duration)
	_, _ = fmt.Fprintf(w, "  p50         %dms\n", r.P50Ms)
	_, _ = fmt.Fprintf(w, "  p95         %dms\n", r.P95Ms)
	_, _ = fmt.Fprintf(w, "  p99         %dms\n", r.P99Ms)
}

// FanoutResult is the summary for the outbox fan-out scenario.
type FanoutResult struct {
	FollowersSeeded    int     `json:"followers_seeded"`
	Instances          int     `json:"instances"`
	DeliveriesExpected int     `json:"deliveries_expected"`
	DeliveriesReceived int     `json:"deliveries_received"`
	FirstDeliveryS     float64 `json:"first_delivery_s"`
	LastDeliveryS      float64 `json:"last_delivery_s"`
	DeliveryRate       float64 `json:"delivery_rate"`
	NATSLagBefore      uint64  `json:"nats_lag_before"`
	NATSLagAfter       uint64  `json:"nats_lag_after"`
}

// PrintFanoutResult writes the fanout results as a table or JSON.
func PrintFanoutResult(w io.Writer, r FanoutResult, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}
	_, _ = fmt.Fprintf(w, "\nFanout Results\n")
	_, _ = fmt.Fprintf(w, "  Followers seeded    %d\n", r.FollowersSeeded)
	_, _ = fmt.Fprintf(w, "  Instances           %d\n", r.Instances)
	_, _ = fmt.Fprintf(w, "  Deliveries expected %d\n", r.DeliveriesExpected)
	_, _ = fmt.Fprintf(w, "  Deliveries received %d\n", r.DeliveriesReceived)
	_, _ = fmt.Fprintf(w, "  First delivery      %.1fs\n", r.FirstDeliveryS)
	_, _ = fmt.Fprintf(w, "  Last delivery       %.1fs\n", r.LastDeliveryS)
	_, _ = fmt.Fprintf(w, "  Delivery rate       %.0f/s\n", r.DeliveryRate)
	_, _ = fmt.Fprintf(w, "  NATS lag (before)   %d\n", r.NATSLagBefore)
	_, _ = fmt.Fprintf(w, "  NATS lag (after)    %d\n", r.NATSLagAfter)
}
