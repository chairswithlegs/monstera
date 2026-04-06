package sse

import (
	"context"
	"encoding/json"
	"errors"
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
	GetListsByMemberAccountID(ctx context.Context, accountID string) ([]domain.List, error)
	ListListAccountIDs(ctx context.Context, listID string) ([]string, error)
	GetFollow(ctx context.Context, accountID, targetID string) (*domain.Follow, error)
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
	case domain.EventStatusUpdated, domain.EventStatusUpdatedRemote:
		s.handleStatusUpdated(ctx, event)
	case domain.EventPollUpdated, domain.EventPollExpired:
		s.handlePollUpdated(ctx, event)
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

	apiStatus := apimodel.StatusFromParts(payload.Status, payload.Author, payload.Mentions, payload.Tags, payload.Media, s.instanceDomain)
	if payload.Status.ReblogOfID != nil {
		if enriched, err := s.statusSvc.GetByIDEnriched(ctx, *payload.Status.ReblogOfID, nil); err == nil {
			orig := apimodel.StatusFromEnriched(enriched, s.instanceDomain)
			apiStatus.Reblog = &orig
		} else {
			slog.WarnContext(ctx, "sse subscriber: get reblog original", slog.Any("error", err), slog.String("reblog_of_id", *payload.Status.ReblogOfID))
		}
	}
	statusJSON, err := json.Marshal(apiStatus)
	if err != nil {
		slog.ErrorContext(ctx, "sse subscriber: marshal status", slog.Any("error", err))
		return
	}

	hashtagNames := hashtagNamesFromTags(payload.Tags)
	isReblog := payload.Status.ReblogOfID != nil
	s.routeStatusEvent(ctx, EventUpdate, payload.Status.AccountID, payload.Status.Visibility, isReblog, payload.Status.Local, statusJSON, hashtagNames, payload.MentionedAccountIDs, payload.Status)
}

// handlePollUpdated builds the SSE status update from the payload data directly
// (avoiding a DB round trip) and pushes a status.update SSE event.
func (s *Subscriber) handlePollUpdated(ctx context.Context, event domain.DomainEvent) {
	var payload domain.PollUpdatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "sse subscriber: unmarshal poll.updated", slog.Any("error", err))
		return
	}
	if payload.Status == nil || payload.Author == nil {
		return
	}
	// Build enriched status from payload data.
	enriched := service.EnrichedStatus{
		Status:   payload.Status,
		Author:   payload.Author,
		Mentions: payload.Mentions,
		Tags:     payload.Tags,
		Media:    payload.Media,
	}
	if payload.Poll != nil {
		opts := make([]service.PollOptionWithCount, len(payload.PollOptions))
		for i, o := range payload.PollOptions {
			opts[i] = service.PollOptionWithCount{Title: o.Title, VotesCount: o.VotesCount}
		}
		enriched.Poll = &service.EnrichedPoll{
			Poll:    *payload.Poll,
			Options: opts,
		}
	}
	apiStatus := apimodel.StatusFromEnriched(enriched, s.instanceDomain)
	statusJSON, err := json.Marshal(apiStatus)
	if err != nil {
		slog.ErrorContext(ctx, "sse subscriber: marshal status (poll update)", slog.Any("error", err))
		return
	}
	hashtagNames := hashtagNamesFromTags(payload.Tags)
	var mentionedIDs []string
	for _, m := range payload.Mentions {
		if m != nil {
			mentionedIDs = append(mentionedIDs, m.ID)
		}
	}
	s.routeStatusEvent(ctx, EventStatusUpdate, payload.Status.AccountID, payload.Status.Visibility, false, payload.Status.Local, statusJSON, hashtagNames, mentionedIDs, payload.Status)
}

func (s *Subscriber) handleStatusUpdated(ctx context.Context, event domain.DomainEvent) {
	var payload domain.StatusUpdatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "sse subscriber: unmarshal status.updated", slog.Any("error", err))
		return
	}
	if payload.Status == nil || payload.Author == nil {
		return
	}

	apiStatus := apimodel.StatusFromParts(payload.Status, payload.Author, payload.Mentions, payload.Tags, payload.Media, s.instanceDomain)
	statusJSON, err := json.Marshal(apiStatus)
	if err != nil {
		slog.ErrorContext(ctx, "sse subscriber: marshal status (update)", slog.Any("error", err))
		return
	}

	hashtagNames := hashtagNamesFromTags(payload.Tags)
	s.routeStatusEvent(ctx, EventStatusUpdate, payload.Status.AccountID, payload.Status.Visibility, false, payload.Status.Local, statusJSON, hashtagNames, payload.MentionedAccountIDs, payload.Status)
}

