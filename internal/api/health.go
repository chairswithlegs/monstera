package api

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

// HealthChecker holds the dependencies needed to execute readiness checks.
type HealthChecker struct {
	DB   *pgxpool.Pool
	NATS *nats.Conn
}

// NewHealthChecker constructs a HealthChecker.
func NewHealthChecker(db *pgxpool.Pool, nc *nats.Conn) *HealthChecker {
	return &HealthChecker{DB: db, NATS: nc}
}

// GETLiveness handles GET /healthz/live.
// Always returns 200 OK. Used as the Kubernetes livenessProbe.
func (h *HealthChecker) GETLiveness(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readinessResponse is the body for the readiness endpoint.
type readinessResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// GETReadiness handles GET /healthz/ready.
// Pings PostgreSQL and NATS within a 2-second deadline.
// Returns 200 if both pass; 503 if either fails.
func (h *HealthChecker) GETReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	const checkError = "error"
	const checkOK = "ok"
	checks := make(map[string]string)
	status := checkOK

	if h.DB != nil {
		if err := h.DB.Ping(ctx); err != nil {
			checks["postgres"] = checkError
			status = checkError
		} else {
			checks["postgres"] = checkOK
		}
	}

	if h.NATS != nil && !h.NATS.IsClosed() {
		if h.NATS.Status() != nats.CONNECTED {
			checks["nats"] = checkError
			status = checkError
		} else {
			checks["nats"] = checkOK
		}
	}

	resp := readinessResponse{Status: status, Checks: checks}
	code := http.StatusOK
	if status == checkError {
		code = http.StatusServiceUnavailable
	}
	WriteJSON(w, code, resp)
}
