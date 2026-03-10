package domain

import "fmt"

type MonsteraRegistrationMode string

const (
	MonsteraRegistrationModeOpen     MonsteraRegistrationMode = "open"
	MonsteraRegistrationModeApproval MonsteraRegistrationMode = "approval"
	MonsteraRegistrationModeInvite   MonsteraRegistrationMode = "invite"
	MonsteraRegistrationModeClosed   MonsteraRegistrationMode = "closed"
)

type MonsteraSettings struct {
	RegistrationMode    MonsteraRegistrationMode `json:"registration_mode"`
	InviteMaxUses       *int                     `json:"invite_max_uses,omitempty"`
	InviteExpiresInDays *int                     `json:"invite_expires_in_days,omitempty"`
	ServerName          *string                  `json:"server_name,omitempty"`
	ServerDescription   *string                  `json:"server_description,omitempty"`
	ServerRules         []string                 `json:"server_rules,omitempty"`
}

func (m MonsteraSettings) Validate() error {
	switch m.RegistrationMode {
	case MonsteraRegistrationModeOpen, MonsteraRegistrationModeApproval, MonsteraRegistrationModeInvite, MonsteraRegistrationModeClosed:
	default:
		return fmt.Errorf("invalid registration_mode %q: %w", m.RegistrationMode, ErrValidation)
	}
	if m.ServerName != nil && len(*m.ServerName) > 24 {
		return fmt.Errorf("server_name must be 24 characters or fewer: %w", ErrValidation)
	}
	return nil
}
