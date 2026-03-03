package mastodon

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/chairswithlegs/monstera-fed/internal/activitypub"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/events"
	"github.com/chairswithlegs/monstera-fed/internal/events/sse"
	"github.com/chairswithlegs/monstera-fed/internal/observability"
)

const (
	// NATS subjects
	metricSubjectPublic      = "events.public"
	metricSubjectPublicLocal = "events.public.local"
	metricSubjectUser        = "events.user.*"
	metricSubjectHashtag     = "events.hashtag.*"

	// SSE stream keys
	streamNameUser = "user"
)

// PublisherStore is the minimal store interface needed by the Mastodon SSE publisher.
type PublisherStore interface {
	GetLocalFollowerAccountIDs(ctx context.Context, targetID string) ([]string, error)
	GetStatusByID(ctx context.Context, id string) (*domain.Status, error)
	GetAccountByID(ctx context.Context, id string) (*domain.Account, error)
	GetStatusMentions(ctx context.Context, statusID string) ([]*domain.Account, error)
	GetStatusHashtags(ctx context.Context, statusID string) ([]domain.Hashtag, error)
	GetStatusAttachments(ctx context.Context, statusID string) ([]domain.MediaAttachment, error)
}

// natsPublisher is the subset of *nats.Conn used for publishing (allows testing without real NATS).
type natsPublisher interface {
	Publish(subject string, data []byte) error
}

// Publisher publishes Mastodon SSE events to NATS for the SSE Hub to fan out.
type Publisher struct {
	nc             natsPublisher
	store          PublisherStore
	metrics        *observability.Metrics
	logger         *slog.Logger
	instanceDomain string
}

// NewPublisher returns a Publisher that implements service.EventBus and activitypub.InboxEventPublisher.
// nc is typically *nats.Conn; use a mock natsPublisher in tests.
func NewPublisher(nc natsPublisher, s PublisherStore, metrics *observability.Metrics, logger *slog.Logger, instanceDomain string) *Publisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Publisher{
		nc:             nc,
		store:          s,
		metrics:        metrics,
		logger:         logger,
		instanceDomain: instanceDomain,
	}
}

// Ensure Publisher implements both interfaces.
var (
	_ events.EventBus                 = (*Publisher)(nil)
	_ activitypub.InboxEventPublisher = (*Publisher)(nil)
)

func (p *Publisher) publish(_ context.Context, subject string, ev sse.SSEEvent, metricSubject string) {
	data, err := json.Marshal(ev)
	if err != nil {
		p.logger.Error("sse: marshal SSEEvent", slog.Any("error", err))
		p.metrics.NATSPublishTotal.WithLabelValues(metricSubject, "error").Inc()
		return
	}
	if err := p.nc.Publish(subject, data); err != nil {
		p.logger.Error("sse: publish", slog.Any("error", err), slog.String("subject", subject))
		p.metrics.NATSPublishTotal.WithLabelValues(metricSubject, "error").Inc()
		return
	}
	p.metrics.NATSPublishTotal.WithLabelValues(metricSubject, "ok").Inc()
}

func (p *Publisher) PublishStatusCreated(ctx context.Context, data events.StatusCreatedEvent) {
	if data.Status == nil || data.Author == nil {
		return
	}
	author := apimodel.ToAccount(data.Author, p.instanceDomain)
	mentions := make([]apimodel.Mention, 0, len(data.Mentions))
	for _, m := range data.Mentions {
		if m != nil {
			mentions = append(mentions, apimodel.MentionFromAccount(m, p.instanceDomain))
		}
	}
	tags := make([]apimodel.Tag, 0, len(data.Tags))
	for _, t := range data.Tags {
		tags = append(tags, apimodel.TagFromName(t.Name, p.instanceDomain))
	}
	media := make([]apimodel.MediaAttachment, 0, len(data.Media))
	for i := range data.Media {
		media = append(media, apimodel.MediaFromDomain(&data.Media[i]))
	}
	apiStatus := apimodel.ToStatus(data.Status, author, mentions, tags, media, p.instanceDomain)
	statusJSON, err := json.Marshal(apiStatus)
	if err != nil {
		p.logger.Error("sse: marshal status", slog.Any("error", err))
		return
	}
	hashtagNames := make([]string, 0, len(data.Tags))
	for _, t := range data.Tags {
		hashtagNames = append(hashtagNames, t.Name)
	}
	p.publishStatusCreatedPayload(ctx, data.Status.AccountID, data.Status.Visibility, data.Status.Local, data.Status.ID, statusJSON, hashtagNames, data.MentionedAccountIDs)
}

