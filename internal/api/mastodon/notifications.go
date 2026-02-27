package mastodon

import (
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// NotificationsHandler handles notification Mastodon API endpoints.
type NotificationsHandler struct {
	notifications  *service.NotificationService
	accounts       *service.AccountService
	instanceDomain string
}

// NewNotificationsHandler returns a new NotificationsHandler.
func NewNotificationsHandler(notifications *service.NotificationService, accounts *service.AccountService, instanceDomain string) *NotificationsHandler {
	return &NotificationsHandler{notifications: notifications, accounts: accounts, instanceDomain: instanceDomain}
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
		out = append(out, apimodel.ToNotification(n, fromAcc, nil, h.instanceDomain))
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
