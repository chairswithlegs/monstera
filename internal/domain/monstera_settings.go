package domain

// MonsteraRegistrationMode controls who may register (open, approval, invite, closed).
type MonsteraRegistrationMode string

const (
	MonsteraRegistrationModeOpen     MonsteraRegistrationMode = "open"
	MonsteraRegistrationModeApproval MonsteraRegistrationMode = "approval"
	MonsteraRegistrationModeInvite   MonsteraRegistrationMode = "invite"
	MonsteraRegistrationModeClosed   MonsteraRegistrationMode = "closed"
)

// MonsteraTrendingScope controls how a trending content type is sourced.
type MonsteraTrendingScope string

const (
	// MonsteraTrendingDisabled disables the trending content type; the index is cleared.
	MonsteraTrendingDisabled MonsteraTrendingScope = "disabled"
	// MonsteraTrendingLocal indexes content only from local users' statuses.
	MonsteraTrendingLocal MonsteraTrendingScope = "local"
	// MonsteraTrendingAll indexes content from both local and remote users' statuses.
	// Note: currently behaves like Local for links until remote card processing is enabled.
	MonsteraTrendingAll MonsteraTrendingScope = "all"
)

// MonsteraSettings holds instance configuration (registration, server name, rules).
type MonsteraSettings struct {
	RegistrationMode      MonsteraRegistrationMode `json:"registration_mode"`
	InviteMaxUses         *int                     `json:"invite_max_uses,omitempty"`
	InviteExpiresInDays   *int                     `json:"invite_expires_in_days,omitempty"`
	ServerName            *string                  `json:"server_name,omitempty"`
	ServerDescription     *string                  `json:"server_description,omitempty"`
	ServerRules           []string                 `json:"server_rules,omitempty"`
	TrendingLinksScope    MonsteraTrendingScope    `json:"trending_links_scope"`
	TrendingTagsScope     MonsteraTrendingScope    `json:"trending_tags_scope"`
	TrendingStatusesScope MonsteraTrendingScope    `json:"trending_statuses_scope"`
}
