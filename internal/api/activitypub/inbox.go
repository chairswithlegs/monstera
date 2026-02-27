package activitypub

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	ap "github.com/chairswithlegs/monstera-fed/internal/activitypub"
	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/cache"
)

// InboxHandler handles POST /users/{username}/inbox and POST /inbox.
// Verifies HTTP Signature, parses the activity, and dispatches to InboxProcessor.
// Always returns 202 Accepted per ActivityPub spec.
type InboxHandler struct {
	inbox *ap.InboxProcessor
	cache cache.Store
}

// NewInboxHandler returns a new InboxHandler.
func NewInboxHandler(inbox *ap.InboxProcessor, cache cache.Store) *InboxHandler {
	return &InboxHandler{inbox: inbox, cache: cache}
}

// POSTInbox handles POST to the inbox.
func (h *InboxHandler) POSTInbox(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "application/activity+json") && !strings.Contains(ct, "application/ld+json") {
		err := api.NewBadRequestError("content type must be application/activity+json or application/ld+json")
		api.HandleError(w, r, err)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		err := api.NewBadRequestError("failed to read body")
		api.HandleError(w, r, err)
		return
	}
	// When Inbox or Cache is nil (tests or federation intentionally disabled), accept without processing
	// so the sender gets a valid 202 and we do not fail the request.
	if h.inbox == nil || h.cache == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	keyID, err := ap.Verify(r.Context(), r, ap.DefaultKeyFetcher, h.cache)
	if err != nil {
		err := api.NewBadRequestError("signature verification failed")
		api.HandleError(w, r, err)
		return
	}
	var activity ap.Activity
	if err := json.Unmarshal(body, &activity); err != nil {
		err := api.NewBadRequestError("invalid activity json")
		api.HandleError(w, r, err)
		return
	}
	// Enforce that the key used to sign belongs to the same domain as the activity actor.
	// Compliant activities have an actor IRI and key IDs are IRIs; both must parse to a host.
	keyDomain := ap.DomainFromKeyID(keyID)
	actorDomain := ap.DomainFromActorID(activity.Actor)
	if keyDomain == "" || actorDomain == "" {
		err := api.NewBadRequestError("cannot verify key attribution")
		api.HandleError(w, r, err)
		return
	}
	if keyDomain != actorDomain {
		err := api.NewBadRequestError("key domain does not match actor")
		api.HandleError(w, r, err)
		return
	}
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
