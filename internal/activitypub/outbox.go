package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
)

// Outbox builds ActivityPub activities and sends them for delivery. Fan-out (status create/delete to followers) is asynchronous; single-target deliveries (follow, undo, accept) are enqueued for delivery. Call Start to run the internal workers.
type Outbox struct {
	store                store.Store
	delivery             outboxDeliveryPublisher
	fanout               outboxFanoutPublisher
	outboxDeliveryWorker *outboxDeliveryWorker
	outboxFanoutWorker   *outboxFanoutWorker
	cfg                  *config.Config
}

// NewOutbox constructs an Outbox. Call Start to begin consuming delivery and fan-out messages.
func NewOutbox(
	s store.Store,
	js jetstream.JetStream,
	bl *BlocklistCache,
	signer *HTTPSignatureService,
	cfg *config.Config,
) *Outbox {
	dw := newOutboxDeliveryWorker(js, bl, signer, cfg)
	fw := newOutboxFanoutWorker(js, s, dw, cfg)
	return &Outbox{
		store:                s,
		delivery:             dw,
		fanout:               fw,
		outboxDeliveryWorker: dw,
		outboxFanoutWorker:   fw,
		cfg:                  cfg,
	}
}

// Start runs the delivery and fan-out workers until ctx is cancelled. It blocks until both have stopped.
func (o *Outbox) Start(ctx context.Context) error {
	errc := make(chan error, 2)
	go func() { errc <- o.outboxDeliveryWorker.start(ctx) }()
	go func() { errc <- o.outboxFanoutWorker.start(ctx) }()
	return <-errc
}

// PublishStatus enqueues a Create{Note} activity for async fan-out to the author's followers' inboxes.
func (p *Outbox) PublishStatus(ctx context.Context, status *domain.Status) error {
	account, err := p.store.GetAccountByID(ctx, status.AccountID)
	if err != nil {
		return fmt.Errorf("outbox: get account: %w", err)
	}
	note := p.statusToNote(status, account)
	activityID := status.APID
	if activityID == "" {
		activityID = status.URI
	}
	if activityID == "" {
		activityID = fmt.Sprintf("https://%s/activities/%s", p.cfg.InstanceDomain, uid.New())
	}
	create, err := WrapInCreate(activityID, note)
	if err != nil {
		return fmt.Errorf("outbox: wrap create: %w", err)
	}
	raw, err := json.Marshal(create)
	if err != nil {
		return fmt.Errorf("outbox: marshal create: %w", err)
	}
	msg := outboxFanoutMessage{
		ActivityID: activityID,
		Activity:   raw,
		SenderID:   account.ID,
	}
	if err := p.fanout.publish(ctx, "create", msg); err != nil {
		return fmt.Errorf("outbox: enqueue fanout: %w", err)
	}
	slog.DebugContext(ctx, "outbox: PublishStatus enqueued fanout", slog.String("status_id", status.ID), slog.String("activity_id", activityID))
	return nil
}

// DeleteStatus enqueues a Delete{Tombstone} activity for async fan-out to the author's followers' inboxes.
func (p *Outbox) DeleteStatus(ctx context.Context, status *domain.Status) error {
	account, err := p.store.GetAccountByID(ctx, status.AccountID)
	if err != nil {
		return fmt.Errorf("outbox: get account: %w", err)
	}
	objectID := status.APID
	if objectID == "" {
		objectID = status.URI
	}
	if objectID == "" {
		objectID = fmt.Sprintf("https://%s/statuses/%s", p.cfg.InstanceDomain, status.ID)
	}
	actorID := account.APID
	if actorID == "" {
		actorID = fmt.Sprintf("https://%s/users/%s", p.cfg.InstanceDomain, account.Username)
	}
	deleteAct, err := NewDeleteActivity(objectID+"#delete", actorID, objectID)
	if err != nil {
		return fmt.Errorf("outbox: new delete activity: %w", err)
	}
	raw, err := json.Marshal(deleteAct)
	if err != nil {
		return fmt.Errorf("outbox: marshal delete: %w", err)
	}
	msg := outboxFanoutMessage{
		ActivityID: objectID + "#delete",
		Activity:   raw,
		SenderID:   account.ID,
	}
	if err := p.fanout.publish(ctx, "delete", msg); err != nil {
		return fmt.Errorf("outbox: enqueue fanout: %w", err)
	}
	slog.DebugContext(ctx, "outbox: DeleteStatus enqueued fanout", slog.String("status_id", status.ID))
	return nil
}

// PublishFollow delivers a Follow activity to the target's inbox (single delivery).
func (p *Outbox) PublishFollow(ctx context.Context, actor, target *domain.Account, followID string) error {
	if target.InboxURL == "" {
		return nil
	}
	actorID := actor.APID
	if actorID == "" {
		actorID = fmt.Sprintf("https://%s/users/%s", p.cfg.InstanceDomain, actor.Username)
	}
	targetID := target.APID
	if targetID == "" {
		targetID = fmt.Sprintf("https://%s/users/%s", p.cfg.InstanceDomain, target.Username)
	}
	activityID := fmt.Sprintf("https://%s/activities/%s", p.cfg.InstanceDomain, followID)
	follow, err := NewFollowActivity(activityID, actorID, targetID)
	if err != nil {
		return fmt.Errorf("outbox: new follow activity: %w", err)
	}
	raw, err := json.Marshal(follow)
	if err != nil {
		return fmt.Errorf("outbox: marshal follow: %w", err)
	}
	msg := outboxDeliveryMessage{
		ActivityID:  activityID,
		Activity:    raw,
		TargetInbox: target.InboxURL,
		SenderID:    actor.ID,
	}
	if err := p.delivery.publish(ctx, "follow", msg); err != nil {
		slog.Warn("outbox: enqueue follow failed", slog.String("target", target.InboxURL), slog.Any("error", err))
	}
	return nil
}

