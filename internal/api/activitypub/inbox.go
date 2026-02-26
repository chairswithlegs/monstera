package activitypub

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/ap"
	"github.com/chairswithlegs/monstera-fed/internal/api"
)

// InboxHandler handles POST /users/{username}/inbox and POST /inbox.
// Verifies HTTP Signature, parses the activity, and dispatches to InboxProcessor.
// Always returns 202 Accepted per ActivityPub spec.
type InboxHandler struct {
	deps Deps
}

// NewInboxHandler returns a new InboxHandler.
func NewInboxHandler(deps Deps) *InboxHandler {
	return &InboxHandler{deps: deps}
}

// POSTInbox handles POST to the inbox.
func (h *InboxHandler) POSTInbox(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "application/activity+json") && !strings.Contains(ct, "application/ld+json") {
		api.WriteError(w, http.StatusUnsupportedMediaType, "content type must be application/activity+json")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.deps.Logger.Warn("inbox: read body", slog.Any("error", err))
		api.WriteError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	// When Inbox or Cache is nil (tests or federation intentionally disabled), accept without processing
	// so the sender gets a valid 202 and we do not fail the request.
	if h.deps.Inbox == nil || h.deps.Cache == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	keyID, err := ap.Verify(r.Context(), r, ap.DefaultKeyFetcher, h.deps.Cache)
	if err != nil {
		h.deps.Logger.Debug("inbox: signature verification failed", slog.Any("error", err))
		api.WriteError(w, http.StatusUnauthorized, "signature verification failed")
		return
	}
	var activity ap.Activity
	if err := json.Unmarshal(body, &activity); err != nil {
		h.deps.Logger.Warn("inbox: parse activity", slog.Any("error", err))
		api.WriteError(w, http.StatusBadRequest, "invalid activity json")
		return
	}
	// Enforce that the key used to sign belongs to the same domain as the activity actor.
	// Compliant activities have an actor IRI and key IDs are IRIs; both must parse to a host.
	keyDomain := ap.DomainFromKeyID(keyID)
	actorDomain := ap.DomainFromActorID(activity.Actor)
	if keyDomain == "" || actorDomain == "" {
		h.deps.Logger.Debug("inbox: cannot verify key attribution", slog.String("keyDomain", keyDomain), slog.String("actorDomain", actorDomain))
		api.WriteError(w, http.StatusUnauthorized, "cannot verify key attribution")
		return
	}
	if keyDomain != actorDomain {
		h.deps.Logger.Debug("inbox: key attribution mismatch", slog.String("keyDomain", keyDomain), slog.String("actorDomain", actorDomain))
		api.WriteError(w, http.StatusUnauthorized, "key domain does not match actor")
		return
	}
	err = h.deps.Inbox.Process(r.Context(), &activity)
	if err != nil {
		var perm *ap.PermanentError
		if errors.As(err, &perm) {
			h.deps.Logger.Warn("inbox: permanent error", slog.Any("error", perm.Err), slog.String("activity_id", activity.ID))
		} else {
			api.HandleError(w, r, h.deps.Logger, err)
			return
		}
	}
	w.WriteHeader(http.StatusAccepted)
}
