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
	}
	if fromAccount != nil {
		out.Account = ToAccount(fromAccount, instanceDomain)
	}
	return out
}