// PublishUndoFollow delivers an Undo{Follow} activity to the target's inbox.
func (p *Outbox) PublishUndoFollow(ctx context.Context, actor, target *domain.Account, followID string) error {
	if target.InboxURL == "" {
		return nil
	}
	actorID := actor.APID
	if actorID == "" {
		actorID = fmt.Sprintf("https://%s/users/%s", p.cfg.InstanceDomain, actor.Username)
	}
	targetID := target.APID
	if targetID == "" {
		targetID = fmt.Sprintf("https://%s/users/%s", p.cfg.InstanceDomain, target.Username)
	}
	followActivityID := fmt.Sprintf("https://%s/activities/%s", p.cfg.InstanceDomain, followID)
	inner, err := NewFollowActivity(followActivityID, actorID, targetID)
	if err != nil {
		return fmt.Errorf("outbox: new follow for undo: %w", err)
	}
	undoID := fmt.Sprintf("https://%s/activities/undo-%s", p.cfg.InstanceDomain, followID)
	undo, err := NewUndoActivity(undoID, actorID, inner)
	if err != nil {
		return fmt.Errorf("outbox: new undo activity: %w", err)
	}
	raw, err := json.Marshal(undo)
	if err != nil {
		return fmt.Errorf("outbox: marshal undo follow: %w", err)
	}
	msg := outboxDeliveryMessage{
		ActivityID:  undoID,
		Activity:    raw,
		TargetInbox: target.InboxURL,
		SenderID:    actor.ID,
	}
	if err := p.delivery.publish(ctx, "undo", msg); err != nil {
		slog.Warn("outbox: enqueue undo follow failed", slog.String("target", target.InboxURL), slog.Any("error", err))
	}
	return nil
}

// SendAcceptFollow implements AcceptFollowSender; delivers Accept{Follow} to the follower's inbox.
// target is the local account that accepted; actor is the remote follower.
func (p *Outbox) SendAcceptFollow(ctx context.Context, target, actor *domain.Account, followID string) error {
	if actor.InboxURL == "" {
		return nil
	}
	targetID := target.APID
	if targetID == "" {
		targetID = fmt.Sprintf("https://%s/users/%s", p.cfg.InstanceDomain, target.Username)
	}
	actorID := actor.APID
	if actorID == "" {
		actorID = fmt.Sprintf("https://%s/users/%s", p.cfg.InstanceDomain, actor.Username)
	}
	followActivityID := fmt.Sprintf("https://%s/activities/%s", p.cfg.InstanceDomain, followID)
	inner, err := NewFollowActivity(followActivityID, actorID, targetID)
	if err != nil {
		return fmt.Errorf("outbox: new follow for accept: %w", err)
	}
	acceptID := fmt.Sprintf("https://%s/activities/accept-%s", p.cfg.InstanceDomain, followID)
	accept, err := NewAcceptActivity(acceptID, targetID, inner)
	if err != nil {
		return fmt.Errorf("outbox: new accept activity: %w", err)
	}
	raw, err := json.Marshal(accept)
	if err != nil {
		return fmt.Errorf("outbox: marshal accept: %w", err)
	}
	msg := outboxDeliveryMessage{
		ActivityID:  acceptID,
		Activity:    raw,
		TargetInbox: actor.InboxURL,
		SenderID:    target.ID,
	}
	if err := p.delivery.publish(ctx, "accept", msg); err != nil {
		slog.Warn("outbox: enqueue accept follow failed", slog.String("target", actor.InboxURL), slog.Any("error", err))
	}
	return nil
}

func (p *Outbox) statusToNote(s *domain.Status, account *domain.Account) *Note {
	content := ""
	if s.Content != nil {
		content = *s.Content
	} else if s.Text != nil {
		content = *s.Text
	}
	noteID := s.APID
	if noteID == "" {
		noteID = s.URI
	}
	if noteID == "" {
		noteID = fmt.Sprintf("https://%s/statuses/%s", p.cfg.InstanceDomain, s.ID)
	}
	actorID := account.APID
	if actorID == "" {
		actorID = fmt.Sprintf("https://%s/users/%s", p.cfg.InstanceDomain, account.Username)
	}
	published := s.CreatedAt.Format(time.RFC3339)
	return &Note{
		Context:      DefaultContext,
		ID:           noteID,
		Type:         "Note",
		AttributedTo: actorID,
		Content:      content,
		To:           []string{PublicAddress},
		Published:    published,
		URL:          noteID,
		Sensitive:    s.Sensitive,
		Summary:      s.ContentWarning,
	}
}
