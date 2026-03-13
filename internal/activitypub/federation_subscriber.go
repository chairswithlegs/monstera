package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/activitypub/blocklist"
	"github.com/chairswithlegs/monstera/internal/activitypub/internal"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// FederationSubscriber consumes domain events from the DOMAIN_EVENTS stream,
// translates them into ActivityPub activities, and sends them to the outbox workers
// for delivery and fanout.
type FederationSubscriber struct {
	js       jetstream.JetStream
	fanout   internal.OutboxFanoutWorker
	delivery internal.OutboxDeliveryWorker
	cfg      *config.Config
}

// NewFederationSubscriber creates a federation subscriber.
func NewFederationSubscriber(
	js jetstream.JetStream,
	followers service.FollowService,
	bl *blocklist.BlocklistCache,
	signer HTTPSignatureService,
	cfg *config.Config,
) *FederationSubscriber {
	delivery := internal.NewOutboxDeliveryWorker(js, bl, signer, cfg)
	fanout := internal.NewOutboxFanoutWorker(js, followers, delivery, cfg)

	return &FederationSubscriber{
		js:       js,
		delivery: delivery,
		fanout:   fanout,
		cfg:      cfg,
	}
}

// Start subscribes to the domain-events-federation consumer and processes
// messages via the outbox workers until ctx is cancelled.
func (s *FederationSubscriber) Start(ctx context.Context) error {
	// Start the outbox workers
	go func() {
		errc := make(chan error, 2)
		go func() { errc <- s.delivery.Start(ctx) }()
		go func() { errc <- s.fanout.Start(ctx) }()
		<-errc
	}()

	// Start the federation consumer
	consumer, err := s.js.Consumer(ctx, events.StreamDomainEvents, events.ConsumerFederation)
	if err != nil {
		return fmt.Errorf("federation subscriber: get consumer: %w", err)
	}

	slog.Info("federation subscriber started",
		slog.String("consumer", events.ConsumerFederation),
	)

	consCtx, err := consumer.Consume(
		func(msg jetstream.Msg) {
			go s.processMessage(ctx, msg)
		},
		jetstream.PullMaxMessages(10),
		jetstream.PullExpiry(5*time.Second),
		jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
			if ctx.Err() == nil {
				slog.Warn("federation subscriber consume error", slog.Any("error", err))
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("federation subscriber: consume: %w", err)
	}

	<-ctx.Done()
	slog.Info("federation subscriber stopping")
	consCtx.Stop()
	<-consCtx.Closed()
	return nil
}

func (s *FederationSubscriber) processMessage(ctx context.Context, msg jetstream.Msg) {
	var event domain.DomainEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		slog.Warn("federation subscriber: invalid event payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}

	var err error
	switch event.EventType {
	case domain.EventStatusCreated:
		err = s.handleStatusCreated(ctx, event)
	case domain.EventStatusDeleted:
		err = s.handleStatusDeleted(ctx, event)
	case domain.EventStatusUpdated:
		err = s.handleStatusUpdated(ctx, event)
	case domain.EventFollowCreated:
		err = s.handleFollowCreated(ctx, event)
	case domain.EventFollowRemoved:
		err = s.handleFollowRemoved(ctx, event)
	case domain.EventFollowAccepted:
		err = s.handleFollowAccepted(ctx, event)
	case domain.EventBlockCreated:
		err = s.handleBlockCreated(ctx, event)
	case domain.EventBlockRemoved:
		err = s.handleBlockRemoved(ctx, event)
	case domain.EventAccountUpdated:
		err = s.handleAccountUpdated(ctx, event)
	case domain.EventStatusCreatedRemote,
		domain.EventStatusDeletedRemote,
		domain.EventNotificationCreated:
		// SSE-only events — ACK and skip.
	default:
		slog.WarnContext(ctx, "federation subscriber: unknown event type", slog.String("event_type", event.EventType))
	}

	if err != nil {
		slog.ErrorContext(ctx, "federation subscriber: handle event failed",
			slog.String("event_type", event.EventType),
			slog.String("event_id", event.ID),
			slog.Any("error", err),
		)
		// NAK so the message is redelivered.
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

func (s *FederationSubscriber) handleStatusCreated(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.StatusCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal status.created payload: %w", err)
	}
	note := StatusToNote(payload.Status, payload.Author, s.cfg.InstanceDomain)
	activityID := s.statusActivityID(payload.Status)
	create, err := WrapInCreate(activityID, note)
	if err != nil {
		return fmt.Errorf("wrap create: %w", err)
	}
	raw, err := json.Marshal(create)
	if err != nil {
		return fmt.Errorf("marshal create: %w", err)
	}
	err = s.fanout.Publish(ctx, "create", internal.OutboxFanoutMessage{
		ActivityID: activityID,
		Activity:   raw,
		SenderID:   payload.Author.ID,
	})
	if err != nil {
		return fmt.Errorf("publish status created: %w", err)
	}
	return nil
}

func (s *FederationSubscriber) handleStatusDeleted(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.StatusDeletedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal status.deleted payload: %w", err)
	}
	if !payload.Local {
		return nil
	}
	objectID := payload.APID
	if objectID == "" {
		objectID = payload.URI
	}
	if objectID == "" {
		objectID = fmt.Sprintf("https://%s/statuses/%s", s.cfg.InstanceDomain, payload.StatusID)
	}
	actorID := s.accountActorID(payload.Author)
	deleteAct, err := NewDeleteActivity(objectID+"#delete", actorID, objectID)
	if err != nil {
		return fmt.Errorf("new delete activity: %w", err)
	}
	raw, err := json.Marshal(deleteAct)
	if err != nil {
		return fmt.Errorf("marshal delete: %w", err)
	}
	err = s.fanout.Publish(ctx, "delete", internal.OutboxFanoutMessage{
		ActivityID: objectID + "#delete",
		Activity:   raw,
		SenderID:   payload.AccountID,
	})
	if err != nil {
		return fmt.Errorf("publish status deleted: %w", err)
	}
	return nil
}

func (s *FederationSubscriber) handleStatusUpdated(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.StatusUpdatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal status.updated payload: %w", err)
	}
	note := StatusToNote(payload.Status, payload.Author, s.cfg.InstanceDomain)
	activityID := s.statusActivityID(payload.Status)
	actorID := s.accountActorID(payload.Author)
	update, err := WrapInUpdate(activityID+"#update", actorID, note)
	if err != nil {
		return fmt.Errorf("wrap update: %w", err)
	}
	raw, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("marshal update: %w", err)
	}
	err = s.fanout.Publish(ctx, "update", internal.OutboxFanoutMessage{
		ActivityID: activityID + "#update",
		Activity:   raw,
		SenderID:   payload.Author.ID,
	})
	if err != nil {
		return fmt.Errorf("publish status updated: %w", err)
	}
	return nil
}

