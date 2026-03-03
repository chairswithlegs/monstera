package activitypub

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	natsutil "github.com/chairswithlegs/monstera-fed/internal/nats"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
)

const outboxUserAgent = "Monstera/1.0"

// outboxDeliveryMessage is the payload for outbound ActivityPub delivery (e.g. to NATS ACTIVITYPUB stream).
type outboxDeliveryMessage struct {
	ActivityID  string          `json:"activity_id"`
	Activity    json.RawMessage `json:"activity"`
	TargetInbox string          `json:"target_inbox"`
	SenderID    string          `json:"sender_id"`
}

// outboxDeliveryPublisher enqueues delivery messages for processing (e.g. to NATS).
type outboxDeliveryPublisher interface {
	publish(ctx context.Context, activityType string, msg outboxDeliveryMessage) error
}

// newOutboxDeliveryWorker constructs an outbox delivery worker. Call start to begin consuming.
func newOutboxDeliveryWorker(
	js jetstream.JetStream,
	bl *BlocklistCache,
	signer *HTTPSignatureService,
	cfg *config.Config,
) *outboxDeliveryWorker {
	transport := &http.Transport{}
	if cfg.FederationInsecureSkipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // G402: intentional for development federation with self-signed certs
	}
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
	return &outboxDeliveryWorker{
		js:        js,
		blocklist: bl,
		cfg:       cfg,
		signer:    signer,
		http:      client,
	}
}

type outboxDeliveryWorker struct {
	js        jetstream.JetStream
	blocklist *BlocklistCache
	cfg       *config.Config
	signer    *HTTPSignatureService
	http      *http.Client
}

// start obtains the durable consumer and runs Consume to process messages concurrently.
//
// Multiple replicas: every replica uses the same consumer name (activitypub-worker).
// The server has one logical consumer; work is distributed across replicas with no
// duplicate delivery.
func (w *outboxDeliveryWorker) start(ctx context.Context) error {
	consumer, err := w.js.Consumer(ctx, streamDelivery, consumerDelivery)
	if err != nil {
		return fmt.Errorf("activitypub worker: get consumer: %w", err)
	}

	concurrency := w.cfg.FederationWorkerConcurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	slog.Info("activitypub worker started",
		slog.Int("concurrency", concurrency),
		slog.String("consumer", consumerDelivery),
	)

	consCtx, err := consumer.Consume(
		func(msg jetstream.Msg) {
			go w.processMessage(ctx, msg)
		},
		jetstream.PullMaxMessages(concurrency),
		jetstream.PullExpiry(5*time.Second),
		jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
			if ctx.Err() == nil {
				slog.Warn("federation worker consume error", slog.Any("error", err))
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("activitypub worker: consume: %w", err)
	}

	<-ctx.Done()
	slog.Info("activitypub worker stopping")
	consCtx.Stop()
	<-consCtx.Closed()
	return nil
}

// publish sends a delivery message to the stream for processing.
// activityType is used as the subject suffix (e.g. "create" -> "federation.deliver.create").
func (w *outboxDeliveryWorker) publish(ctx context.Context, activityType string, msg outboxDeliveryMessage) (err error) {
	subject := subjectPrefixDeliver + strings.ToLower(activityType)

	defer func() {
		if err != nil {
			observability.IncNATSPublish(subject, "error")
		} else {
			observability.IncNATSPublish(subject, "ok")
		}
	}()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("activitypub: marshal delivery message: %w", err)
	}
	_, err = w.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("activitypub: publish to %s: %w", subject, err)
	}
	return nil
}

