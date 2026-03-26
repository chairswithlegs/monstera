package observability

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type contextKey int

const (
	requestIDKey contextKey = iota
	accountIDKey
)

func NewLogger(env, level string) *slog.Logger {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: parseLevel(level),
	}
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(&monsteraLogHandler{handler: handler})
}

// monsteraLogHandler is a thin wrapper around the default slog.Handler
// It exists to automatically add common internal attributes to the record.
type monsteraLogHandler struct {
	handler slog.Handler
}

func (h *monsteraLogHandler) Handle(ctx context.Context, r slog.Record) error {
	requestID := RequestIDFromContext(ctx)
	if requestID != "" {
		r.AddAttrs(slog.String("request_id", requestID))
	}
	accountID := AccountIDFromContext(ctx)
	if accountID != "" {
		r.AddAttrs(slog.String("account_id", accountID))
	}
	err := h.handler.Handle(ctx, r)
	if err != nil {
		return fmt.Errorf("monstera log handler: %w", err)
	}
	return nil
}

func (h *monsteraLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *monsteraLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.handler.WithAttrs(attrs)
}

func (h *monsteraLogHandler) WithGroup(name string) slog.Handler {
	return h.handler.WithGroup(name)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// RequestIDMiddleware generates a request ID and stores it in the request context.
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := generateRequestID()
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b[:4]) + "-" + hex.EncodeToString(b[4:6]) + "-" + hex.EncodeToString(b[6:8]) + "-" + hex.EncodeToString(b[8:10]) + "-" + hex.EncodeToString(b[10:])
}

// RequestLoggerMiddleware is a middleware that logs the request.
func RequestLoggerMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			// Don't log successful health checks
			if strings.HasPrefix(r.URL.Path, "/healthz") && rec.status == http.StatusOK {
				return
			}

			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			}
			slog.InfoContext(r.Context(), "request", attrs...)
		})
	}
}

// responseRecorder is a wrapper around the http.ResponseWriter that records the status code.
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker so that WebSocket upgrades work through this wrapper.
func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("responseRecorder: underlying ResponseWriter does not implement http.Hijacker")
	}
	conn, rw, err := h.Hijack()
	if err != nil {
		return nil, nil, fmt.Errorf("responseRecorder: hijack: %w", err)
	}
	return conn, rw, nil
}

// RequestIDFromContext retrieves the request ID that was added to the context by the RequestIDMiddleware.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// WithAccountID stores the account ID in the context.
func WithAccountID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, accountIDKey, id)
}

// AccountIDFromContext retrieves the account ID from the context.
func AccountIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(accountIDKey).(string)
	return id
}