func hashtagNamesFromTags(tags []domain.Hashtag) []string {
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	return names
}

func (s *Subscriber) routeStatusEvent(ctx context.Context, eventType, accountID, visibility string, isReblog, local bool, statusJSON []byte, hashtagNames, mentionedAccountIDs []string, status *domain.Status) {
	ev := SSEEvent{Event: eventType, Data: string(statusJSON)}

	// Fetch lists containing the status author (used for both list streaming and exclusive filtering).
	lists, err := s.store.GetListsByMemberAccountID(ctx, accountID)
	if err != nil {
		slog.ErrorContext(ctx, "sse subscriber: get lists for account", slog.Any("error", err), slog.String("account_id", accountID))
		lists = nil
	}

	exclusiveOwners := exclusiveOwnerSet(lists)

	switch visibility {
	case domain.VisibilityPublic:
		// Reblogs are excluded from public timeline streams per the Mastodon spec.
		if !isReblog {
			ev.Stream = StreamPublic
			s.publish(ctx, SubjectPrefixPublic, ev, "events.public")
			if local {
				ev.Stream = StreamPublicLocal
				s.publish(ctx, SubjectPrefixPublicLocal, ev, "events.public.local")
			}
		}
		fallthrough
	case domain.VisibilityUnlisted:
		// Deliver to the author's own stream so they see their post in real time.
		ev.Stream = streamNameUser
		if subj := StreamKeyToSubject(StreamUserPrefix + accountID); subj != "" {
			s.publish(ctx, subj, ev, "events.user.*")
		}
		followerIDs, err := s.store.GetLocalFollowerAccountIDs(ctx, accountID)
		if err != nil {
			slog.ErrorContext(ctx, "sse subscriber: get local followers", slog.Any("error", err), slog.String("account_id", accountID))
		} else {
			ev.Stream = streamNameUser
			for _, fid := range followerIDs {
				if fid == accountID {
					continue // already delivered to author above
				}
				if _, excluded := exclusiveOwners[fid]; excluded {
					continue
				}
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
		s.publishToListStreams(ctx, lists, status, ev)
	case domain.VisibilityPrivate:
		// Deliver to the author's own stream so they see their post in real time.
		ev.Stream = streamNameUser
		if subj := StreamKeyToSubject(StreamUserPrefix + accountID); subj != "" {
			s.publish(ctx, subj, ev, "events.user.*")
		}
		followerIDs, err := s.store.GetLocalFollowerAccountIDs(ctx, accountID)
		if err != nil {
			slog.ErrorContext(ctx, "sse subscriber: get local followers", slog.Any("error", err), slog.String("account_id", accountID))
		} else {
			ev.Stream = streamNameUser
			for _, fid := range followerIDs {
				if fid == accountID {
					continue // already delivered to author above
				}
				if _, excluded := exclusiveOwners[fid]; excluded {
					continue
				}
				subj := StreamKeyToSubject(StreamUserPrefix + fid)
				if subj != "" {
					s.publish(ctx, subj, ev, "events.user.*")
				}
			}
		}
		s.publishToListStreams(ctx, lists, status, ev)
	case domain.VisibilityDirect:
		// Deliver to the author's own streams so they see the DM they sent in real time.
		ev.Stream = streamNameUser
		if subj := StreamKeyToSubject(StreamUserPrefix + accountID); subj != "" {
			s.publish(ctx, subj, ev, "events.user.*")
		}
		// Deliver to all mentioned accounts' user streams.
		for _, fid := range mentionedAccountIDs {
			if fid == "" || fid == accountID {
				continue
			}
			subj := StreamKeyToSubject(StreamUserPrefix + fid)
			if subj != "" {
				s.publish(ctx, subj, ev, "events.user.*")
			}
		}
		// Deliver to direct streams for author and all mentioned accounts.
		allDirect := make([]string, 0, len(mentionedAccountIDs)+1)
		allDirect = append(allDirect, accountID)
		for _, fid := range mentionedAccountIDs {
			if fid != "" && fid != accountID {
				allDirect = append(allDirect, fid)
			}
		}
		s.publishToDirectStreams(ctx, allDirect, ev)
	}
}

// exclusiveOwnerSet returns the set of list-owner account IDs that have the
// status author in an exclusive list. Used to skip home-stream delivery for
// those owners.
func exclusiveOwnerSet(lists []domain.List) map[string]struct{} {
	owners := make(map[string]struct{})
	for _, l := range lists {
		if l.Exclusive {
			owners[l.AccountID] = struct{}{}
		}
	}
	return owners
}

// publishToListStreams publishes SSE events to list streams, applying replies_policy filtering.
// For delete events (status is nil), events are published to all lists without filtering.
func (s *Subscriber) publishToListStreams(ctx context.Context, lists []domain.List, status *domain.Status, ev SSEEvent) {
	isReply := status != nil && status.InReplyToID != nil
	for _, list := range lists {
		if isReply {
			if !s.passesRepliesPolicy(ctx, list, status) {
				continue
			}
		}
		ev.Stream = StreamListPrefix + list.ID
		subj := StreamKeyToSubject(ev.Stream)
		if subj != "" {
			s.publish(ctx, subj, ev, "events.list.*")
		}
	}
}

// passesRepliesPolicy checks whether a reply should be delivered to the given list
// based on its replies_policy setting.
func (s *Subscriber) passesRepliesPolicy(ctx context.Context, list domain.List, status *domain.Status) bool {
	if status.InReplyToAccountID == nil {
		return false
	}
	switch list.RepliesPolicy {
	case domain.ListRepliesPolicyNone:
		return false
	case domain.ListRepliesPolicyList:
		memberIDs, err := s.store.ListListAccountIDs(ctx, list.ID)
		if err != nil {
			slog.ErrorContext(ctx, "sse subscriber: get list member IDs", slog.Any("error", err), slog.String("list_id", list.ID))
			return false
		}
		for _, mid := range memberIDs {
			if mid == *status.InReplyToAccountID {
				return true
			}
		}
		return false
	case domain.ListRepliesPolicyFollowed:
		_, err := s.store.GetFollow(ctx, list.AccountID, *status.InReplyToAccountID)
		if errors.Is(err, domain.ErrNotFound) {
			return false
		}
		if err != nil {
			slog.ErrorContext(ctx, "sse subscriber: check follow for list replies_policy", slog.Any("error", err))
			return false
		}
		return true
	default:
		return true
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

	// Fetch lists for the author (for list stream delivery and exclusive filtering).
	lists, err := s.store.GetListsByMemberAccountID(ctx, payload.AccountID)
	if err != nil {
		slog.ErrorContext(ctx, "sse subscriber: get lists for account (delete)", slog.Any("error", err), slog.String("account_id", payload.AccountID))
		lists = nil
	}

	exclusiveOwners := make(map[string]struct{})
	for _, l := range lists {
		if l.Exclusive {
			exclusiveOwners[l.AccountID] = struct{}{}
		}
	}

	if payload.Visibility == domain.VisibilityPublic {
		ev.Stream = StreamPublic
		s.publish(ctx, SubjectPrefixPublic, ev, "events.public")
		if payload.Local {
			ev.Stream = StreamPublicLocal
			s.publish(ctx, SubjectPrefixPublicLocal, ev, "events.public.local")
		}
	}

	// Deliver delete to the author's own stream.
	ev.Stream = streamNameUser
	if subj := StreamKeyToSubject(StreamUserPrefix + payload.AccountID); subj != "" {
		s.publish(ctx, subj, ev, "events.user.*")
	}

	followerIDs, err := s.store.GetLocalFollowerAccountIDs(ctx, payload.AccountID)
	if err != nil {
		slog.ErrorContext(ctx, "sse subscriber: get local followers for delete", slog.Any("error", err), slog.String("account_id", payload.AccountID))
	} else {
		ev.Stream = streamNameUser
		for _, fid := range followerIDs {
			if fid == payload.AccountID {
				continue // already delivered to author above
			}
			if _, excluded := exclusiveOwners[fid]; excluded {
				continue
			}
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

	// Publish delete to list streams (no replies_policy filtering for deletes).
	s.publishToListStreams(ctx, lists, nil, ev)

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
		} else {
			slog.WarnContext(ctx, "sse subscriber: get notification status", slog.Any("error", err), slog.String("status_id", *payload.StatusID))
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

	// Also publish to the notification-only stream so clients subscribed to
	// "user:notification" (e.g. Elk) receive notification events separately.
	notifEv := SSEEvent{Stream: StreamUserNotificationPrefix + payload.RecipientAccountID, Event: EventNotification, Data: string(notifJSON)}
	notifSubj := StreamKeyToSubject(StreamUserNotificationPrefix + payload.RecipientAccountID)
	if notifSubj != "" {
		s.publish(ctx, notifSubj, notifEv, "events.user.notification.*")
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
