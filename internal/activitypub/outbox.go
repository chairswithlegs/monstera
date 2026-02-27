package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
	"github.com/chairswithlegs/monstera-fed/internal/uid"
)

// DeliveryMessage is the payload for outbound ActivityPub delivery (e.g. to NATS FEDERATION stream).
type DeliveryMessage struct {
	ActivityID  string          `json:"activity_id"`
	Activity    json.RawMessage `json:"activity"`
	TargetInbox string          `json:"target_inbox"`
	SenderID    string          `json:"sender_id"`
}

// DeliveryEnqueuer enqueues outbound delivery messages (implemented by federation.Producer).
type DeliveryEnqueuer interface {
	EnqueueDelivery(ctx context.Context, activityType string, msg DeliveryMessage) error
}

// OutboxPublisher builds ActivityPub activities and enqueues them for delivery to remote inboxes.
type OutboxPublisher struct {
	store   store.Store
	enqueue DeliveryEnqueuer
	cfg     *config.Config
	logger  *slog.Logger
}

// NewOutboxPublisher constructs an OutboxPublisher.
func NewOutboxPublisher(s store.Store, enqueue DeliveryEnqueuer, cfg *config.Config, logger *slog.Logger) *OutboxPublisher {
	return &OutboxPublisher{store: s, enqueue: enqueue, cfg: cfg, logger: logger}
}

// PublishStatus delivers a Create{Note} activity to the author's followers' inboxes.
func (p *OutboxPublisher) PublishStatus(ctx context.Context, status *domain.Status) error {
	if p.enqueue == nil {
		return nil
	}
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
	inboxURLs, err := p.store.GetFollowerInboxURLs(ctx, account.ID)
	if err != nil {
		return fmt.Errorf("outbox: get follower inboxes: %w", err)
	}
	seen := make(map[string]bool)
	for _, inbox := range inboxURLs {
		if inbox == "" || seen[inbox] {
			continue
		}
		seen[inbox] = true
		msg := DeliveryMessage{
			ActivityID:  activityID,
			Activity:    raw,
			TargetInbox: inbox,
			SenderID:    account.ID,
		}
		if err := p.enqueue.EnqueueDelivery(ctx, "create", msg); err != nil {
			p.logger.Warn("outbox: enqueue create failed", slog.String("inbox", inbox), slog.Any("error", err))
		}
	}
	return nil
}

// DeleteStatus delivers a Delete{Tombstone} activity to the author's followers' inboxes.
func (p *OutboxPublisher) DeleteStatus(ctx context.Context, status *domain.Status) error {
	if p.enqueue == nil {
		return nil
	}
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
	inboxURLs, err := p.store.GetFollowerInboxURLs(ctx, account.ID)
	if err != nil {
		return fmt.Errorf("outbox: get follower inboxes: %w", err)
	}
	seen := make(map[string]bool)
	for _, inbox := range inboxURLs {
		if inbox == "" || seen[inbox] {
			continue
		}
		seen[inbox] = true
		msg := DeliveryMessage{
			ActivityID:  objectID + "#delete",
			Activity:    raw,
			TargetInbox: inbox,
			SenderID:    account.ID,
		}
		if err := p.enqueue.EnqueueDelivery(ctx, "delete", msg); err != nil {
			p.logger.Warn("outbox: enqueue delete failed", slog.String("inbox", inbox), slog.Any("error", err))
		}
	}
	return nil
}

// PublishFollow delivers a Follow activity to the target's inbox (single delivery).
func (p *OutboxPublisher) PublishFollow(ctx context.Context, actor, target *domain.Account, followID string) error {
	if p.enqueue == nil || target.InboxURL == "" {
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
	msg := DeliveryMessage{
		ActivityID:  activityID,
		Activity:    raw,
		TargetInbox: target.InboxURL,
		SenderID:    actor.ID,
	}
	if err := p.enqueue.EnqueueDelivery(ctx, "follow", msg); err != nil {
		p.logger.Warn("outbox: enqueue follow failed", slog.String("target", target.InboxURL), slog.Any("error", err))
	}
	return nil
}

// PublishUndoFollow delivers an Undo{Follow} activity to the target's inbox.
func (p *OutboxPublisher) PublishUndoFollow(ctx context.Context, actor, target *domain.Account, followID string) error {
	if p.enqueue == nil || target.InboxURL == "" {
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
	msg := DeliveryMessage{
		ActivityID:  undoID,
		Activity:    raw,
		TargetInbox: target.InboxURL,
		SenderID:    actor.ID,
	}
	if err := p.enqueue.EnqueueDelivery(ctx, "undo", msg); err != nil {
		p.logger.Warn("outbox: enqueue undo follow failed", slog.String("target", target.InboxURL), slog.Any("error", err))
	}
	return nil
}

// SendAcceptFollow implements AcceptFollowSender; delivers Accept{Follow} to the follower's inbox.
// target is the local account that accepted; actor is the remote follower.
func (p *OutboxPublisher) SendAcceptFollow(ctx context.Context, target, actor *domain.Account, followID string) error {
	if p.enqueue == nil || actor.InboxURL == "" {
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
	msg := DeliveryMessage{
		ActivityID:  acceptID,
		Activity:    raw,
		TargetInbox: actor.InboxURL,
		SenderID:    target.ID,
	}
	if err := p.enqueue.EnqueueDelivery(ctx, "accept", msg); err != nil {
		p.logger.Warn("outbox: enqueue accept follow failed", slog.String("target", actor.InboxURL), slog.Any("error", err))
	}
	return nil
}

func (p *OutboxPublisher) statusToNote(s *domain.Status, account *domain.Account) *Note {
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
