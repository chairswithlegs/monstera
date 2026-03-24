package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/activitypub/internal"
	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/blocklist"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// FederationSubscriber consumes domain events from the DOMAIN_EVENTS stream,
// translates them into ActivityPub activities, and sends them to the outbox workers
// for delivery and fanout.
type FederationSubscriber struct {
	js              jetstream.JetStream
	fanout          internal.OutboxFanoutWorker
	delivery        internal.OutboxDeliveryWorker
	instanceBaseURL string
	uiBaseURL       string
}

// NewFederationSubscriber creates a federation subscriber.
// instanceBaseURL is the AP/API server base URL; uiBaseURL is the web UI base URL
// used for the human-readable Actor.URL field in federated profile documents.
func NewFederationSubscriber(
	js jetstream.JetStream,
	followers service.RemoteFollowService,
	bl *blocklist.BlocklistCache,
	signer HTTPSignatureService,
	instanceBaseURL string,
	uiBaseURL string,
	appEnv string,
	insecureSkipTLS bool,
	workerConcurrency int,
) *FederationSubscriber {
	delivery := internal.NewOutboxDeliveryWorker(js, bl, signer, appEnv, insecureSkipTLS, workerConcurrency)
	fanout := internal.NewOutboxFanoutWorker(js, followers, delivery, workerConcurrency)

	return &FederationSubscriber{
		js:              js,
		delivery:        delivery,
		fanout:          fanout,
		instanceBaseURL: instanceBaseURL,
		uiBaseURL:       uiBaseURL,
	}
}

// Start subscribes to the domain-events-federation consumer and processes
// messages via the outbox workers until ctx is cancelled.
func (s *FederationSubscriber) Start(ctx context.Context) error {
	go func() {
		errc := make(chan error, 2)
		go func() { errc <- s.delivery.Start(ctx) }()
		go func() { errc <- s.fanout.Start(ctx) }()
		if err := <-errc; err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "federation subscriber: outbox worker failed", slog.Any("error", err))
		}
	}()

	// Start the federation consumer
	consumer, err := s.js.Consumer(ctx, events.StreamDomainEvents, events.ConsumerFederation)
	if err != nil {
		return fmt.Errorf("federation subscriber: get consumer: %w", err)
	}

	slog.InfoContext(ctx, "federation subscriber started",
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
				slog.WarnContext(ctx, "federation subscriber consume error", slog.Any("error", err))
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("federation subscriber: consume: %w", err)
	}

	<-ctx.Done()
	slog.InfoContext(ctx, "federation subscriber stopping")
	consCtx.Stop()
	<-consCtx.Closed()
	return nil
}

