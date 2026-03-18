package activitypub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/blocklist"
	"github.com/chairswithlegs/monstera/internal/service"
)

// ErrInboxFatal represent an inbox error that should not be retried.
var ErrInboxFatal = errors.New("fatal inbox error")

// Inbox processes incoming ActivityPub activities.
type Inbox interface {
	Process(ctx context.Context, activity *vocab.Activity) error
}

// NewInbox constructs an Inbox. The inbox is a pure AP-to-service translation
// layer: it calls service methods which internally emit domain events for
// federation and SSE.
func NewInbox(
	accounts service.AccountService,
	follows service.FollowService,
	remoteFollows service.RemoteFollowService,
	statuses service.StatusService,
	remoteStatusWrites service.RemoteStatusWriteService,
	media service.MediaService,
	remoteResolver *RemoteAccountResolver,
	bl *blocklist.BlocklistCache,
	instanceDomain string,
) Inbox {
	return &inbox{
		accounts:           accounts,
		follows:            follows,
		remoteFollows:      remoteFollows,
		statuses:           statuses,
		remoteStatusWrites: remoteStatusWrites,
		media:              media,
		remoteResolver:     remoteResolver,
		blocklist:          bl,
		instanceDomain:     instanceDomain,
	}
}

// inbox dispatches verified incoming ActivityPub activities to type-specific handlers.
type inbox struct {
	accounts           service.AccountService
	follows            service.FollowService
	remoteFollows      service.RemoteFollowService
	statuses           service.StatusService
	remoteStatusWrites service.RemoteStatusWriteService
	media              service.MediaService
	remoteResolver     *RemoteAccountResolver
	blocklist          *blocklist.BlocklistCache
	instanceDomain     string
}

// Process dispatches a verified incoming activity to the appropriate handler.
func (p *inbox) Process(ctx context.Context, activity *vocab.Activity) error {
	slog.DebugContext(ctx, "inbox: processing activity",
		slog.String("type", string(activity.Type)), slog.String("id", activity.ID), slog.String("actor", activity.Actor))

	actorDomain := vocab.DomainFromIRI(activity.Actor)
	if actorDomain == "" {
		return fmt.Errorf("%w: cannot extract domain from actor %q", ErrInboxFatal, activity.Actor)
	}
	if actorDomain == p.instanceDomain {
		return fmt.Errorf("%w: activities from own domain are illegitimate", ErrInboxFatal)
	}
	if p.blocklist.IsSuspended(ctx, actorDomain) {
		slog.DebugContext(ctx, "inbox: dropped activity from suspended domain",
			slog.String("domain", actorDomain),
			slog.String("type", string(activity.Type)),
			slog.String("id", activity.ID),
		)
		return nil
	}
	switch activity.Type {
	case vocab.ObjectTypeFollow:
		return p.handleFollow(ctx, activity)
	case vocab.ObjectTypeAccept:
		return p.handleAcceptFollow(ctx, activity)
	case vocab.ObjectTypeReject:
		return p.handleRejectFollow(ctx, activity)
	case vocab.ObjectTypeUndo:
		return p.handleUndo(ctx, activity)
	case vocab.ObjectTypeCreate:
		return p.handleCreate(ctx, activity, actorDomain)
	case vocab.ObjectTypeAnnounce:
		return p.handleAnnounce(ctx, activity, actorDomain)
	case vocab.ObjectTypeLike:
		return p.handleLike(ctx, activity)
	case vocab.ObjectTypeDelete:
		return p.handleDelete(ctx, activity)
	case vocab.ObjectTypeUpdate:
		return p.handleUpdate(ctx, activity)
	case vocab.ObjectTypeBlock:
		return p.handleBlock(ctx, activity)
	default:
		slog.DebugContext(ctx, "inbox: unsupported activity type", slog.String("type", string(activity.Type)), slog.String("id", activity.ID))
		return nil
	}
}
