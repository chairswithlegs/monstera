package activitypub

import (
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	ap "github.com/chairswithlegs/monstera-fed/internal/activitypub"
	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

const defaultOutboxPageSize = 20
const maxOutboxPageSize = 40

// OutboxHandler serves GET /users/{username}/outbox — paginated OrderedCollectionPage of Create{Note} activities.
type OutboxHandler struct {
	accounts  *service.AccountService
	timelines *service.TimelineService
	config    *config.Config
}

// NewOutbox returns a new OutboxHandler.
func NewOutbox(accounts *service.AccountService, timelines *service.TimelineService, config *config.Config) *OutboxHandler {
	return &OutboxHandler{accounts: accounts, timelines: timelines, config: config}
}

// GETOutbox serves the outbox collection or a page.
func (h *OutboxHandler) GETOutbox(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := api.ValidateRequiredString(username); err != nil {
		api.HandleError(w, r, err)
		return
	}
	account, err := h.accounts.GetLocalActorForFederation(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	base := "https://" + h.config.InstanceDomain
	outboxID := base + "/users/" + username + "/outbox"

	page := r.URL.Query().Get("page")
	if page == "" {
		total, err := h.timelines.CountAccountPublicStatuses(r.Context(), account.ID)
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		coll := ap.OrderedCollection{
			Context:    ap.DefaultContext,
			ID:         outboxID,
			Type:       "OrderedCollection",
			TotalItems: int(total),
			First:      outboxID + "?page=true",
		}
		w.Header().Set("Cache-Control", "max-age=60")
		w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
		api.WriteJSON(w, http.StatusOK, coll)
		return
	}

	maxID := r.URL.Query().Get("max_id")
	limit := defaultOutboxPageSize
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			if n > maxOutboxPageSize {
				n = maxOutboxPageSize
			}
			limit = n
		}
	}

	var maxIDPtr *string
	if maxID != "" {
		maxIDPtr = &maxID
	}
	statuses, err := h.timelines.GetAccountPublicStatuses(r.Context(), account.ID, maxIDPtr, limit+1)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	hasMore := len(statuses) > limit
	var publicStatuses []domain.Status
	if len(statuses) > limit {
		publicStatuses = statuses[:limit]
	} else {
		publicStatuses = statuses
	}
	actorID := account.APID
	if actorID == "" {
		actorID = fmt.Sprintf("%s/users/%s", base, account.Username)
	}
	var orderedItems []json.RawMessage
	for i := range publicStatuses {
		note := statusToNote(&publicStatuses[i], actorID, base)
		activityID := publicStatuses[i].APID
		if activityID == "" {
			activityID = publicStatuses[i].URI
		}
		if activityID == "" {
			activityID = fmt.Sprintf("%s/statuses/%s", base, publicStatuses[i].ID)
		}
		create, err := ap.WrapInCreate(activityID, note)
		if err != nil {
			slog.WarnContext(r.Context(), "outbox: wrap create failed", slog.String("status_id", publicStatuses[i].ID), slog.Any("error", err))
			continue
		}
		raw, err := json.Marshal(create)
		if err != nil {
			slog.WarnContext(r.Context(), "outbox: marshal create failed", slog.String("status_id", publicStatuses[i].ID), slog.Any("error", err))
			continue
		}
		orderedItems = append(orderedItems, raw)
	}
	pageID := outboxID + "?page=true"
	if maxID != "" {
		pageID = outboxID + "?page=true&max_id=" + url.QueryEscape(maxID)
	}
	pageResp := ap.OrderedCollectionPage{
		Context:      ap.DefaultContext,
		ID:           pageID,
		Type:         "OrderedCollectionPage",
		TotalItems:   len(orderedItems),
		PartOf:       outboxID,
		OrderedItems: orderedItems,
	}
	if hasMore && len(publicStatuses) > 0 {
		pageResp.Next = outboxID + "?page=true&max_id=" + url.QueryEscape(publicStatuses[len(publicStatuses)-1].ID)
	}
	w.Header().Set("Cache-Control", "max-age=60")
	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
	api.WriteJSON(w, http.StatusOK, pageResp)
}

func statusToNote(s *domain.Status, actorID, base string) *ap.Note {
	content := ""
	if s.Content != nil && *s.Content != "" {
		content = *s.Content
	} else if s.Text != nil && *s.Text != "" {
		content = html.EscapeString(*s.Text)
	}
	noteID := s.APID
	if noteID == "" {
		noteID = s.URI
	}
	if noteID == "" {
		noteID = fmt.Sprintf("%s/statuses/%s", base, s.ID)
	}
	inReplyTo := s.InReplyToID
	var inReplyToIRI *string
	if inReplyTo != nil && *inReplyTo != "" {
		iri := fmt.Sprintf("%s/statuses/%s", base, *inReplyTo)
		inReplyToIRI = &iri
	}
	published := s.CreatedAt.Format(time.RFC3339)
	updated := ""
	if s.EditedAt != nil {
		updated = s.EditedAt.Format(time.RFC3339)
	}
	return &ap.Note{
		Context:      ap.DefaultContext,
		ID:           noteID,
		Type:         "Note",
		AttributedTo: actorID,
		Content:      content,
		To:           []string{ap.PublicAddress},
		InReplyTo:    inReplyToIRI,
		Published:    published,
		Updated:      updated,
		URL:          noteID,
		Sensitive:    s.Sensitive,
		Summary:      s.ContentWarning,
	}
}
