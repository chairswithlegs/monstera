package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/events"
	"github.com/chairswithlegs/monstera/internal/natsutil"
	"github.com/chairswithlegs/monstera/internal/observability"
	"github.com/chairswithlegs/monstera/internal/service"
)

const streamNameUser = "user"

// SubscriberStore is the minimal store interface the SSE subscriber needs for
// routing events to the correct streams.
type SubscriberStore interface {
	GetLocalFollowerAccountIDs(ctx context.Context, targetID string) ([]string, error)
	GetListIDsByMemberAccountID(ctx context.Context, accountID string) ([]string, error)
}

// subscriberStatusService is the minimal StatusService interface the SSE subscriber
// needs to enrich statuses for streamed events.
type subscriberStatusService interface {
	GetByIDEnriched(ctx context.Context, id string, viewerAccountID *string) (service.EnrichedStatus, error)
}

// Subscriber consumes domain events from the DOMAIN_EVENTS stream and
// publishes SSE events to NATS core subjects for the Hub to fan out.
type Subscriber struct {
	js             jetstream.JetStream
	nc             natsutil.Publisher
	store          SubscriberStore
	statusSvc      subscriberStatusService
	instanceDomain string
}

// NewSubscriber creates an SSE subscriber.
func NewSubscriber(
	js jetstream.JetStream,
	nc natsutil.Publisher,
	store SubscriberStore,
	statusSvc subscriberStatusService,
	instanceDomain string,
) *Subscriber {
	return &Subscriber{
		js:             js,
		nc:             nc,
		store:          store,
		statusSvc:      statusSvc,
		instanceDomain: instanceDomain,
	}
}

// Start subscribes to the domain-events-sse consumer and processes messages
// until ctx is cancelled.
func (s *Subscriber) Start(ctx context.Context) error {
	if err := natsutil.RunConsumer(ctx, s.js, events.StreamDomainEvents, events.ConsumerSSE,
		func(msg jetstream.Msg) { go s.processMessage(ctx, msg) },
		natsutil.WithMaxMessages(20),
		natsutil.WithLabel("sse subscriber"),
	); err != nil {
		return fmt.Errorf("sse subscriber: %w", err)
	}
	return nil
}

func (s *Subscriber) processMessage(ctx context.Context, msg jetstream.Msg) {
	var event domain.DomainEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		slog.Warn("sse subscriber: invalid event payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}

	switch event.EventType {
	case domain.EventStatusCreated, domain.EventStatusCreatedRemote:
		s.handleStatusCreated(ctx, event)
	case domain.EventStatusDeleted, domain.EventStatusDeletedRemote:
		s.handleStatusDeleted(ctx, event)
	case domain.EventNotificationCreated:
		s.handleNotificationCreated(ctx, event)
	default:
		// Federation-only or unknown events — ACK and skip.
	}
	_ = msg.Ack()
}

func (s *Subscriber) handleStatusCreated(ctx context.Context, event domain.DomainEvent) {
	var payload domain.StatusCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "sse subscriber: unmarshal status.created", slog.Any("error", err))
		return
	}
	if payload.Status == nil || payload.Author == nil {
		return
	}

	author := apimodel.ToAccount(payload.Author, s.instanceDomain)
	mentions := make([]apimodel.Mention, 0, len(payload.Mentions))
	for _, m := range payload.Mentions {
		if m != nil {
			mentions = append(mentions, apimodel.MentionFromAccount(m, s.instanceDomain))
		}
	}
	tags := make([]apimodel.Tag, 0, len(payload.Tags))
	for _, t := range payload.Tags {
		tags = append(tags, apimodel.TagFromName(t.Name, s.instanceDomain))
	}
	media := make([]apimodel.MediaAttachment, 0, len(payload.Media))
	for i := range payload.Media {
		media = append(media, apimodel.MediaFromDomain(&payload.Media[i]))
	}
	apiStatus := apimodel.ToStatus(payload.Status, author, mentions, tags, media, nil, s.instanceDomain)
	if payload.Status.ReblogOfID != nil {
		if enriched, err := s.statusSvc.GetByIDEnriched(ctx, *payload.Status.ReblogOfID, nil); err == nil {
			orig := apimodel.StatusFromEnriched(enriched, s.instanceDomain)
			apiStatus.Reblog = &orig
		}
	}
	statusJSON, err := json.Marshal(apiStatus)
	if err != nil {
		slog.ErrorContext(ctx, "sse subscriber: marshal status", slog.Any("error", err))
		return
	}

	hashtagNames := make([]string, 0, len(payload.Tags))
	for _, t := range payload.Tags {
		hashtagNames = append(hashtagNames, t.Name)
	}

	s.routeStatusCreated(ctx, payload.Status.AccountID, payload.Status.Visibility, payload.Status.Local, statusJSON, hashtagNames, payload.MentionedAccountIDs)
}

