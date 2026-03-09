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
	RegistrationMode MonsteraRegistrationMode `json:"registration_mode"`
}

func (m MonsteraSettings) Validate() error {
	switch m.RegistrationMode {
	case MonsteraRegistrationModeOpen, MonsteraRegistrationModeApproval, MonsteraRegistrationModeInvite, MonsteraRegistrationModeClosed:
	default:
		return fmt.Errorf("invalid registration_mode %q: %w", m.RegistrationMode, ErrValidation)
	}
	return nil
}
