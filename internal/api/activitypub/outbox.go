package activitypub

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/activitypub/vocab"
	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/config"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

const defaultOutboxPageSize = 20
const maxOutboxPageSize = 40

// OutboxHandler serves GET /users/{username}/outbox — paginated OrderedCollectionPage of Create{Note} activities.
type OutboxHandler struct {
	accounts  service.AccountService
	timelines service.TimelineService
	config    *config.Config
}

// NewOutbox returns a new OutboxHandler.
func NewOutbox(accounts service.AccountService, timelines service.TimelineService, config *config.Config) *OutboxHandler {
	return &OutboxHandler{accounts: accounts, timelines: timelines, config: config}
}

// GETOutbox serves the outbox collection or a page.
func (h *OutboxHandler) GETOutbox(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := api.ValidateRequiredField(username, "username"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	account, err := h.accounts.GetActiveLocalAccount(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	base := h.config.InstanceBaseURL()
	outboxID := base + "/users/" + username + "/outbox"

	page := r.URL.Query().Get("page")
	if page == "" {
		total, err := h.timelines.CountAccountPublicStatuses(r.Context(), account.ID)
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		coll := vocab.NewOrderedCollection(outboxID, int(total))
		coll.First = outboxID + "?page=true"
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
	var orderedItems []json.RawMessage
	for i := range publicStatuses {
		note := vocab.StatusToNote(&publicStatuses[i], account, base)
		activityID := vocab.StatusNoteID(&publicStatuses[i], base)
		create, err := vocab.NewCreateNoteActivity(activityID, note)
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
	pageResp := vocab.NewOrderedCollectionPage(pageID, outboxID, orderedItems)
	if hasMore && len(publicStatuses) > 0 {
		pageResp.Next = outboxID + "?page=true&max_id=" + url.QueryEscape(publicStatuses[len(publicStatuses)-1].ID)
	}
	w.Header().Set("Cache-Control", "max-age=60")
	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
	api.WriteJSON(w, http.StatusOK, pageResp)
}
