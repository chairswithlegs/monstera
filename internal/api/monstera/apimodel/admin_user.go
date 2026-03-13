package apimodel

import (
	"fmt"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/domain"
)

// AdminUser is a single user in the admin API.
type AdminUser struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Username  string `json:"username"`
	Suspended bool   `json:"suspended"`
	Silenced  bool   `json:"silenced"`
}

// AdminUserFromDomain builds an AdminUser from a domain user and account-derived fields.
func AdminUserFromDomain(u *domain.User, username string, suspended, silenced bool) AdminUser {
	return AdminUser{
		ID:        u.ID,
		AccountID: u.AccountID,
		Email:     u.Email,
		Role:      u.Role,
		Username:  username,
		Suspended: suspended,
		Silenced:  silenced,
	}
}

// AdminUserList is the response for GET /admin/users.
type AdminUserList struct {
	Users []AdminUser `json:"users"`
}

type PutRoleRequest struct {
	Role string `json:"role"`
}

func (r *PutRoleRequest) Validate() error {
	if err := api.ValidateOneOf(r.Role, []string{domain.RoleUser, domain.RoleModerator, domain.RoleAdmin}, "role"); err != nil {
		return fmt.Errorf("role: %w", err)
	}
	return nil
}
