package federation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera-fed/internal/activitypub"
	natsutil "github.com/chairswithlegs/monstera-fed/internal/nats"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
)

// FederationProducer publishes federation delivery messages to NATS JetStream.
type FederationProducer struct {
	js jetstream.JetStream
}

// NewFederationProducer constructs a federation delivery producer.
func NewFederationProducer(js jetstream.JetStream) *FederationProducer {
	return &FederationProducer{js: js}
}

// EnqueueDelivery publishes a delivery message to the FEDERATION stream.
// activityType is used as the subject suffix (e.g. "create" -> "federation.deliver.create").
func (p *FederationProducer) EnqueueDelivery(ctx context.Context, activityType string, msg activitypub.DeliveryMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("federation: marshal delivery message: %w", err)
	}
	subject := natsutil.SubjectPrefixFederationDeliver + strings.ToLower(activityType)
	_, err = p.js.Publish(ctx, subject, data)
	if err != nil {
		observability.IncNATSPublish(subject, "error")
		return fmt.Errorf("federation: publish to %s: %w", subject, err)
	}
	observability.IncNATSPublish(subject, "ok")
	return nil
}

// EnqueueDLQ moves a failed delivery message to the dead-letter queue.
func (p *FederationProducer) EnqueueDLQ(ctx context.Context, activityType string, msg activitypub.DeliveryMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("federation: marshal DLQ message: %w", err)
	}
	subject := natsutil.SubjectPrefixFederationDLQ + strings.ToLower(activityType)
	_, err = p.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("federation: publish DLQ to %s: %w", subject, err)
	}
	return nil
}
