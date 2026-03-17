package webpush

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	wp "github.com/SherClockHolmes/webpush-go"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/ssrf"
)

const (
	pushTimeout         = 30 * time.Second
	maxResponseBodyRead = 4096 // bytes to drain before discarding
)

// ErrSubscriptionGone indicates the push endpoint returned 410 Gone.
var ErrSubscriptionGone = errors.New("webpush: subscription gone")

// Sender sends Web Push notifications.
type Sender interface {
	Send(ctx context.Context, sub *domain.PushSubscription, payload []byte) error
}

type sender struct {
	vapidPublicKey  string
	vapidPrivateKey string
	subscriberURL   string // mailto: or https:// contact
	httpClient      *http.Client
}

// NewSender creates a Sender that signs with the given VAPID keys.
// It uses an SSRF-hardened HTTP client to prevent requests to internal addresses.
func NewSender(vapidPublicKey, vapidPrivateKey, subscriberURL string) Sender {
	return &sender{
		vapidPublicKey:  vapidPublicKey,
		vapidPrivateKey: vapidPrivateKey,
		subscriberURL:   subscriberURL,
		httpClient:      ssrf.NewHTTPClient(ssrf.HTTPClientOptions{Timeout: pushTimeout}),
	}
}

// newSenderWithClient creates a Sender with a custom HTTP client (for testing).
func newSenderWithClient(vapidPublicKey, vapidPrivateKey, subscriberURL string, client *http.Client) Sender {
	return &sender{
		vapidPublicKey:  vapidPublicKey,
		vapidPrivateKey: vapidPrivateKey,
		subscriberURL:   subscriberURL,
		httpClient:      client,
	}
}

// ctxHTTPClient wraps an http.Client to apply a context with timeout to every request.
type ctxHTTPClient struct {
	inner *http.Client
	ctx   context.Context //nolint:containedctx // passed through to each request
}

func (c *ctxHTTPClient) Do(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(c.ctx, pushTimeout)
	defer cancel()
	resp, err := c.inner.Do(req.WithContext(ctx)) //nolint:gosec // G704: inner is ssrf.NewHTTPClient in production; tests inject http.DefaultClient against localhost
	if err != nil {
		return nil, fmt.Errorf("webpush: do request: %w", err)
	}
	return resp, nil
}

func (s *sender) Send(ctx context.Context, sub *domain.PushSubscription, payload []byte) error {
	subscription := &wp.Subscription{
		Endpoint: sub.Endpoint,
		Keys: wp.Keys{
			P256dh: sub.KeyP256DH,
			Auth:   sub.KeyAuth,
		},
	}
	resp, err := wp.SendNotification(payload, subscription, &wp.Options{
		HTTPClient:      &ctxHTTPClient{inner: s.httpClient, ctx: ctx},
		Subscriber:      s.subscriberURL,
		VAPIDPublicKey:  s.vapidPublicKey,
		VAPIDPrivateKey: s.vapidPrivateKey,
		TTL:             86400,
		Urgency:         wp.UrgencyHigh,
	})
	if err != nil {
		return fmt.Errorf("webpush: send: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBodyRead))
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusGone {
		return ErrSubscriptionGone
	}
	if resp.StatusCode >= 400 {
		slog.WarnContext(ctx, "webpush: push endpoint returned error",
			slog.Int("status", resp.StatusCode),
			slog.String("endpoint", sub.Endpoint),
		)
		return fmt.Errorf("webpush: endpoint returned %d", resp.StatusCode)
	}
	return nil
}
