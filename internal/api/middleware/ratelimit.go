package middleware

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/ratelimit"
)

// RateLimit returns a middleware that rate limits by extracted key.
// keyFn extracts the rate limit key from the request; if it returns "",
// the request passes through without limiting.
func RateLimit(lim ratelimit.Limiter, limit int, window time.Duration, keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			result, err := lim.Allow(r.Context(), key, limit, window)
			if err != nil {
				slog.WarnContext(r.Context(), "rate limit check failed, allowing request", slog.Any("error", err))
				next.ServeHTTP(w, r)
				return
			}
			setRateLimitHeaders(w, result)
			if !result.Allowed {
				retryAfter := int(time.Until(result.ResetAt).Seconds()) + 1
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				api.HandleError(w, r, fmt.Errorf("%w", domain.ErrRateLimited))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitByAccount rate limits authenticated requests by account ID.
func RateLimitByAccount(lim ratelimit.Limiter, limit int, window time.Duration) func(http.Handler) http.Handler {
	return RateLimit(lim, limit, window, func(r *http.Request) string {
		if acct := AccountFromContext(r.Context()); acct != nil {
			return "acct:" + acct.ID
		}
		return ""
	})
}

// RateLimitByIP rate limits by client IP address.
func RateLimitByIP(lim ratelimit.Limiter, limit int, window time.Duration) func(http.Handler) http.Handler {
	return RateLimit(lim, limit, window, func(r *http.Request) string {
		return "ip:" + clientIP(r)
	})
}

func setRateLimitHeaders(w http.ResponseWriter, r ratelimit.Result) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(r.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(r.Remaining))
	w.Header().Set("X-RateLimit-Reset", r.ResetAt.UTC().Format(time.RFC3339))
}

func clientIP(r *http.Request) string {
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}
	return r.RemoteAddr
}
