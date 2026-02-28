package federation

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
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera-fed/internal/activitypub"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	natsutil "github.com/chairswithlegs/monstera-fed/internal/nats"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
)

const federationUserAgent = "Monstera-fed/1.0"

// FederationWorker consumes delivery jobs from the FEDERATION JetStream stream
// and POSTs AP activities to remote inboxes with HTTP Signature authentication.
type FederationWorker struct {
	js        jetstream.JetStream
	producer  *FederationProducer
	blocklist *activitypub.BlocklistCache
	cfg       *config.Config
	signer    *activitypub.HTTPSignatureService
	http      *http.Client
}

// NewFederationWorker constructs a FederationWorker. Call Start to begin consuming.
func NewFederationWorker(
	js jetstream.JetStream,
	producer *FederationProducer,
	bl *activitypub.BlocklistCache,
	signer *activitypub.HTTPSignatureService,
	cfg *config.Config,
) *FederationWorker {
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
	return &FederationWorker{
		js:        js,
		producer:  producer,
		blocklist: bl,
		cfg:       cfg,
		signer:    signer,
		http:      client,
	}
}

// Start obtains the durable consumer and runs Consume to process messages concurrently.
//
// Multiple replicas: every replica uses the same consumer name (federation-worker).
// The server has one logical consumer; work is distributed across replicas with no
// duplicate delivery.
func (w *FederationWorker) Start(ctx context.Context) error {
	consumer, err := w.js.Consumer(ctx, natsutil.StreamFederation, natsutil.ConsumerFederationWorker)
	if err != nil {
		return fmt.Errorf("federation worker: get consumer: %w", err)
	}

	concurrency := w.cfg.FederationWorkerConcurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	slog.Info("federation worker started",
		slog.Int("concurrency", concurrency),
		slog.String("consumer", natsutil.ConsumerFederationWorker),
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
		return fmt.Errorf("federation worker: consume: %w", err)
	}

	<-ctx.Done()
	slog.Info("federation worker stopping")
	consCtx.Stop()
	<-consCtx.Closed()
	return nil
}

func (w *FederationWorker) processMessage(ctx context.Context, msg jetstream.Msg) {
	var delivery activitypub.DeliveryMessage
	if err := json.Unmarshal(msg.Data(), &delivery); err != nil {
		slog.Warn("federation worker: invalid payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}

	activityType := subjectToActivityType(msg.Subject())

	targetDomain := domainFromURL(delivery.TargetInbox)
	if targetDomain != "" && w.blocklist != nil && w.blocklist.IsSuspended(ctx, targetDomain) {
		_ = msg.Ack()
		return
	}

	statusCode, err := w.deliverHTTP(ctx, delivery)
	if err != nil {
		slog.Warn("federation worker: delivery failed",
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
		observability.IncNATSPublish(natsutil.SubjectPrefixFederationDeliver+activityType, "ok")
		return
	}

	w.handleDeliveryFailure(ctx, msg, delivery, activityType, statusCode)
}

func (w *FederationWorker) deliverHTTP(ctx context.Context, delivery activitypub.DeliveryMessage) (int, error) {
	deliverCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(deliverCtx, http.MethodPost, delivery.TargetInbox, bytes.NewReader(delivery.Activity))
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("User-Agent", federationUserAgent)

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
		slog.WarnContext(ctx, "federation worker: inbox returned error",
			slog.Int("status", resp.StatusCode),
			slog.String("target", delivery.TargetInbox),
			slog.String("activity_id", delivery.ActivityID),
			slog.String("response_body", bodySnippet))
	}
	return resp.StatusCode, nil
}

func (w *FederationWorker) handleDeliveryFailure(ctx context.Context, msg jetstream.Msg, delivery activitypub.DeliveryMessage, activityType string, statusCode int) {
	meta, err := msg.Metadata()
	if err != nil {
		w.nakWithBackoff(msg, nil)
		return
	}

	if statusCode >= 400 && statusCode < 500 {
		if err := w.producer.EnqueueDLQ(ctx, activityType, delivery); err != nil {
			slog.Warn("federation worker: enqueue DLQ failed", slog.String("activity_id", delivery.ActivityID), slog.Any("error", err))
		}
		_ = msg.Term()
		return
	}

	if meta.NumDelivered >= natsutil.MaxDeliverFederation {
		if err := w.producer.EnqueueDLQ(ctx, activityType, delivery); err != nil {
			slog.Warn("federation worker: enqueue DLQ failed", slog.String("activity_id", delivery.ActivityID), slog.Any("error", err))
		}
		_ = msg.Ack()
		observability.IncNATSPublish(natsutil.SubjectPrefixFederationDLQ+activityType, "ok")
		return
	}

	w.nakWithBackoff(msg, meta)
}

func (w *FederationWorker) termToDLQ(ctx context.Context, msg jetstream.Msg, activityType string, delivery activitypub.DeliveryMessage) {
	if err := w.producer.EnqueueDLQ(ctx, activityType, delivery); err != nil {
		slog.Warn("federation worker: enqueue DLQ failed", slog.String("activity_id", delivery.ActivityID), slog.Any("error", err))
	}
	_ = msg.Term()
}

var nakBackoff = []time.Duration{30 * time.Second, 5 * time.Minute, 30 * time.Minute}

// nakWithBackoff naks the message with a delay from nakBackoff based on meta.NumDelivered.
// If meta is nil, the first backoff duration is used.
func (w *FederationWorker) nakWithBackoff(msg jetstream.Msg, meta *jetstream.MsgMetadata) {
	numDelivered := uint64(0)
	if meta != nil {
		numDelivered = meta.NumDelivered
	}
	_ = msg.NakWithDelay(nakBackoffDelay(numDelivered))
}

// nakBackoffDelay returns the NAK delay for the given delivery count (0 = first attempt).
// Used so backoff logic can be unit-tested without a real jetstream.Msg.
func nakBackoffDelay(numDelivered uint64) time.Duration {
	if numDelivered == 0 {
		return nakBackoff[0]
	}
	idx := len(nakBackoff) - 1
	if numDelivered <= uint64(len(nakBackoff)) {
		idx = int(numDelivered - 1) //nolint:gosec // G115: bounded by len(nakBackoff), small in practice
	}
	return nakBackoff[idx]
}
