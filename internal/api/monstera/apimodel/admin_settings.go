package apimodel

import (
	"fmt"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/domain"
)

var allowedRegistrationModes = []string{"open", "approval", "invite", "closed"}

// AdminSettings is the request/response for GET/PUT /admin/settings (Monstera settings).
type AdminSettings struct {
	RegistrationMode string `json:"registration_mode"`
}

func (a AdminSettings) Validate() error {
	if err := api.ValidateOneOf(a.RegistrationMode, allowedRegistrationModes, "registration_mode"); err != nil {
		return fmt.Errorf("registration_mode: %w", err)
	}
	return nil
}

func (a AdminSettings) ToDomain() domain.MonsteraSettings {
	return domain.MonsteraSettings{
		RegistrationMode: domain.MonsteraRegistrationMode(a.RegistrationMode),
	}
}

func AdminSettingsFromDomain(m domain.MonsteraSettings) AdminSettings {
	return AdminSettings{
		RegistrationMode: string(m.RegistrationMode),
	}
}