func (s *Subscriber) routeStatusCreated(ctx context.Context, accountID, visibility string, local bool, statusJSON []byte, hashtagNames, mentionedAccountIDs []string) {
	ev := SSEEvent{Event: EventUpdate, Data: string(statusJSON)}

	switch visibility {
	case domain.VisibilityPublic:
		ev.Stream = StreamPublic
		s.publish(ctx, SubjectPrefixPublic, ev, "events.public")
		if local {
			ev.Stream = StreamPublicLocal
			s.publish(ctx, SubjectPrefixPublicLocal, ev, "events.public.local")
		}
		fallthrough
	case domain.VisibilityUnlisted:
		followerIDs, err := s.store.GetLocalFollowerAccountIDs(ctx, accountID)
		if err != nil {
			slog.ErrorContext(ctx, "sse subscriber: get local followers", slog.Any("error", err), slog.String("account_id", accountID))
		} else {
			ev.Stream = streamNameUser
			for _, fid := range followerIDs {
				subj := StreamKeyToSubject(StreamUserPrefix + fid)
				if subj != "" {
					s.publish(ctx, subj, ev, "events.user.*")
				}
			}
		}
		if visibility == domain.VisibilityUnlisted || visibility == domain.VisibilityPublic {
			for _, tag := range hashtagNames {
				if tag == "" {
					continue
				}
				ev.Stream = StreamHashtagPrefix + tag
				subj := StreamKeyToSubject(ev.Stream)
				if subj != "" {
					s.publish(ctx, subj, ev, "events.hashtag.*")
				}
			}
		}
		s.publishToListStreams(ctx, accountID, ev)
	case domain.VisibilityPrivate:
		followerIDs, err := s.store.GetLocalFollowerAccountIDs(ctx, accountID)
		if err != nil {
			slog.ErrorContext(ctx, "sse subscriber: get local followers", slog.Any("error", err), slog.String("account_id", accountID))
		} else {
			ev.Stream = streamNameUser
			for _, fid := range followerIDs {
				subj := StreamKeyToSubject(StreamUserPrefix + fid)
				if subj != "" {
					s.publish(ctx, subj, ev, "events.user.*")
				}
			}
		}
		s.publishToListStreams(ctx, accountID, ev)
	case domain.VisibilityDirect:
		ev.Stream = streamNameUser
		for _, fid := range mentionedAccountIDs {
			if fid == "" {
				continue
			}
			subj := StreamKeyToSubject(StreamUserPrefix + fid)
			if subj != "" {
				s.publish(ctx, subj, ev, "events.user.*")
			}
		}
		s.publishToDirectStreams(ctx, mentionedAccountIDs, ev)
	}
}

func (s *Subscriber) publishToListStreams(ctx context.Context, accountID string, ev SSEEvent) {
	listIDs, err := s.store.GetListIDsByMemberAccountID(ctx, accountID)
	if err != nil {
		slog.ErrorContext(ctx, "sse subscriber: get lists for account", slog.Any("error", err), slog.String("account_id", accountID))
		return
	}
	for _, lid := range listIDs {
		ev.Stream = StreamListPrefix + lid
		subj := StreamKeyToSubject(ev.Stream)
		if subj != "" {
			s.publish(ctx, subj, ev, "events.list.*")
		}
	}
}

