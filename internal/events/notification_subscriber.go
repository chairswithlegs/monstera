package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/natsutil"
)

// newAccountThreshold is the age below which an account is considered "new"
// for the FilterNewAccounts notification policy filter.
const newAccountThreshold = 30 * 24 * time.Hour

// NotificationDeps groups the service dependencies needed by the notification subscriber.
type NotificationDeps struct {
	Notifications      NotificationCreator
	Accounts           AccountLookup
	Conversations      ConversationMuteChecker
	Follows            FollowChecker
	Blocks             BlockChecker
	Mutes              MuteChecker
	UserDomainBlocks   UserDomainBlockChecker
	Statuses           StatusLookup
	ContentFilters     ContentFilterMatcher
	NotificationPolicy NotificationPolicyProvider
	DomainSilence      DomainSilenceChecker
}

// NotificationCreator is the service interface for creating notifications with events.
type NotificationCreator interface {
	CreateAndEmit(ctx context.Context, recipientID, fromAccountID, notifType string, statusID *string) error
}

// AccountLookup is the service interface for looking up accounts by ID.
type AccountLookup interface {
	GetByID(ctx context.Context, id string) (*domain.Account, error)
}

// ConversationMuteChecker checks if a viewer has muted the conversation containing a status.
type ConversationMuteChecker interface {
	IsConversationMutedForViewer(ctx context.Context, viewerAccountID, statusID string) (bool, error)
}

// FollowChecker checks follow relationships between accounts.
type FollowChecker interface {
	GetFollow(ctx context.Context, actorAccountID, targetAccountID string) (*domain.Follow, error)
}

// BlockChecker checks block relationships between accounts.
type BlockChecker interface {
	IsBlockedEitherDirection(ctx context.Context, accountID, targetID string) (bool, error)
}

// MuteChecker checks account-level mutes between accounts.
type MuteChecker interface {
	GetMute(ctx context.Context, accountID, targetID string) (*domain.Mute, error)
}

// UserDomainBlockChecker checks per-user domain blocks.
type UserDomainBlockChecker interface {
	IsUserDomainBlocked(ctx context.Context, accountID, domain string) (bool, error)
}

// StatusLookup retrieves a status by ID.
type StatusLookup interface {
	GetStatusByID(ctx context.Context, id string) (*domain.Status, error)
}

// ContentFilterMatcher checks whether a status matches any of a user's content filters.
type ContentFilterMatcher interface {
	StatusMatchesNotificationFilters(ctx context.Context, recipientID string, status *domain.Status) (bool, error)
}

// DomainSilenceChecker checks whether a domain is silenced (limited).
type DomainSilenceChecker interface {
	IsSilenced(ctx context.Context, domain string) bool
}

// NotificationPolicyProvider retrieves notification policies and creates notification requests.
type NotificationPolicyProvider interface {
	GetOrCreatePolicy(ctx context.Context, accountID string) (*domain.NotificationPolicy, error)
	UpsertNotificationRequest(ctx context.Context, accountID, fromAccountID string, lastStatusID *string) error
}

// NotificationSubscriber consumes domain events from DOMAIN_EVENTS and creates
// notifications reactively. This centralizes all notification creation logic,
// removing it from the inbox and inline service code.
type NotificationSubscriber struct {
	js   jetstream.JetStream
	deps NotificationDeps
}

// NewNotificationSubscriber creates a notification subscriber.
func NewNotificationSubscriber(js jetstream.JetStream, deps NotificationDeps) *NotificationSubscriber {
	return &NotificationSubscriber{js: js, deps: deps}
}

// Start subscribes to the domain-events-notifications consumer and processes
// messages until ctx is cancelled.
func (n *NotificationSubscriber) Start(ctx context.Context) error {
	if err := natsutil.RunConsumer(ctx, n.js, StreamDomainEvents, ConsumerNotifications,
		func(msg jetstream.Msg) { go n.processMessage(ctx, msg) },
		natsutil.WithLabel("notification subscriber"),
	); err != nil {
		return fmt.Errorf("notification subscriber: %w", err)
	}
	return nil
}

