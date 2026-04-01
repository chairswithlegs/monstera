package apimodel

import (
	"strconv"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// NotificationPolicySummary holds counts of pending notification requests and notifications.
type NotificationPolicySummary struct {
	PendingRequestsCount      int64 `json:"pending_requests_count"`
	PendingNotificationsCount int64 `json:"pending_notifications_count"`
}

// NotificationPolicyResponse is the Mastodon API notification policy response shape.
type NotificationPolicyResponse struct {
	FilterNotFollowing    domain.NotificationFilterPolicy `json:"filter_not_following"`
	FilterNotFollowers    domain.NotificationFilterPolicy `json:"filter_not_followers"`
	FilterNewAccounts     domain.NotificationFilterPolicy `json:"filter_new_accounts"`
	FilterPrivateMentions domain.NotificationFilterPolicy `json:"filter_private_mentions"`
	ForLimitedAccounts    domain.NotificationFilterPolicy `json:"for_limited_accounts"`
	Summary               NotificationPolicySummary       `json:"summary"`
}

// ToNotificationPolicyResponse converts a domain policy + summary counts to the API shape.
func ToNotificationPolicyResponse(p *domain.NotificationPolicy, pendingRequests, pendingNotifications int64) NotificationPolicyResponse {
	return NotificationPolicyResponse{
		FilterNotFollowing:    p.FilterNotFollowing,
		FilterNotFollowers:    p.FilterNotFollowers,
		FilterNewAccounts:     p.FilterNewAccounts,
		FilterPrivateMentions: p.FilterPrivateMentions,
		ForLimitedAccounts:    p.ForLimitedAccounts,
		Summary: NotificationPolicySummary{
			PendingRequestsCount:      pendingRequests,
			PendingNotificationsCount: pendingNotifications,
		},
	}
}

// NotificationRequestResponse is the Mastodon API notification request response shape.
// notifications_count is serialized as a string per the Mastodon API spec.
type NotificationRequestResponse struct {
	ID                 string  `json:"id"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	NotificationsCount string  `json:"notifications_count"`
	Account            Account `json:"account"`
	LastStatus         *Status `json:"last_status"`
}

// ToNotificationRequestResponse converts a domain NotificationRequest with resolved account
// and optional status to the API shape.
func ToNotificationRequestResponse(r *domain.NotificationRequest, fromAccount *domain.Account, lastStatus *Status, instanceDomain string) NotificationRequestResponse {
	out := NotificationRequestResponse{
		ID:                 r.ID,
		CreatedAt:          r.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          r.UpdatedAt.UTC().Format(time.RFC3339),
		NotificationsCount: strconv.FormatInt(int64(r.NotificationsCount), 10),
		LastStatus:         lastStatus,
	}
	if fromAccount != nil {
		out.Account = ToAccount(fromAccount, instanceDomain)
	}
	return out
}
