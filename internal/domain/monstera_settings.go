package domain

// MonsteraRegistrationMode controls who may register (open, approval, invite, closed).
type MonsteraRegistrationMode string

const (
	MonsteraRegistrationModeOpen     MonsteraRegistrationMode = "open"
	MonsteraRegistrationModeApproval MonsteraRegistrationMode = "approval"
	MonsteraRegistrationModeInvite   MonsteraRegistrationMode = "invite"
	MonsteraRegistrationModeClosed   MonsteraRegistrationMode = "closed"
)

// MonsteraSettings holds instance configuration (registration, server name, rules).
type MonsteraSettings struct {
	RegistrationMode    MonsteraRegistrationMode `json:"registration_mode"`
	InviteMaxUses       *int                     `json:"invite_max_uses,omitempty"`
	InviteExpiresInDays *int                     `json:"invite_expires_in_days,omitempty"`
	ServerName          *string                  `json:"server_name,omitempty"`
	ServerDescription   *string                  `json:"server_description,omitempty"`
	ServerRules         []string                 `json:"server_rules,omitempty"`
}
