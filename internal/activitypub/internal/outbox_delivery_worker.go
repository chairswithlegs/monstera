package internal

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

	"github.com/chairswithlegs/monstera/internal/blocklist"
	"github.com/chairswithlegs/monstera/internal/natsutil"
	"github.com/chairswithlegs/monstera/internal/ssrf"
)

const outboxUserAgent = "Monstera/1.0"

var outboxDeliveryTimeout = 30 * time.Second

// OutboxDeliveryMessage is the payload for outbound ActivityPub delivery (e.g. to NATS ACTIVITYPUB stream).
//
// Exactly one of SenderID and DeletionID identifies the signer:
//   - SenderID — the normal path. The signer loads the local account by ID
//     and uses its private key.
//   - DeletionID — a Delete{Actor} for a locally hard-deleted account. The
//     accounts row is gone; the signer loads the PEM from the
//     account_deletion_snapshots side table instead.
type OutboxDeliveryMessage struct {
	ActivityID  string          `json:"activity_id"`
	Activity    json.RawMessage `json:"activity"`
	TargetInbox string          `json:"target_inbox"`
	SenderID    string          `json:"sender_id,omitempty"`
	DeletionID  string          `json:"deletion_id,omitempty"`
}

// OutboxDeliveryWorker consumes from the ACTIVITYPUB_DELIVERY stream and delivers activities to remote inboxes.
type OutboxDeliveryWorker interface {
	Publish(ctx context.Context, activityType string, msg OutboxDeliveryMessage) error
	Start(ctx context.Context) error
}

type OutboxHTTPSigner interface {
	SignWithSenderID(ctx context.Context, r *http.Request, senderID string) error
	// SignWithDeletionID signs r using the PEM private key snapshotted in
	// account_deletion_snapshots at account-delete time. Used for
	// Delete{Actor} deliveries after the accounts row (and its key) are gone.
	SignWithDeletionID(ctx context.Context, r *http.Request, deletionID string) error
}

type outboxDeliveryWorker struct {
	js                jetstream.JetStream
	blocklist         *blocklist.BlocklistCache
	workerConcurrency int
	signer            OutboxHTTPSigner
	http              *http.Client
}

// NewOutboxDeliveryWorker constructs an outbox delivery worker. Call Start to begin consuming.
func NewOutboxDeliveryWorker(
	js jetstream.JetStream,
	bl *blocklist.BlocklistCache,
	signer OutboxHTTPSigner,
	appEnv string,
	insecureSkipTLS bool,
	workerConcurrency int,
) OutboxDeliveryWorker {
	var client *http.Client
	if appEnv != "production" && insecureSkipTLS {
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // G402: intentional for development federation with self-signed certs
		}
		client = &http.Client{
			Timeout:   outboxDeliveryTimeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return errors.New("too many redirects")
				}
				return nil
			},
		}
	} else {
		client = ssrf.NewHTTPClient(ssrf.HTTPClientOptions{
			Timeout: outboxDeliveryTimeout,
		})
	}
	return &outboxDeliveryWorker{
		js:                js,
		blocklist:         bl,
		workerConcurrency: workerConcurrency,
		signer:            signer,
		http:              client,
	}
}

// Start obtains the durable consumer and runs Consume to process messages concurrently.
//
// Multiple replicas: every replica uses the same consumer name (activitypub-worker).
// The server has one logical consumer; work is distributed across replicas with no
// duplicate delivery.
func (w *outboxDeliveryWorker) Start(ctx context.Context) error {
	concurrency := w.workerConcurrency
	if concurrency <= 0 {
		concurrency = 5
	}
	if err := natsutil.RunConsumer(ctx, w.js, StreamOutboxDelivery, consumerDelivery,
		func(msg jetstream.Msg) { go w.processMessage(ctx, msg) },
		natsutil.WithMaxMessages(concurrency),
		natsutil.WithLabel("activitypub delivery worker"),
	); err != nil {
		return fmt.Errorf("delivery worker: %w", err)
	}
	return nil
}

