package federation

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera-fed/internal/activitypub"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
	"github.com/chairswithlegs/monstera-fed/internal/store"
)

const (
	streamNameFederation = "FEDERATION"
	consumerName         = "federation-worker"
	maxDeliver           = 5
)

// FederationWorker consumes delivery jobs from the FEDERATION JetStream stream
// and POSTs AP activities to remote inboxes with HTTP Signature authentication.
type FederationWorker struct {
	js        jetstream.JetStream
	producer  *Producer
	store     store.Store
	blocklist *activitypub.BlocklistCache
	cfg       *config.Config
	logger    *slog.Logger
	metrics   *observability.Metrics
	http      *http.Client
}

// NewFederationWorker constructs a FederationWorker. Call Start to begin consuming.
func NewFederationWorker(
	js jetstream.JetStream,
	producer *Producer,
	s store.Store,
	bl *activitypub.BlocklistCache,
	cfg *config.Config,
	logger *slog.Logger,
	metrics *observability.Metrics,
) *FederationWorker {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
	return &FederationWorker{
		js:        js,
		producer:  producer,
		store:     s,
		blocklist: bl,
		cfg:       cfg,
		logger:    logger,
		metrics:   metrics,
		http:      client,
	}
}

// Start obtains the durable consumer and launches worker goroutines. Blocks until ctx is cancelled.
func (w *FederationWorker) Start(ctx context.Context) error {
	consumer, err := w.js.Consumer(ctx, streamNameFederation, consumerName)
	if err != nil {
		return fmt.Errorf("federation worker: get consumer: %w", err)
	}

	concurrency := w.cfg.FederationWorkerConcurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	w.logger.Info("federation worker started",
		slog.Int("concurrency", concurrency),
		slog.String("consumer", consumerName),
	)

	for i := 0; i < concurrency; i++ {
		go w.runWorker(ctx, consumer, i)
	}

	<-ctx.Done()
	w.logger.Info("federation worker stopping")
	return nil
}

func (w *FederationWorker) runWorker(ctx context.Context, consumer jetstream.Consumer, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgs, err := consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.Debug("federation worker fetch", slog.Int("worker", workerID), slog.Any("error", err))
			continue
		}

		for msg := range msgs.Messages() {
			w.processMessage(ctx, msg)
		}
		if err := msgs.Error(); err != nil && ctx.Err() == nil {
			w.logger.Debug("federation worker fetch error", slog.Int("worker", workerID), slog.Any("error", err))
		}
	}
}

func (w *FederationWorker) processMessage(ctx context.Context, msg jetstream.Msg) {
	var delivery activitypub.DeliveryMessage
	if err := json.Unmarshal(msg.Data(), &delivery); err != nil {
		w.logger.Warn("federation worker: invalid payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}

	activityType := subjectToActivityType(msg.Subject())

	targetDomain := domainFromURL(delivery.TargetInbox)
	if targetDomain != "" && w.blocklist != nil && w.blocklist.IsSuspended(ctx, targetDomain) {
		w.logger.Debug("federation worker: skip suspended domain",
			slog.String("domain", targetDomain),
			slog.String("activity_id", delivery.ActivityID),
		)
		_ = msg.Ack()
		return
	}

	account, err := w.store.GetAccountByID(ctx, delivery.SenderID)
	if err != nil || account == nil {
		w.logger.Warn("federation worker: sender not found", slog.String("sender_id", delivery.SenderID), slog.Any("error", err))
		w.moveToDLQIfLastAttempt(ctx, msg, activityType, delivery)
		return
	}
	if account.PrivateKey == nil || *account.PrivateKey == "" {
		w.logger.Warn("federation worker: sender has no private key", slog.String("sender_id", delivery.SenderID))
		w.moveToDLQIfLastAttempt(ctx, msg, activityType, delivery)
		return
	}

	privateKey, err := parseRSAPrivateKeyPEM(*account.PrivateKey)
	if err != nil {
		w.logger.Warn("federation worker: invalid private key", slog.String("sender_id", delivery.SenderID), slog.Any("error", err))
		w.moveToDLQIfLastAttempt(ctx, msg, activityType, delivery)
		return
	}

	keyID := actorKeyID(account, w.cfg.InstanceDomain)

	statusCode, err := w.deliverHTTP(ctx, delivery, keyID, privateKey)
	if err != nil {
		w.logger.Warn("federation worker: delivery failed",
			slog.String("activity_id", delivery.ActivityID),
			slog.String("target", delivery.TargetInbox),
			slog.Any("error", err),
		)
		w.handleDeliveryFailure(ctx, msg, delivery, activityType, statusCode)
		return
	}

	if statusCode >= 200 && statusCode < 300 {
		_ = msg.Ack()
		if w.metrics != nil {
			w.metrics.NATSPublishTotal.WithLabelValues("federation.deliver."+activityType, "ok").Inc()
		}
		return
	}

	w.handleDeliveryFailure(ctx, msg, delivery, activityType, statusCode)
}

func (w *FederationWorker) deliverHTTP(ctx context.Context, delivery activitypub.DeliveryMessage, keyID string, privateKey *rsa.PrivateKey) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.TargetInbox, bytes.NewReader(delivery.Activity))
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("User-Agent", "Monstera-fed/1.0")

	if err := activitypub.Sign(req, keyID, privateKey); err != nil {
		return 0, fmt.Errorf("sign: %w", err)
	}

	resp, err := w.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func (w *FederationWorker) handleDeliveryFailure(ctx context.Context, msg jetstream.Msg, delivery activitypub.DeliveryMessage, activityType string, _ int) {
	meta, err := msg.Metadata()
	if err != nil {
		_ = msg.Nak()
		return
	}

	if meta.NumDelivered >= maxDeliver {
		if err := w.producer.EnqueueDLQ(ctx, activityType, delivery); err != nil {
			w.logger.Warn("federation worker: enqueue DLQ failed", slog.String("activity_id", delivery.ActivityID), slog.Any("error", err))
		}
		_ = msg.Ack()
		if w.metrics != nil {
			w.metrics.NATSPublishTotal.WithLabelValues("federation.dlq."+activityType, "ok").Inc()
		}
		return
	}

	_ = msg.Nak()
}

func (w *FederationWorker) moveToDLQIfLastAttempt(ctx context.Context, msg jetstream.Msg, activityType string, delivery activitypub.DeliveryMessage) {
	meta, err := msg.Metadata()
	if err != nil {
		_ = msg.Nak()
		return
	}
	if meta.NumDelivered >= maxDeliver {
		if err := w.producer.EnqueueDLQ(ctx, activityType, delivery); err != nil {
			w.logger.Warn("federation worker: enqueue DLQ failed", slog.String("activity_id", delivery.ActivityID), slog.Any("error", err))
		}
		_ = msg.Ack()
		return
	}
	_ = msg.Nak()
}

func subjectToActivityType(subject string) string {
	const prefix = "federation.deliver."
	if strings.HasPrefix(subject, prefix) {
		return strings.TrimPrefix(subject, prefix)
	}
	return "unknown"
}

func domainFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

func actorKeyID(account *domain.Account, instanceDomain string) string {
	// Match Actor document: id + "#main-key"
	base := account.APID
	if base == "" {
		base = fmt.Sprintf("https://%s/users/%s", instanceDomain, account.Username)
	}
	return base + "#main-key"
}

// parseRSAPrivateKeyPEM decodes a PEM-encoded RSA private key.
func parseRSAPrivateKeyPEM(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS1 private key: %w", err)
	}
	return key, nil
}