func (p *Publisher) publishStatusCreatedPayload(ctx context.Context, accountID, visibility string, local bool, _ string, statusJSON []byte, hashtagNames []string, mentionedAccountIDs []string) {
	ev := sse.SSEEvent{Event: sse.EventUpdate, Data: string(statusJSON)}

	switch visibility {
	case domain.VisibilityPublic:
		ev.Stream = sse.StreamPublic
		p.publish(ctx, sse.SubjectPrefixPublic, ev, metricSubjectPublic)
		if local {
			ev.Stream = sse.StreamPublicLocal
			p.publish(ctx, sse.SubjectPrefixPublicLocal, ev, metricSubjectPublicLocal)
		}
		fallthrough
	case domain.VisibilityUnlisted:
		followerIDs, err := p.store.GetLocalFollowerAccountIDs(ctx, accountID)
		if err != nil {
			p.logger.Error("sse: get local followers", slog.Any("error", err), slog.String("account_id", accountID))
		} else {
			ev.Stream = streamNameUser
			for _, fid := range followerIDs {
				subj := sse.StreamKeyToSubject(sse.StreamUserPrefix + fid)
				if subj != "" {
					p.publish(ctx, subj, ev, metricSubjectUser)
				}
			}
		}
		if visibility == domain.VisibilityUnlisted || visibility == domain.VisibilityPublic {
			for _, tag := range hashtagNames {
				if tag == "" {
					continue
				}
				ev.Stream = sse.StreamHashtagPrefix + tag
				subj := sse.StreamKeyToSubject(ev.Stream)
				if subj != "" {
					p.publish(ctx, subj, ev, metricSubjectHashtag)
				}
			}
		}
	case domain.VisibilityPrivate:
		followerIDs, err := p.store.GetLocalFollowerAccountIDs(ctx, accountID)
		if err != nil {
			p.logger.Error("sse: get local followers", slog.Any("error", err), slog.String("account_id", accountID))
		} else {
			ev.Stream = streamNameUser
			for _, fid := range followerIDs {
				subj := sse.StreamKeyToSubject(sse.StreamUserPrefix + fid)
				if subj != "" {
					p.publish(ctx, subj, ev, metricSubjectUser)
				}
			}
		}
	case domain.VisibilityDirect:
		ev.Stream = streamNameUser
		for _, fid := range mentionedAccountIDs {
			if fid == "" {
				continue
			}
			subj := sse.StreamKeyToSubject(sse.StreamUserPrefix + fid)
			if subj != "" {
				p.publish(ctx, subj, ev, metricSubjectUser)
			}
		}
	}
}

func (p *Publisher) PublishStatusDeleted(ctx context.Context, data events.StatusDeletedEvent) {
	ev := sse.SSEEvent{Event: sse.EventDelete, Data: data.StatusID}

	if data.Visibility == domain.VisibilityPublic {
		ev.Stream = sse.StreamPublic
		p.publish(ctx, sse.SubjectPrefixPublic, ev, metricSubjectPublic)
		if data.Local {
			ev.Stream = sse.StreamPublicLocal
			p.publish(ctx, sse.SubjectPrefixPublicLocal, ev, metricSubjectPublicLocal)
		}
	}

	followerIDs, err := p.store.GetLocalFollowerAccountIDs(ctx, data.AccountID)
	if err != nil {
		p.logger.Error("sse: get local followers for delete", slog.Any("error", err), slog.String("account_id", data.AccountID))
	} else {
		ev.Stream = streamNameUser
		for _, fid := range followerIDs {
			subj := sse.StreamKeyToSubject(sse.StreamUserPrefix + fid)
			if subj != "" {
				p.publish(ctx, subj, ev, metricSubjectUser)
			}
		}
	}

	for _, tag := range data.HashtagNames {
		if tag == "" {
			continue
		}
		ev.Stream = sse.StreamHashtagPrefix + tag
		subj := sse.StreamKeyToSubject(ev.Stream)
		if subj != "" {
			p.publish(ctx, subj, ev, metricSubjectHashtag)
		}
	}

	if data.Visibility == domain.VisibilityDirect {
		ev.Stream = streamNameUser
		for _, fid := range data.MentionedAccountIDs {
			if fid == "" {
				continue
			}
			subj := sse.StreamKeyToSubject(sse.StreamUserPrefix + fid)
			if subj != "" {
				p.publish(ctx, subj, ev, metricSubjectUser)
			}
		}
	}
}