func (w *outboxDeliveryWorker) processMessage(ctx context.Context, msg jetstream.Msg) {
	var delivery outboxDeliveryMessage
	if err := json.Unmarshal(msg.Data(), &delivery); err != nil {
		slog.Warn("activitypub worker: invalid payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}

	activityType := w.getActivityType(msg.Subject())

	targetDomain := domainFromURL(delivery.TargetInbox)
	if targetDomain != "" && w.blocklist != nil && w.blocklist.IsSuspended(ctx, targetDomain) {
		_ = msg.Ack()
		return
	}

	statusCode, err := w.deliverHTTP(ctx, delivery)
	if err != nil {
		slog.Warn("activitypub worker: delivery failed",
			slog.String("activity_id", delivery.ActivityID),
			slog.String("target", delivery.TargetInbox),
			slog.String("sender_id", delivery.SenderID),
			slog.Any("error", err),
		)
		w.termToDLQ(ctx, msg, activityType, delivery)
		return
	}

	if statusCode >= 200 && statusCode < 300 {
		_ = msg.Ack()
		return
	}

	w.handleDeliveryFailure(ctx, msg, delivery, activityType, statusCode)
}

func (w *outboxDeliveryWorker) deliverHTTP(ctx context.Context, delivery outboxDeliveryMessage) (int, error) {
	deliverCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(deliverCtx, http.MethodPost, delivery.TargetInbox, bytes.NewReader(delivery.Activity))
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("User-Agent", outboxUserAgent)

	if err := w.signer.SignWithSenderID(ctx, req, delivery.SenderID); err != nil {
		return 0, fmt.Errorf("sign: %w", err)
	}

	resp, err := w.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// If the response is an error, log it
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		bodySnippet := string(body)
		if len(bodySnippet) > 512 {
			bodySnippet = bodySnippet[:512] + "..."
		}
		slog.WarnContext(ctx, "activitypub worker: inbox returned error",
			slog.Int("status", resp.StatusCode),
			slog.String("target", delivery.TargetInbox),
			slog.String("activity_id", delivery.ActivityID),
			slog.String("response_body", bodySnippet))
	}
	return resp.StatusCode, nil
}

func (w *outboxDeliveryWorker) handleDeliveryFailure(ctx context.Context, msg jetstream.Msg, delivery outboxDeliveryMessage, activityType string, statusCode int) {
	meta, err := msg.Metadata()
	if err != nil {
		natsutil.NAKWithBackoff(msg, nil, deliveryRetries)
		return
	}

	if statusCode >= 400 && statusCode < 500 {
		if err := w.sendToDLQ(ctx, activityType, delivery); err != nil {
			slog.Warn("activitypub worker: publish DLQ failed", slog.String("activity_id", delivery.ActivityID), slog.Any("error", err))
		}
		_ = msg.Term()
		return
	}

	if meta.NumDelivered >= uint64(len(deliveryRetries)) {
		if err := w.sendToDLQ(ctx, activityType, delivery); err != nil {
			slog.Warn("activitypub worker: publish DLQ failed", slog.String("activity_id", delivery.ActivityID), slog.Any("error", err))
		}
		_ = msg.Ack()
		return
	}

	natsutil.NAKWithBackoff(msg, meta, deliveryRetries)
}

func (w *outboxDeliveryWorker) termToDLQ(ctx context.Context, msg jetstream.Msg, activityType string, delivery outboxDeliveryMessage) {
	if err := w.sendToDLQ(ctx, activityType, delivery); err != nil {
		slog.Warn("activitypub worker: publish DLQ failed", slog.String("activity_id", delivery.ActivityID), slog.Any("error", err))
	}
	_ = msg.Term()
}

// sendToDLQ moves a failed delivery message to the dead-letter queue.
func (w *outboxDeliveryWorker) sendToDLQ(ctx context.Context, activityType string, msg outboxDeliveryMessage) (err error) {
	subject := subjectPrefixDeliverDLQ + strings.ToLower(activityType)
	defer func() {
		if err != nil {
			observability.IncNATSPublish(subject, "error")
		} else {
			observability.IncNATSPublish(subject, "ok")
		}
	}()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("activitypub: marshal DLQ message: %w", err)
	}
	_, err = w.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("activitypub: publish DLQ to %s: %w", subject, err)
	}
	return nil
}

func (w *outboxDeliveryWorker) getActivityType(subject string) string {
	if strings.HasPrefix(subject, subjectPrefixDeliver) {
		return strings.TrimPrefix(subject, subjectPrefixDeliver)
	}
	return "unknown"
}
