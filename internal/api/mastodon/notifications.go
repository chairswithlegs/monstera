package mastodon

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// NotificationsHandler handles notification Mastodon API endpoints.
type NotificationsHandler struct {
	notifications  service.NotificationService
	accounts       service.AccountService
	statuses       service.StatusService
	instanceDomain string
}

// NewNotificationsHandler returns a new NotificationsHandler.
func NewNotificationsHandler(notifications service.NotificationService, accounts service.AccountService, statuses service.StatusService, instanceDomain string) *NotificationsHandler {
	return &NotificationsHandler{notifications: notifications, accounts: accounts, statuses: statuses, instanceDomain: instanceDomain}
}

func notificationStatusType(t string) bool {
	switch t {
	case domain.NotificationTypeMention, domain.NotificationTypeReblog, domain.NotificationTypeFavourite,
		domain.NotificationTypeQuote, domain.NotificationTypeQuotedUpdate, "update", "poll":
		return true
	default:
		return false
	}
}

// GETNotifications handles GET /api/v1/notifications.
func (h *NotificationsHandler) GETNotifications(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	params := PageParamsFromRequest(r)
	maxID := optionalString(params.MaxID)
	list, err := h.notifications.List(r.Context(), account.ID, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Notification, 0, len(list))
	for i := range list {
		n := &list[i]
		fromAcc, _ := h.accounts.GetByID(r.Context(), n.FromID)
		var statusAPI *apimodel.Status
		if h.statuses != nil && n.StatusID != nil && *n.StatusID != "" && notificationStatusType(n.Type) {
			if enriched, err := h.statuses.GetByIDEnriched(r.Context(), *n.StatusID, &account.ID); err == nil {
				s := apimodel.StatusFromEnriched(enriched, h.instanceDomain)
				statusAPI = &s
			}
		}
		out = append(out, apimodel.ToNotification(n, fromAcc, statusAPI, h.instanceDomain))
	}
	firstID, lastID := firstLastNotificationIDs(list)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

func firstLastNotificationIDs(list []domain.Notification) (firstID, lastID string) {
	if len(list) == 0 {
		return "", ""
	}
	return list[0].ID, list[len(list)-1].ID
}

// GETNotification handles GET /api/v1/notifications/:id.
func (h *NotificationsHandler) GETNotification(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	n, err := h.notifications.Get(r.Context(), id, account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	fromAcc, _ := h.accounts.GetByID(r.Context(), n.FromID)
	var statusAPI *apimodel.Status
	if h.statuses != nil && n.StatusID != nil && *n.StatusID != "" && notificationStatusType(n.Type) {
		if enriched, err := h.statuses.GetByIDEnriched(r.Context(), *n.StatusID, &account.ID); err == nil {
			s := apimodel.StatusFromEnriched(enriched, h.instanceDomain)
			statusAPI = &s
		}
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToNotification(n, fromAcc, statusAPI, h.instanceDomain))
}

// POSTClear handles POST /api/v1/notifications/clear.
func (h *NotificationsHandler) POSTClear(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	if err := h.notifications.Clear(r.Context(), account.ID); err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{})
}

// POSTDismiss handles POST /api/v1/notifications/:id/dismiss.
func (h *NotificationsHandler) POSTDismiss(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	if err := h.notifications.Dismiss(r.Context(), id, account.ID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{})
}