func (p *Publisher) PublishNotificationCreated(ctx context.Context, data events.NotificationCreatedEvent) {
	if data.Notification == nil || data.FromAccount == nil {
		return
	}
	var status *apimodel.Status
	if data.StatusID != nil && *data.StatusID != "" {
		st, err := p.store.GetStatusByID(ctx, *data.StatusID)
		if err == nil && st != nil && st.DeletedAt == nil {
			author, _ := p.store.GetAccountByID(ctx, st.AccountID)
			var authorAcc apimodel.Account
			if author != nil {
				authorAcc = apimodel.ToAccount(author, p.instanceDomain)
			}
			mentions, _ := p.store.GetStatusMentions(ctx, st.ID)
			mentionList := make([]apimodel.Mention, 0, len(mentions))
			for _, m := range mentions {
				if m != nil {
					mentionList = append(mentionList, apimodel.MentionFromAccount(m, p.instanceDomain))
				}
			}
			tags, _ := p.store.GetStatusHashtags(ctx, st.ID)
			tagList := make([]apimodel.Tag, 0, len(tags))
			for _, t := range tags {
				tagList = append(tagList, apimodel.TagFromName(t.Name, p.instanceDomain))
			}
			media, _ := p.store.GetStatusAttachments(ctx, st.ID)
			mediaList := make([]apimodel.MediaAttachment, 0, len(media))
			for i := range media {
				mediaList = append(mediaList, apimodel.MediaFromDomain(&media[i]))
			}
			s := apimodel.ToStatus(st, authorAcc, mentionList, tagList, mediaList, p.instanceDomain)
			status = &s
		}
	}
	notif := apimodel.ToNotification(data.Notification, data.FromAccount, status, p.instanceDomain)
	notifJSON, err := json.Marshal(notif)
	if err != nil {
		p.logger.Error("sse: marshal notification", slog.Any("error", err))
		return
	}
	ev := sse.SSEEvent{Stream: "user", Event: sse.EventNotification, Data: string(notifJSON)}
	subj := sse.StreamKeyToSubject(sse.StreamUserPrefix + data.RecipientAccountID)
	if subj != "" {
		p.publish(ctx, subj, ev, metricSubjectUser)
	}
}

func (p *Publisher) PublishStatusCreatedRaw(ctx context.Context, statusJSON json.RawMessage, opts activitypub.StatusEventOpts) {
	p.publishStatusCreatedPayload(ctx, opts.AccountID, opts.Visibility, opts.Local, "", statusJSON, opts.HashtagNames, opts.MentionedAccountIDs)
}

func (p *Publisher) PublishStatusDeletedRaw(ctx context.Context, statusID string, opts activitypub.StatusEventOpts) {
	p.PublishStatusDeleted(ctx, events.StatusDeletedEvent{
		StatusID:            statusID,
		AccountID:           opts.AccountID,
		Visibility:          opts.Visibility,
		Local:               opts.Local,
		HashtagNames:        opts.HashtagNames,
		MentionedAccountIDs: opts.MentionedAccountIDs,
	})
}

func (p *Publisher) PublishNotificationCreatedRaw(ctx context.Context, accountID string, notifJSON json.RawMessage) {
	ev := sse.SSEEvent{Stream: "user", Event: sse.EventNotification, Data: string(notifJSON)}
	subj := sse.StreamKeyToSubject(sse.StreamUserPrefix + accountID)
	if subj != "" {
		p.publish(ctx, subj, ev, metricSubjectUser)
	}
}