func (n *NotificationSubscriber) processMessage(ctx context.Context, msg jetstream.Msg) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "notification subscriber: panic in processMessage", slog.Any("panic", r), slog.String("subject", msg.Subject()))
			_ = msg.Nak()
		}
	}()
	var event domain.DomainEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		slog.WarnContext(ctx, "notification subscriber: invalid event payload", slog.Any("error", err))
		_ = msg.Ack()
		return
	}

	switch event.EventType {
	case domain.EventFollowCreated:
		n.handleFollowCreated(ctx, event)
	case domain.EventFollowRequested:
		n.handleFollowRequested(ctx, event)
	case domain.EventFavouriteCreated:
		n.handleFavouriteCreated(ctx, event)
	case domain.EventReblogCreated:
		n.handleReblogCreated(ctx, event)
	case domain.EventStatusCreated, domain.EventStatusCreatedRemote:
		n.handleStatusCreatedMentions(ctx, event)
	case domain.EventPollExpired:
		n.handlePollExpired(ctx, event)
	}
	_ = msg.Ack()
}

func (n *NotificationSubscriber) handleFollowCreated(ctx context.Context, event domain.DomainEvent) {
	var payload domain.FollowCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "notification subscriber: unmarshal follow.created", slog.Any("error", err))
		return
	}
	if payload.Target == nil || payload.Target.IsRemote() {
		return
	}
	if payload.Actor == nil {
		return
	}
	fc := filterContext{fromAccount: payload.Actor}
	n.createOrFilterNotification(ctx, payload.Target.ID, payload.Actor.ID, domain.NotificationTypeFollow, nil, fc)
}

func (n *NotificationSubscriber) handleFollowRequested(ctx context.Context, event domain.DomainEvent) {
	var payload domain.FollowRequestedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "notification subscriber: unmarshal follow.requested", slog.Any("error", err))
		return
	}
	if payload.Target == nil || payload.Target.IsRemote() {
		return
	}
	if payload.Actor == nil {
		return
	}
	fc := filterContext{fromAccount: payload.Actor}
	n.createOrFilterNotification(ctx, payload.Target.ID, payload.Actor.ID, domain.NotificationTypeFollowRequest, nil, fc)
}

func (n *NotificationSubscriber) handleFavouriteCreated(ctx context.Context, event domain.DomainEvent) {
	var payload domain.FavouriteCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "notification subscriber: unmarshal favourite.created", slog.Any("error", err))
		return
	}
	author, err := n.deps.Accounts.GetByID(ctx, payload.StatusAuthorID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.WarnContext(ctx, "notification subscriber: GetByID for favourite author",
				slog.String("account_id", payload.StatusAuthorID), slog.Any("error", err))
		}
		return
	}
	if author.IsRemote() {
		return
	}
	if payload.AccountID == payload.StatusAuthorID {
		return
	}
	statusID := payload.StatusID
	fc := filterContext{fromAccount: payload.FromAccount}
	n.createOrFilterNotification(ctx, payload.StatusAuthorID, payload.AccountID, domain.NotificationTypeFavourite, &statusID, fc)
}

func (n *NotificationSubscriber) handleReblogCreated(ctx context.Context, event domain.DomainEvent) {
	var payload domain.ReblogCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "notification subscriber: unmarshal reblog.created", slog.Any("error", err))
		return
	}
	author, err := n.deps.Accounts.GetByID(ctx, payload.OriginalAuthorID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.WarnContext(ctx, "notification subscriber: GetByID for reblog author",
				slog.String("account_id", payload.OriginalAuthorID), slog.Any("error", err))
		}
		return
	}
	if author.IsRemote() {
		return
	}
	if payload.AccountID == payload.OriginalAuthorID {
		return
	}
	statusID := payload.OriginalStatusID
	fc := filterContext{fromAccount: payload.FromAccount}
	n.createOrFilterNotification(ctx, payload.OriginalAuthorID, payload.AccountID, domain.NotificationTypeReblog, &statusID, fc)
}

func (n *NotificationSubscriber) handleStatusCreatedMentions(ctx context.Context, event domain.DomainEvent) {
	var payload domain.StatusCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "notification subscriber: unmarshal status.created (mentions)", slog.Any("error", err))
		return
	}
	if payload.Status == nil || payload.Author == nil {
		return
	}
	statusID := payload.Status.ID
	fc := filterContext{
		fromAccount:      payload.Author,
		statusVisibility: payload.Status.Visibility,
	}
	for _, mentioned := range payload.Mentions {
		if mentioned == nil || mentioned.IsRemote() {
			continue
		}
		if mentioned.ID == payload.Author.ID {
			continue
		}
		muted, err := n.deps.Conversations.IsConversationMutedForViewer(ctx, mentioned.ID, statusID)
		if err != nil {
			slog.WarnContext(ctx, "notification subscriber: IsConversationMutedForViewer", slog.Any("error", err))
			continue
		}
		if muted {
			continue
		}
		n.createOrFilterNotification(ctx, mentioned.ID, payload.Author.ID, domain.NotificationTypeMention, &statusID, fc)
	}
}

