package federation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera-fed/internal/activitypub"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
)

const (
	subjectPrefixDeliver = "federation.deliver."
	subjectPrefixDLQ     = "federation.dlq."
)

// Producer publishes federation delivery messages to NATS JetStream.
type Producer struct {
	js      jetstream.JetStream
	metrics *observability.Metrics
}

// NewProducer constructs a federation delivery Producer.
func NewProducer(js jetstream.JetStream, metrics *observability.Metrics) *Producer {
	return &Producer{js: js, metrics: metrics}
}

// EnqueueDelivery publishes a delivery message to the FEDERATION stream.
// activityType is used as the subject suffix (e.g. "create" -> "federation.deliver.create").
func (p *Producer) EnqueueDelivery(ctx context.Context, activityType string, msg activitypub.DeliveryMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("federation: marshal delivery message: %w", err)
	}
	subject := subjectPrefixDeliver + strings.ToLower(activityType)
	_, err = p.js.Publish(ctx, subject, data)
	if err != nil {
		if p.metrics != nil {
			p.metrics.NATSPublishTotal.WithLabelValues(subject, "error").Inc()
		}
		return fmt.Errorf("federation: publish to %s: %w", subject, err)
	}
	if p.metrics != nil {
		p.metrics.NATSPublishTotal.WithLabelValues(subject, "ok").Inc()
	}
	return nil
}

// EnqueueDLQ moves a failed delivery message to the dead-letter queue.
func (p *Producer) EnqueueDLQ(ctx context.Context, activityType string, msg activitypub.DeliveryMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("federation: marshal DLQ message: %w", err)
	}
	subject := subjectPrefixDLQ + strings.ToLower(activityType)
	_, err = p.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("federation: publish DLQ to %s: %w", subject, err)
	}
	return nil
}
