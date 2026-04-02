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

// GroupedNotificationsHandler handles the Mastodon v2 grouped notifications API.
type GroupedNotificationsHandler struct {
	notifications  service.NotificationService
	accounts       service.AccountService
	statuses       service.StatusService
	instanceDomain string
}

// NewGroupedNotificationsHandler returns a new GroupedNotificationsHandler.
func NewGroupedNotificationsHandler(notifications service.NotificationService, accounts service.AccountService, statuses service.StatusService, instanceDomain string) *GroupedNotificationsHandler {
	return &GroupedNotificationsHandler{notifications: notifications, accounts: accounts, statuses: statuses, instanceDomain: instanceDomain}
}

// GETGroupedNotifications handles GET /api/v2/notifications.
func (h *GroupedNotificationsHandler) GETGroupedNotifications(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	params := PageParamsFromRequest(r)
	maxID := optionalString(params.MaxID)
	groups, err := h.notifications.ListGrouped(r.Context(), account.ID, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	// Collect unique account IDs and status IDs for the envelope.
	accountIDSet := make(map[string]bool)
	statusIDSet := make(map[string]bool)
	groupsJSON := make([]apimodel.NotificationGroupJSON, 0, len(groups))
	for i := range groups {
		g := &groups[i]
		gj := apimodel.ToNotificationGroupJSON(g)
		groupsJSON = append(groupsJSON, gj)
		for _, id := range g.SampleAccountIDs {
			accountIDSet[id] = true
		}
		if g.StatusID != nil && *g.StatusID != "" && notificationStatusType(g.Type) {
			statusIDSet[*g.StatusID] = true
		}
	}

	// Resolve accounts.
	accounts := make([]apimodel.Account, 0, len(accountIDSet))
	for id := range accountIDSet {
		acc, err := h.accounts.GetByID(r.Context(), id)
		if err != nil {
			continue
		}
		accounts = append(accounts, apimodel.ToAccount(acc, h.instanceDomain))
	}

	// Resolve statuses.
	statuses := make([]apimodel.Status, 0, len(statusIDSet))
	if h.statuses != nil {
		for id := range statusIDSet {
			enriched, err := h.statuses.GetByIDEnriched(r.Context(), id, &account.ID)
			if err != nil {
				continue
			}
			statuses = append(statuses, apimodel.StatusFromEnriched(enriched, h.instanceDomain))
		}
	}

	// Pagination Link header.
	if len(groups) > 0 {
		firstID := groups[0].PageMaxID
		lastID := groups[len(groups)-1].PageMinID
		if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
			w.Header().Set("Link", link)
		}
	}

	api.WriteJSON(w, http.StatusOK, apimodel.GroupedNotificationsResponse{
		Accounts:           accounts,
		Statuses:           statuses,
		NotificationGroups: groupsJSON,
	})
}

// GETNotificationGroup handles GET /api/v2/notifications/{group_key}.
func (h *GroupedNotificationsHandler) GETNotificationGroup(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	groupKey := chi.URLParam(r, "group_key")
	if groupKey == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	notifs, err := h.notifications.GetGroup(r.Context(), account.ID, groupKey)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}

	// Build the envelope response from the group's notifications.
	accountIDSet := make(map[string]bool)
	statusIDSet := make(map[string]bool)
	sampleAccountIDs := make([]string, 0)
	var statusID *string
	var groupType string
	for i, n := range notifs {
		if i == 0 {
			groupType = n.Type
			statusID = n.StatusID
		}
		if !accountIDSet[n.FromID] {
			accountIDSet[n.FromID] = true
			sampleAccountIDs = append(sampleAccountIDs, n.FromID)
		}
		if n.StatusID != nil && *n.StatusID != "" && notificationStatusType(n.Type) {
			statusIDSet[*n.StatusID] = true
		}
	}

	gj := apimodel.NotificationGroupJSON{
		GroupKey:                 groupKey,
		NotificationsCount:       len(notifs),
		Type:                     groupType,
		MostRecentNotificationID: notifs[0].ID,
		PageMinID:                notifs[len(notifs)-1].ID,
		PageMaxID:                notifs[0].ID,
		LatestPageNotificationAt: notifs[0].CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		SampleAccountIDs:         sampleAccountIDs,
		StatusID:                 statusID,
	}

	accounts := make([]apimodel.Account, 0, len(accountIDSet))
	for id := range accountIDSet {
		acc, err := h.accounts.GetByID(r.Context(), id)
		if err != nil {
			continue
		}
		accounts = append(accounts, apimodel.ToAccount(acc, h.instanceDomain))
	}

	statuses := make([]apimodel.Status, 0, len(statusIDSet))
	if h.statuses != nil {
		for id := range statusIDSet {
			enriched, err := h.statuses.GetByIDEnriched(r.Context(), id, &account.ID)
			if err != nil {
				continue
			}
			statuses = append(statuses, apimodel.StatusFromEnriched(enriched, h.instanceDomain))
		}
	}

	api.WriteJSON(w, http.StatusOK, apimodel.GroupedNotificationsResponse{
		Accounts:           accounts,
		Statuses:           statuses,
		NotificationGroups: []apimodel.NotificationGroupJSON{gj},
	})
}

// POSTDismissNotificationGroup handles POST /api/v2/notifications/{group_key}/dismiss.
func (h *GroupedNotificationsHandler) POSTDismissNotificationGroup(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	groupKey := chi.URLParam(r, "group_key")
	if groupKey == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	if err := h.notifications.DismissGroup(r.Context(), account.ID, groupKey); err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{})
}

// GETUnreadCount handles GET /api/v2/notifications/unread_count.
func (h *GroupedNotificationsHandler) GETUnreadCount(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	count, err := h.notifications.CountUnreadGroups(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.UnreadCountResponse{Count: count})
}