// Publish sends a delivery message to the stream for processing.
// activityType is used as the subject suffix (e.g. "create" -> "federation.deliver.create").
func (w *outboxDeliveryWorker) Publish(ctx context.Context, activityType string, msg OutboxDeliveryMessage) error {
	slog.DebugContext(ctx, "outbox delivery worker: publishing message", slog.String("activity_type", activityType), slog.String("activity_id", msg.ActivityID))

	subject := subjectPrefixDeliver + strings.ToLower(activityType)
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("activitypub: marshal delivery message: %w", err)
	}
	if err := natsutil.Publish(ctx, w.js, subject, data); err != nil {
		return fmt.Errorf("delivery publish: %w", err)
	}
	return nil
}

func (w *outboxDeliveryWorker) processMessage(ctx context.Context, msg jetstream.Msg) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "outbox delivery worker: panic in processMessage", slog.Any("panic", r), slog.String("subject", msg.Subject()))
			_ = msg.Nak()
		}
	}()
	slog.DebugContext(ctx, "outbox delivery worker: processing message", slog.String("subject", msg.Subject()))

	var delivery OutboxDeliveryMessage
	if err := json.Unmarshal(msg.Data(), &delivery); err != nil {
		slog.WarnContext(ctx, "activitypub worker: invalid payload", slog.Any("error", err))
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
		slog.WarnContext(ctx, "activitypub worker: delivery failed",
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

func (w *outboxDeliveryWorker) deliverHTTP(ctx context.Context, delivery OutboxDeliveryMessage) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.TargetInbox, bytes.NewReader(delivery.Activity))
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("User-Agent", outboxUserAgent)

	if delivery.DeletionID != "" {
		if err := w.signer.SignWithDeletionID(ctx, req, delivery.DeletionID); err != nil {
			return 0, fmt.Errorf("sign with deletion %s: %w", delivery.DeletionID, err)
		}
	} else {
		if err := w.signer.SignWithSenderID(ctx, req, delivery.SenderID); err != nil {
			return 0, fmt.Errorf("sign: %w", err)
		}
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

func (w *outboxDeliveryWorker) handleDeliveryFailure(ctx context.Context, msg jetstream.Msg, delivery OutboxDeliveryMessage, activityType string, statusCode int) {
	meta, err := msg.Metadata()
	if err != nil {
		natsutil.NAKWithBackoff(msg, nil, deliveryRetries)
		return
	}

	if statusCode >= 400 && statusCode < 500 {
		if err := w.sendToDLQ(ctx, activityType, delivery); err != nil {
			slog.WarnContext(ctx, "activitypub worker: publish DLQ failed", slog.String("activity_id", delivery.ActivityID), slog.Any("error", err))
		}
		_ = msg.Term()
		return
	}

	if meta.NumDelivered >= uint64(len(deliveryRetries)) {
		if err := w.sendToDLQ(ctx, activityType, delivery); err != nil {
			slog.WarnContext(ctx, "activitypub worker: publish DLQ failed", slog.String("activity_id", delivery.ActivityID), slog.Any("error", err))
		}
		_ = msg.Ack()
		return
	}

	natsutil.NAKWithBackoff(msg, meta, deliveryRetries)
}

func (w *outboxDeliveryWorker) termToDLQ(ctx context.Context, msg jetstream.Msg, activityType string, delivery OutboxDeliveryMessage) {
	if err := w.sendToDLQ(ctx, activityType, delivery); err != nil {
		slog.WarnContext(ctx, "activitypub worker: publish DLQ failed", slog.String("activity_id", delivery.ActivityID), slog.Any("error", err))
	}
	_ = msg.Term()
}

// sendToDLQ moves a failed delivery message to the dead-letter queue.
func (w *outboxDeliveryWorker) sendToDLQ(ctx context.Context, activityType string, msg OutboxDeliveryMessage) error {
	subject := subjectPrefixDeliverDLQ + strings.ToLower(activityType)
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("activitypub: marshal DLQ message: %w", err)
	}
	if err := natsutil.Publish(ctx, w.js, subject, data); err != nil {
		return fmt.Errorf("DLQ publish: %w", err)
	}
	return nil
}

func (w *outboxDeliveryWorker) getActivityType(subject string) string {
	if strings.HasPrefix(subject, subjectPrefixDeliver) {
		return strings.TrimPrefix(subject, subjectPrefixDeliver)
	}
	return activityTypeUnknown
}