// handlePollExpired notifies the poll author when their poll closes.
// Per Mastodon behavior, poll authors are only notified on expiry, not on each vote.
func (n *NotificationSubscriber) handlePollExpired(ctx context.Context, event domain.DomainEvent) {
	var payload domain.PollUpdatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		slog.ErrorContext(ctx, "notification subscriber: unmarshal poll event", slog.Any("error", err))
		return
	}
	if payload.Status == nil || payload.Author == nil {
		return
	}
	// Only notify local poll authors.
	if payload.Author.IsRemote() {
		return
	}
	statusID := payload.Status.ID
	// Use the author as both the "from" and recipient for poll expiry notifications
	// (Mastodon sends "your poll has ended" from the author to themselves).
	fc := filterContext{fromAccount: payload.Author}
	n.createOrFilterNotification(ctx, payload.Author.ID, payload.Author.ID, domain.NotificationTypePoll, &statusID, fc)
}

// filterContext carries optional context needed for notification policy filtering.
type filterContext struct {
	// fromAccount is the sender account (used for FilterNewAccounts).
	// If nil, the account will be fetched from deps.Accounts.
	fromAccount *domain.Account
	// statusVisibility is the visibility of the related status (used for FilterPrivateMentions).
	statusVisibility string
}

func (n *NotificationSubscriber) createOrFilterNotification(ctx context.Context, recipientID, fromAccountID, notifType string, statusID *string, fc filterContext) {
	if recipientID != fromAccountID && n.deps.Blocks != nil {
		blocked, err := n.deps.Blocks.IsBlockedEitherDirection(ctx, recipientID, fromAccountID)
		if err != nil {
			slog.WarnContext(ctx, "notification subscriber: block check failed, dropping notification",
				slog.String("type", notifType),
				slog.String("recipient", recipientID),
				slog.Any("error", err),
			)
			return
		}
		if blocked {
			return
		}
	}

	if recipientID != fromAccountID && n.deps.Mutes != nil {
		mute, err := n.deps.Mutes.GetMute(ctx, recipientID, fromAccountID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			slog.WarnContext(ctx, "notification subscriber: mute check failed, allowing notification",
				slog.String("type", notifType),
				slog.String("recipient", recipientID),
				slog.Any("error", err),
			)
		}
		if mute != nil && mute.HideNotifications {
			return
		}
	}

	// Resolve the sender account once for domain-level checks (user domain blocks, domain silence).
	from := fc.fromAccount
	if from == nil && recipientID != fromAccountID && (n.deps.UserDomainBlocks != nil || (n.deps.DomainSilence != nil && n.deps.NotificationPolicy != nil)) {
		var lookupErr error
		from, lookupErr = n.deps.Accounts.GetByID(ctx, fromAccountID)
		if lookupErr != nil {
			slog.WarnContext(ctx, "notification subscriber: account lookup for domain checks failed",
				slog.String("from_account_id", fromAccountID),
				slog.Any("error", lookupErr),
			)
		}
	}

	if recipientID != fromAccountID && n.deps.UserDomainBlocks != nil {
		if from != nil && from.IsRemote() && from.Domain != nil {
			blocked, err := n.deps.UserDomainBlocks.IsUserDomainBlocked(ctx, recipientID, *from.Domain)
			if err != nil {
				slog.WarnContext(ctx, "notification subscriber: user domain block check failed",
					slog.String("type", notifType),
					slog.String("recipient", recipientID),
					slog.Any("error", err),
				)
			} else if blocked {
				return
			}
		}
	}

	if n.deps.DomainSilence != nil && n.deps.NotificationPolicy != nil {
		if from != nil && from.IsRemote() && n.deps.DomainSilence.IsSilenced(ctx, *from.Domain) {
			following, followErr := n.isFollowing(ctx, recipientID, fromAccountID)
			if followErr != nil {
				slog.WarnContext(ctx, "notification subscriber: follow check for silenced domain failed",
					slog.String("recipient", recipientID),
					slog.String("from_account_id", fromAccountID),
					slog.Any("error", followErr),
				)
			}
			if !following {
				if err := n.deps.NotificationPolicy.UpsertNotificationRequest(ctx, recipientID, fromAccountID, statusID); err != nil {
					slog.WarnContext(ctx, "notification subscriber: upsert notification request for silenced domain failed",
						slog.String("recipient", recipientID),
						slog.String("from_account_id", fromAccountID),
						slog.Any("error", err),
					)
				}
				return
			}
		}
	}

	if n.deps.NotificationPolicy != nil {
		filtered, err := n.shouldFilter(ctx, recipientID, fromAccountID, notifType, fc)
		if err != nil {
			slog.WarnContext(ctx, "notification subscriber: policy check failed, allowing notification",
				slog.String("type", notifType),
				slog.String("recipient", recipientID),
				slog.Any("error", err),
			)
		} else if filtered {
			if err := n.deps.NotificationPolicy.UpsertNotificationRequest(ctx, recipientID, fromAccountID, statusID); err != nil {
				slog.WarnContext(ctx, "notification subscriber: upsert notification request failed",
					slog.String("type", notifType),
					slog.String("recipient", recipientID),
					slog.Any("error", err),
				)
			}
			return
		}
	}
	if statusID != nil && n.deps.ContentFilters != nil && n.deps.Statuses != nil {
		status, err := n.deps.Statuses.GetStatusByID(ctx, *statusID)
		if err != nil {
			slog.WarnContext(ctx, "notification subscriber: status lookup for content filter failed",
				slog.String("status_id", *statusID),
				slog.Any("error", err),
			)
		} else {
			matched, matchErr := n.deps.ContentFilters.StatusMatchesNotificationFilters(ctx, recipientID, status)
			if matchErr != nil {
				slog.WarnContext(ctx, "notification subscriber: content filter check failed",
					slog.String("recipient", recipientID),
					slog.Any("error", matchErr),
				)
			} else if matched {
				return
			}
		}
	}
	if err := n.deps.Notifications.CreateAndEmit(ctx, recipientID, fromAccountID, notifType, statusID); err != nil {
		slog.WarnContext(ctx, "notification subscriber: create notification failed",
			slog.String("type", notifType),
			slog.String("recipient", recipientID),
			slog.Any("error", err),
		)
	}
}