func (s *FederationSubscriber) processMessage(ctx context.Context, msg jetstream.Msg) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "federation subscriber: panic in processMessage", slog.Any("panic", r), slog.String("subject", msg.Subject()))
			_ = msg.Nak()
		}
	}()
	var event domain.DomainEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		slog.WarnContext(ctx, "federation subscriber: invalid event payload", slog.Any("error", err))
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
	case domain.EventReblogCreated:
		err = s.handleReblogCreated(ctx, event)
	case domain.EventReblogRemoved:
		err = s.handleReblogRemoved(ctx, event)
	case domain.EventFavouriteCreated:
		err = s.handleFavouriteCreated(ctx, event)
	case domain.EventFavouriteRemoved:
		err = s.handleFavouriteRemoved(ctx, event)
	case domain.EventStatusCreatedRemote,
		domain.EventStatusDeletedRemote,
		domain.EventStatusUpdatedRemote,
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
	if !payload.Local || payload.Author == nil {
		return nil
	}
	note, err := vocab.LocalStatusToNote(vocab.LocalStatusToNoteInput{
		Status:       payload.Status,
		Author:       payload.Author,
		InstanceBase: s.instanceBaseURL,
		Mentions:     payload.Mentions,
		Tags:         payload.Tags,
		Media:        payload.Media,
		ParentAPID:   payload.ParentAPID,
	})
	if err != nil {
		return fmt.Errorf("local status to note: %w", err)
	}
	activityID := s.statusActivityID(payload.Status)
	create, err := vocab.NewCreateNoteActivity(activityID, note)
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
		objectID = fmt.Sprintf("%s/statuses/%s", s.instanceBaseURL, payload.StatusID)
	}
	actorID := vocab.AccountActorID(payload.Author, s.instanceBaseURL)
	deleteAct, err := vocab.NewDeleteActivity(objectID+"#delete", actorID, objectID)
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
	if !payload.Local || payload.Author == nil {
		return nil
	}
	note, err := vocab.LocalStatusToNote(vocab.LocalStatusToNoteInput{
		Status:       payload.Status,
		Author:       payload.Author,
		InstanceBase: s.instanceBaseURL,
		Mentions:     payload.Mentions,
		Tags:         payload.Tags,
		Media:        payload.Media,
		ParentAPID:   payload.ParentAPID,
	})
	if err != nil {
		return fmt.Errorf("local status to note: %w", err)
	}
	activityID := s.statusActivityID(payload.Status)
	actorID := vocab.AccountActorID(payload.Author, s.instanceBaseURL)
	update, err := vocab.NewUpdateNoteActivity(activityID+"#update", actorID, note)
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
	if !payload.Local || payload.Actor == nil {
		return nil
	}
	base := s.instanceBaseURL
	actorID := vocab.AccountActorID(payload.Actor, base)
	targetID := vocab.AccountActorID(payload.Target, base)
	activityID := fmt.Sprintf("%s/activities/%s", base, payload.Follow.ID)
	follow, err := vocab.NewFollowActivity(activityID, actorID, targetID)
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
	if !payload.Local || payload.Actor == nil {
		return nil
	}
	base := s.instanceBaseURL
	actorID := vocab.AccountActorID(payload.Actor, base)
	targetID := vocab.AccountActorID(payload.Target, base)
	followActivityID := fmt.Sprintf("%s/activities/%s", base, payload.FollowID)
	inner, err := vocab.NewFollowActivity(followActivityID, actorID, targetID)
	if err != nil {
		return fmt.Errorf("new follow for undo: %w", err)
	}
	undoID := fmt.Sprintf("%s/activities/undo-%s", base, payload.FollowID)
	undo, err := vocab.NewUndoActivity(undoID, actorID, inner)
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
	if !payload.Local || payload.Target == nil {
		return nil
	}
	base := s.instanceBaseURL
	targetID := vocab.AccountActorID(payload.Target, base)
	actorID := vocab.AccountActorID(payload.Actor, base)
	followActivityID := fmt.Sprintf("%s/activities/%s", base, payload.Follow.ID)
	inner, err := vocab.NewFollowActivity(followActivityID, actorID, targetID)
	if err != nil {
		return fmt.Errorf("new follow for accept: %w", err)
	}
	acceptID := fmt.Sprintf("%s/activities/accept-%s", base, payload.Follow.ID)
	accept, err := vocab.NewAcceptActivity(acceptID, targetID, inner)
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
	if !payload.Local || payload.Actor == nil {
		return nil
	}
	base := s.instanceBaseURL
	actorID := vocab.AccountActorID(payload.Actor, base)
	targetID := vocab.AccountActorID(payload.Target, base)
	activityID := fmt.Sprintf("%s/activities/%s", base, uid.New())
	block, err := vocab.NewBlockActivity(activityID, actorID, targetID)
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
	if !payload.Local || payload.Actor == nil {
		return nil
	}
	base := s.instanceBaseURL
	actorID := vocab.AccountActorID(payload.Actor, base)
	targetID := vocab.AccountActorID(payload.Target, base)
	blockID := fmt.Sprintf("%s/activities/block-%s-%s", base, payload.Actor.ID, payload.Target.ID)
	inner, err := vocab.NewBlockActivity(blockID, actorID, targetID)
	if err != nil {
		return fmt.Errorf("new block for undo: %w", err)
	}
	undoID := fmt.Sprintf("%s/activities/undo-block-%s-%s", base, payload.Actor.ID, payload.Target.ID)
	undo, err := vocab.NewUndoActivity(undoID, actorID, inner)
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
	if !payload.Local || payload.Account == nil {
		return nil
	}
	actor := vocab.AccountToActor(payload.Account, s.instanceBaseURL, s.uiBaseURL)
	actorID := actor.ID
	activityID := fmt.Sprintf("%s/activities/%s", s.instanceBaseURL, uid.New())
	update, err := vocab.NewUpdateActorActivity(activityID, actorID, actor)
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

func (s *FederationSubscriber) handleReblogCreated(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.ReblogCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal reblog.created payload: %w", err)
	}
	if !payload.Local || payload.FromAccount == nil {
		return nil
	}
	if payload.OriginalStatusAPID == "" {
		return nil
	}
	base := s.instanceBaseURL
	actorID := vocab.AccountActorID(payload.FromAccount, base)
	followersURL := vocab.AccountFollowersURL(payload.FromAccount, base)
	activityID := fmt.Sprintf("%s/activities/%s", base, uid.New())
	announce, err := vocab.NewAnnounceActivity(
		activityID, actorID, payload.OriginalStatusAPID,
		[]string{vocab.PublicAddress}, []string{followersURL},
	)
	if err != nil {
		return fmt.Errorf("new announce activity: %w", err)
	}
	raw, err := json.Marshal(announce)
	if err != nil {
		return fmt.Errorf("marshal announce: %w", err)
	}
	if payload.OriginalAuthor != nil && payload.OriginalAuthor.IsRemote() {
		err = s.delivery.Publish(ctx, "announce", internal.OutboxDeliveryMessage{
			ActivityID:  activityID,
			Activity:    raw,
			TargetInbox: payload.OriginalAuthor.InboxURL,
			SenderID:    payload.FromAccount.ID,
		})
		if err != nil {
			return fmt.Errorf("publish announce to original author: %w", err)
		}
	}
	err = s.fanout.Publish(ctx, "announce", internal.OutboxFanoutMessage{
		ActivityID: activityID,
		Activity:   raw,
		SenderID:   payload.FromAccount.ID,
	})
	if err != nil {
		return fmt.Errorf("publish announce fanout: %w", err)
	}
	return nil
}

