package activitypub

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	ap "github.com/chairswithlegs/monstera/internal/activitypub"
	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/cache"
	"github.com/chairswithlegs/monstera/internal/config"
)

// InboxHandler handles POST /users/{username}/inbox and POST /inbox.
// Verifies HTTP Signature, parses the activity, and dispatches to InboxProcessor.
// Always returns 202 Accepted per ActivityPub spec.
type InboxHandler struct {
	inbox    ap.Inbox
	cache    cache.Store
	verifier *ap.HTTPSignatureService
}

// NewInboxHandler returns a new InboxHandler.
func NewInboxHandler(inbox ap.Inbox, cache cache.Store, cfg *config.Config, verifier *ap.HTTPSignatureService) *InboxHandler {
	return &InboxHandler{inbox: inbox, cache: cache, verifier: verifier}
}

// POSTInbox handles POST to the inbox.
func (h *InboxHandler) POSTInbox(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "application/activity+json") && !strings.Contains(ct, "application/ld+json") {
		slog.WarnContext(r.Context(), "inbox: bad request", slog.String("reason", "content type must be application/activity+json or application/ld+json"))
		err := api.NewBadRequestError("content type must be application/activity+json or application/ld+json")
		api.HandleError(w, r, err)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.WarnContext(r.Context(), "inbox: bad request", slog.String("reason", "failed to read body"), slog.Any("error", err))
		err := api.NewBadRequestError("failed to read body")
		api.HandleError(w, r, err)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	keyID, err := h.verifier.Verify(r.Context(), r)
	if err != nil {
		slog.WarnContext(r.Context(), "inbox: bad request", slog.String("reason", "signature verification failed"), slog.String("key_id", keyID), slog.Any("error", err))
		err := api.NewBadRequestError("signature verification failed")
		api.HandleError(w, r, err)
		return
	}
	var activity ap.Activity
	if err := json.Unmarshal(body, &activity); err != nil {
		slog.WarnContext(r.Context(), "inbox: bad request", slog.String("reason", "invalid activity json"), slog.Any("error", err))
		err := api.NewBadRequestError("invalid activity json")
		api.HandleError(w, r, err)
		return
	}
	// Enforce that the key used to sign belongs to the same domain as the activity actor.
	// Compliant activities have an actor IRI and key IDs are IRIs; both must parse to a host.
	keyDomain := ap.DomainFromKeyID(keyID)
	actorDomain := ap.DomainFromActorID(activity.Actor)
	if keyDomain == "" || actorDomain == "" {
		slog.WarnContext(r.Context(), "inbox: bad request", slog.String("reason", "cannot verify key attribution"), slog.String("key_id", keyID), slog.String("actor", activity.Actor))
		err := api.NewBadRequestError("cannot verify key attribution")
		api.HandleError(w, r, err)
		return
	}
	if keyDomain != actorDomain {
		slog.WarnContext(r.Context(), "inbox: bad request", slog.String("reason", "key domain does not match actor"), slog.String("key_domain", keyDomain), slog.String("actor_domain", actorDomain))
		err := api.NewBadRequestError("key domain does not match actor")
		api.HandleError(w, r, err)
		return
	}
	slog.DebugContext(r.Context(), "inbox: received activity",
		slog.String("type", activity.Type), slog.String("id", activity.ID), slog.String("actor", activity.Actor))
	err = h.inbox.Process(r.Context(), &activity)
	if err != nil {
		if errors.Is(err, ap.ErrFatal) {
			// Since ErrFatal is not a transient error, log the error at warn level and return 202 Accepted.
			slog.WarnContext(r.Context(), "fatal inbox error", slog.Any("error", err.Error()), slog.String("activity_id", activity.ID))
		} else {
			api.HandleError(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusAccepted)
}