// shouldFilter checks whether the notification should be filtered by the recipient's policy.
func (n *NotificationSubscriber) shouldFilter(ctx context.Context, recipientID, fromAccountID, notifType string, fc filterContext) (bool, error) {
	policy, err := n.deps.NotificationPolicy.GetOrCreatePolicy(ctx, recipientID)
	if err != nil {
		return false, fmt.Errorf("get policy: %w", err)
	}

	// Quick check: if no filters are enabled, skip all lookups.
	if !policy.FilterNotFollowing.ShouldFilter() &&
		!policy.FilterNotFollowers.ShouldFilter() &&
		!policy.FilterNewAccounts.ShouldFilter() &&
		!policy.FilterPrivateMentions.ShouldFilter() {
		return false, nil
	}

	if policy.FilterNotFollowing.ShouldFilter() {
		following, err := n.isFollowing(ctx, recipientID, fromAccountID)
		if err != nil {
			return false, fmt.Errorf("check following: %w", err)
		}
		if !following {
			return true, nil
		}
	}

	if policy.FilterNotFollowers.ShouldFilter() {
		followedBy, err := n.isFollowing(ctx, fromAccountID, recipientID)
		if err != nil {
			return false, fmt.Errorf("check followers: %w", err)
		}
		if !followedBy {
			return true, nil
		}
	}

	if policy.FilterNewAccounts.ShouldFilter() {
		from := fc.fromAccount
		if from == nil {
			from, err = n.deps.Accounts.GetByID(ctx, fromAccountID)
			if err != nil {
				return false, fmt.Errorf("get from account: %w", err)
			}
		}
		if time.Since(from.CreatedAt) < newAccountThreshold {
			return true, nil
		}
	}

	if policy.FilterPrivateMentions.ShouldFilter() {
		if notifType == domain.NotificationTypeMention && fc.statusVisibility == domain.VisibilityDirect {
			// Exception: allow through if recipient follows the sender.
			following, err := n.isFollowing(ctx, recipientID, fromAccountID)
			if err != nil {
				return false, fmt.Errorf("check following for private mention: %w", err)
			}
			if !following {
				return true, nil
			}
		}
	}

	return false, nil
}

// isFollowing checks whether actor follows target with an accepted follow.
func (n *NotificationSubscriber) isFollowing(ctx context.Context, actorID, targetID string) (bool, error) {
	f, err := n.deps.Follows.GetFollow(ctx, actorID, targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return f.State == domain.FollowStateAccepted, nil
}
