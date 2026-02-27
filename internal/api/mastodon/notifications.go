package mastodon

import (
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
)

// NotificationsHandler handles notification Mastodon API endpoints.
type NotificationsHandler struct {
	deps Deps
}

// NewNotificationsHandler returns a new NotificationsHandler.
func NewNotificationsHandler(deps Deps) *NotificationsHandler {
	return &NotificationsHandler{deps: deps}
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
	list, err := h.deps.Notifications.List(r.Context(), account.ID, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Notification, 0, len(list))
	for i := range list {
		n := &list[i]
		fromAcc, _ := h.deps.Accounts.GetByID(r.Context(), n.FromID)
		out = append(out, apimodel.ToNotification(n, fromAcc, nil, h.deps.InstanceDomain))
	}
	firstID, lastID := firstLastNotificationIDs(list)
	if link := LinkHeader(r.URL.String(), firstID, lastID); link != "" {
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