func (s *FederationSubscriber) handleFollowCreated(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.FollowCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal follow.created payload: %w", err)
	}
	if payload.Target.InboxURL == "" {
		return nil
	}
	actorID := s.accountActorID(payload.Actor)
	targetID := s.accountActorID(payload.Target)
	activityID := fmt.Sprintf("https://%s/activities/%s", s.cfg.InstanceDomain, payload.Follow.ID)
	follow, err := NewFollowActivity(activityID, actorID, targetID)
	if err != nil {
		return fmt.Errorf("new follow activity: %w", err)
	}
	raw, err := json.Marshal(follow)
	if err != nil {
		return fmt.Errorf("marshal follow: %w", err)
	}
	err = s.delivery.Publish(ctx, "follow", internal.OutboxDeliveryMessage{
		ActivityID:  activityID,
		Activity:    raw,
		TargetInbox: payload.Target.InboxURL,
		SenderID:    payload.Actor.ID,
	})
	if err != nil {
		return fmt.Errorf("publish follow: %w", err)
	}
	return nil
}

func (s *FederationSubscriber) handleFollowRemoved(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.FollowRemovedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal follow.removed payload: %w", err)
	}
	if payload.Target.InboxURL == "" {
		return nil
	}
	actorID := s.accountActorID(payload.Actor)
	targetID := s.accountActorID(payload.Target)
	followActivityID := fmt.Sprintf("https://%s/activities/%s", s.cfg.InstanceDomain, payload.FollowID)
	inner, err := NewFollowActivity(followActivityID, actorID, targetID)
	if err != nil {
		return fmt.Errorf("new follow for undo: %w", err)
	}
	undoID := fmt.Sprintf("https://%s/activities/undo-%s", s.cfg.InstanceDomain, payload.FollowID)
	undo, err := NewUndoActivity(undoID, actorID, inner)
	if err != nil {
		return fmt.Errorf("new undo activity: %w", err)
	}
	raw, err := json.Marshal(undo)
	if err != nil {
		return fmt.Errorf("marshal undo follow: %w", err)
	}
	err = s.delivery.Publish(ctx, "undo", internal.OutboxDeliveryMessage{
		ActivityID:  undoID,
		Activity:    raw,
		TargetInbox: payload.Target.InboxURL,
		SenderID:    payload.Actor.ID,
	})
	if err != nil {
		return fmt.Errorf("publish undo follow: %w", err)
	}
	return nil
}