func (s *FederationSubscriber) handleFavouriteCreated(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.FavouriteCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal favourite.created payload: %w", err)
	}
	if !payload.Local || payload.FromAccount == nil {
		return nil
	}
	if payload.StatusAuthor == nil || payload.StatusAuthor.IsLocal() {
		return nil
	}
	if payload.StatusAPID == "" {
		return nil
	}
	base := s.instanceBaseURL
	actorID := vocab.AccountActorID(payload.FromAccount, base)
	statusAuthorIRI := vocab.AccountActorID(payload.StatusAuthor, base)
	activityID := fmt.Sprintf("%s/activities/%s", base, uid.New())
	like, err := vocab.NewLikeActivity(activityID, actorID, payload.StatusAPID, []string{statusAuthorIRI})
	if err != nil {
		return fmt.Errorf("new like activity: %w", err)
	}
	raw, err := json.Marshal(like)
	if err != nil {
		return fmt.Errorf("marshal like: %w", err)
	}
	err = s.delivery.Publish(ctx, "like", internal.OutboxDeliveryMessage{
		ActivityID:  activityID,
		Activity:    raw,
		TargetInbox: payload.StatusAuthor.InboxURL,
		SenderID:    payload.FromAccount.ID,
	})
	if err != nil {
		return fmt.Errorf("publish like: %w", err)
	}
	return nil
}

func (s *FederationSubscriber) handleReblogRemoved(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.ReblogRemovedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal reblog.removed payload: %w", err)
	}
	if !payload.Local || payload.FromAccount == nil {
		return nil
	}
	if payload.OriginalStatusAPID == "" {
		return nil
	}
	base := s.instanceBaseURL
	actorID := vocab.AccountActorID(payload.FromAccount, base)
	followersURL := vocab.AccountFollowersURL(payload.FromAccount, base)
	announceID := fmt.Sprintf("%s/activities/%s", base, payload.ReblogStatusID)
	inner, err := vocab.NewAnnounceActivity(
		announceID, actorID, payload.OriginalStatusAPID,
		[]string{vocab.PublicAddress}, []string{followersURL},
	)
	if err != nil {
		return fmt.Errorf("new announce for undo: %w", err)
	}
	undoID := fmt.Sprintf("%s/activities/undo-%s", base, payload.ReblogStatusID)
	undo, err := vocab.NewUndoActivity(undoID, actorID, inner)
	if err != nil {
		return fmt.Errorf("new undo announce activity: %w", err)
	}
	raw, err := json.Marshal(undo)
	if err != nil {
		return fmt.Errorf("marshal undo announce: %w", err)
	}
	if err := s.fanout.Publish(ctx, "undo", internal.OutboxFanoutMessage{
		ActivityID: undoID,
		Activity:   raw,
		SenderID:   payload.FromAccount.ID,
	}); err != nil {
		return fmt.Errorf("publish undo announce: %w", err)
	}
	return nil
}

func (s *FederationSubscriber) handleFavouriteRemoved(ctx context.Context, event domain.DomainEvent) error {
	var payload domain.FavouriteRemovedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal favourite.removed payload: %w", err)
	}
	if !payload.Local || payload.FromAccount == nil {
		return nil
	}
	if payload.StatusAPID == "" {
		return nil
	}
	if payload.StatusAuthor == nil || payload.StatusAuthor.IsLocal() {
		return nil
	}
	base := s.instanceBaseURL
	actorID := vocab.AccountActorID(payload.FromAccount, base)
	statusAuthorIRI := vocab.AccountActorID(payload.StatusAuthor, base)
	likeID := fmt.Sprintf("%s/activities/like-%s-%s", base, payload.AccountID, payload.StatusID)
	inner, err := vocab.NewLikeActivity(likeID, actorID, payload.StatusAPID, []string{statusAuthorIRI})
	if err != nil {
		return fmt.Errorf("new like for undo: %w", err)
	}
	undoID := fmt.Sprintf("%s/activities/undo-like-%s-%s", base, payload.AccountID, payload.StatusID)
	undo, err := vocab.NewUndoActivity(undoID, actorID, inner)
	if err != nil {
		return fmt.Errorf("new undo like activity: %w", err)
	}
	raw, err := json.Marshal(undo)
	if err != nil {
		return fmt.Errorf("marshal undo like: %w", err)
	}
	if err := s.delivery.Publish(ctx, "undo", internal.OutboxDeliveryMessage{
		ActivityID:  undoID,
		Activity:    raw,
		TargetInbox: payload.StatusAuthor.InboxURL,
		SenderID:    payload.FromAccount.ID,
	}); err != nil {
		return fmt.Errorf("publish undo like: %w", err)
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
	return fmt.Sprintf("%s/activities/%s", s.instanceBaseURL, uid.New())
}
