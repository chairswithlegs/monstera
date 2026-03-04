package apimodel

import (
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// AdminInvite is one invite in the admin API.
type AdminInvite struct {
	ID        string     `json:"id"`
	Code      string     `json:"code"`
	Uses      int        `json:"uses"`
	MaxUses   *int       `json:"max_uses,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// ToAdminInvite converts a domain invite to the admin API shape.
func ToAdminInvite(inv *domain.Invite) AdminInvite {
	return AdminInvite{
		ID:        inv.ID,
		Code:      inv.Code,
		Uses:      inv.Uses,
		MaxUses:   inv.MaxUses,
		ExpiresAt: inv.ExpiresAt,
		CreatedAt: inv.CreatedAt,
	}
}

// AdminInviteList is the response for GET /admin/invites.
type AdminInviteList struct {
	Invites []AdminInvite `json:"invites"`
}