func (s *FederationSubscriber) handleFollowAccepted(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.FollowAcceptedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal follow.accepted payload: %w", err)
	}
	if payload.Actor.InboxURL == "" {
		return nil
	}
	targetID := s.accountActorID(payload.Target)
	actorID := s.accountActorID(payload.Actor)
	followActivityID := fmt.Sprintf("https://%s/activities/%s", s.cfg.InstanceDomain, payload.Follow.ID)
	inner, err := NewFollowActivity(followActivityID, actorID, targetID)
	if err != nil {
		return fmt.Errorf("new follow for accept: %w", err)
	}
	acceptID := fmt.Sprintf("https://%s/activities/accept-%s", s.cfg.InstanceDomain, payload.Follow.ID)
	accept, err := NewAcceptActivity(acceptID, targetID, inner)
	if err != nil {
		return fmt.Errorf("new accept activity: %w", err)
	}
	raw, err := json.Marshal(accept)
	if err != nil {
		return fmt.Errorf("marshal accept: %w", err)
	}
	err = s.delivery.Publish(ctx, "accept", internal.OutboxDeliveryMessage{
		ActivityID:  acceptID,
		Activity:    raw,
		TargetInbox: payload.Actor.InboxURL,
		SenderID:    payload.Target.ID,
	})
	if err != nil {
		return fmt.Errorf("publish follow accepted: %w", err)
	}
	return nil
}

func (s *FederationSubscriber) handleBlockCreated(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.BlockCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal block.created payload: %w", err)
	}
	if payload.Target.InboxURL == "" {
		return nil
	}
	actorID := s.accountActorID(payload.Actor)
	targetID := s.accountActorID(payload.Target)
	activityID := fmt.Sprintf("https://%s/activities/%s", s.cfg.InstanceDomain, uid.New())
	block, err := NewBlockActivity(activityID, actorID, targetID)
	if err != nil {
		return fmt.Errorf("new block activity: %w", err)
	}
	raw, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("marshal block: %w", err)
	}
	err = s.delivery.Publish(ctx, "block", internal.OutboxDeliveryMessage{
		ActivityID:  activityID,
		Activity:    raw,
		TargetInbox: payload.Target.InboxURL,
		SenderID:    payload.Actor.ID,
	})
	if err != nil {
		return fmt.Errorf("publish block created: %w", err)
	}
	return nil
}

func (s *FederationSubscriber) handleBlockRemoved(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.BlockRemovedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal block.removed payload: %w", err)
	}
	if payload.Target.InboxURL == "" {
		return nil
	}
	actorID := s.accountActorID(payload.Actor)
	targetID := s.accountActorID(payload.Target)
	blockID := fmt.Sprintf("https://%s/activities/block-%s-%s", s.cfg.InstanceDomain, payload.Actor.ID, payload.Target.ID)
	inner, err := NewBlockActivity(blockID, actorID, targetID)
	if err != nil {
		return fmt.Errorf("new block for undo: %w", err)
	}
	undoID := fmt.Sprintf("https://%s/activities/undo-block-%s-%s", s.cfg.InstanceDomain, payload.Actor.ID, payload.Target.ID)
	undo, err := NewUndoActivity(undoID, actorID, inner)
	if err != nil {
		return fmt.Errorf("new undo activity: %w", err)
	}
	raw, err := json.Marshal(undo)
	if err != nil {
		return fmt.Errorf("marshal undo block: %w", err)
	}
	err = s.delivery.Publish(ctx, "undo", internal.OutboxDeliveryMessage{
		ActivityID:  undoID,
		Activity:    raw,
		TargetInbox: payload.Target.InboxURL,
		SenderID:    payload.Actor.ID,
	})
	if err != nil {
		return fmt.Errorf("publish undo block: %w", err)
	}
	return nil
}

func (s *FederationSubscriber) handleAccountUpdated(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.AccountUpdatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal account.updated payload: %w", err)
	}
	actor := AccountToActor(payload.Account, s.cfg.InstanceDomain)
	actorID := actor.ID
	activityID := fmt.Sprintf("https://%s/activities/%s", s.cfg.InstanceDomain, uid.New())
	update, err := WrapInUpdateActor(activityID, actorID, actor)
	if err != nil {
		return fmt.Errorf("wrap update actor: %w", err)
	}
	raw, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("marshal update actor: %w", err)
	}
	err = s.fanout.Publish(ctx, "update", internal.OutboxFanoutMessage{
		ActivityID: activityID,
		Activity:   raw,
		SenderID:   payload.Account.ID,
	})
	if err != nil {
		return fmt.Errorf("publish account updated: %w", err)
	}
	return nil
}

// statusActivityID derives an activity ID from a status.
func (s *FederationSubscriber) statusActivityID(status *domain.Status) string {
	if status.APID != "" {
		return status.APID
	}
	if status.URI != "" {
		return status.URI
	}
	return fmt.Sprintf("https://%s/activities/%s", s.cfg.InstanceDomain, uid.New())
}

// accountActorID derives an AP actor ID from an account.
func (s *FederationSubscriber) accountActorID(account *domain.Account) string {
	if account.APID != "" {
		return account.APID
	}
	return fmt.Sprintf("https://%s/users/%s", s.cfg.InstanceDomain, account.Username)
}