func (s *Subscriber) publishToDirectStreams(ctx context.Context, mentionedAccountIDs []string, ev SSEEvent) {
	for _, fid := range mentionedAccountIDs {
		if fid == "" {
			continue
		}
		ev.Stream = StreamDirectPrefix + fid
		subj := StreamKeyToSubject(ev.Stream)
		if subj != "" {
			s.publish(ctx, subj, ev, "events.direct.*")
		}
	}
}

func (s *Subscriber) handleStatusDeleted(ctx context.Context, event domain.DomainEvent) {
	var payload domain.StatusDeletedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "sse subscriber: unmarshal status.deleted", slog.Any("error", err))
		return
	}

	ev := SSEEvent{Event: EventDelete, Data: payload.StatusID}

	if payload.Visibility == domain.VisibilityPublic {
		ev.Stream = StreamPublic
		s.publish(ctx, SubjectPrefixPublic, ev, "events.public")
		if payload.Local {
			ev.Stream = StreamPublicLocal
			s.publish(ctx, SubjectPrefixPublicLocal, ev, "events.public.local")
		}
	}

	followerIDs, err := s.store.GetLocalFollowerAccountIDs(ctx, payload.AccountID)
	if err != nil {
		slog.ErrorContext(ctx, "sse subscriber: get local followers for delete", slog.Any("error", err), slog.String("account_id", payload.AccountID))
	} else {
		ev.Stream = streamNameUser
		for _, fid := range followerIDs {
			subj := StreamKeyToSubject(StreamUserPrefix + fid)
			if subj != "" {
				s.publish(ctx, subj, ev, "events.user.*")
			}
		}
	}

	for _, tag := range payload.HashtagNames {
		if tag == "" {
			continue
		}
		ev.Stream = StreamHashtagPrefix + tag
		subj := StreamKeyToSubject(ev.Stream)
		if subj != "" {
			s.publish(ctx, subj, ev, "events.hashtag.*")
		}
	}

	if payload.Visibility == domain.VisibilityDirect {
		ev.Stream = streamNameUser
		for _, fid := range payload.MentionedAccountIDs {
			if fid == "" {
				continue
			}
			subj := StreamKeyToSubject(StreamUserPrefix + fid)
			if subj != "" {
				s.publish(ctx, subj, ev, "events.user.*")
			}
		}
	}
}

func (s *Subscriber) handleNotificationCreated(ctx context.Context, event domain.DomainEvent) {
	var payload domain.NotificationCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "sse subscriber: unmarshal notification.created", slog.Any("error", err))
		return
	}
	if payload.Notification == nil || payload.FromAccount == nil {
		return
	}

	var status *apimodel.Status
	if payload.StatusID != nil && *payload.StatusID != "" {
		viewerID := &payload.RecipientAccountID
		if enriched, err := s.statusSvc.GetByIDEnriched(ctx, *payload.StatusID, viewerID); err == nil {
			apiSt := apimodel.StatusFromEnriched(enriched, s.instanceDomain)
			status = &apiSt
		}
	}

	notif := apimodel.ToNotification(payload.Notification, payload.FromAccount, status, s.instanceDomain)
	notifJSON, err := json.Marshal(notif)
	if err != nil {
		slog.ErrorContext(ctx, "sse subscriber: marshal notification", slog.Any("error", err))
		return
	}

	ev := SSEEvent{Stream: streamNameUser, Event: EventNotification, Data: string(notifJSON)}
	subj := StreamKeyToSubject(StreamUserPrefix + payload.RecipientAccountID)
	if subj != "" {
		s.publish(ctx, subj, ev, "events.user.*")
	}
}

func (s *Subscriber) publish(_ context.Context, subject string, ev SSEEvent, metricSubject string) {
	data, err := json.Marshal(ev)
	if err != nil {
		slog.Error("sse subscriber: marshal SSEEvent", slog.Any("error", err))
		observability.IncNATSPublish(metricSubject, "error")
		return
	}
	if err := s.nc.Publish(subject, data); err != nil {
		slog.Error("sse subscriber: publish", slog.Any("error", err), slog.String("subject", subject))
		observability.IncNATSPublish(metricSubject, "error")
		return
	}
	observability.IncNATSPublish(metricSubject, "ok")
}
