package apimodel

import (
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// Notification is the Mastodon API notification response shape.
type Notification struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	CreatedAt string  `json:"created_at"`
	Account   Account `json:"account"`
	Status    *Status `json:"status,omitempty"`
	GroupKey  string  `json:"group_key"`
}

// ToNotification converts a domain notification with resolved account (and optional status) to the API shape.
func ToNotification(n *domain.Notification, fromAccount *domain.Account, status *Status, instanceDomain string) Notification {
	createdAt := ""
	if n != nil {
		createdAt = n.CreatedAt.UTC().Format(time.RFC3339)
	}
	out := Notification{
		ID:        "",
		Type:      "",
		CreatedAt: createdAt,
		Account:   Account{},
		Status:    status,
	}
	if n != nil {
		out.ID = n.ID
		out.Type = n.Type
		out.GroupKey = n.GroupKey
		if out.GroupKey == "" {
			out.GroupKey = "ungrouped-" + n.ID
		}
	}
	if fromAccount != nil {
		out.Account = ToAccount(fromAccount, instanceDomain)
	}
	return out
}

// NotificationGroupJSON is the Mastodon v2 grouped notification response shape.
type NotificationGroupJSON struct {
	GroupKey                 string   `json:"group_key"`
	NotificationsCount       int      `json:"notifications_count"`
	Type                     string   `json:"type"`
	MostRecentNotificationID string   `json:"most_recent_notification_id"`
	PageMinID                string   `json:"page_min_id"`
	PageMaxID                string   `json:"page_max_id"`
	LatestPageNotificationAt string   `json:"latest_page_notification_at"`
	SampleAccountIDs         []string `json:"sample_account_ids"`
	StatusID                 *string  `json:"status_id"`
}

// GroupedNotificationsResponse is the envelope for GET /api/v2/notifications.
type GroupedNotificationsResponse struct {
	Accounts           []Account               `json:"accounts"`
	Statuses           []Status                `json:"statuses"`
	NotificationGroups []NotificationGroupJSON `json:"notification_groups"`
}

// ToNotificationGroupJSON converts a domain NotificationGroup to the v2 API shape.
func ToNotificationGroupJSON(g *domain.NotificationGroup) NotificationGroupJSON {
	return NotificationGroupJSON{
		GroupKey:                 g.GroupKey,
		NotificationsCount:       g.NotificationsCount,
		Type:                     g.Type,
		MostRecentNotificationID: g.MostRecentNotificationID,
		PageMinID:                g.PageMinID,
		PageMaxID:                g.PageMaxID,
		LatestPageNotificationAt: g.LatestPageNotificationAt.UTC().Format(time.RFC3339),
		SampleAccountIDs:         g.SampleAccountIDs,
		StatusID:                 g.StatusID,
	}
}

// UnreadCountResponse is the response for GET /api/v2/notifications/unread_count.
type UnreadCountResponse struct {
	Count int64 `json:"count"`
}
