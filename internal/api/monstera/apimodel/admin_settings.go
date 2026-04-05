package apimodel

import (
	"fmt"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/domain"
)

var allowedRegistrationModes = []string{"open", "approval", "invite", "closed"}
var allowedTrendingScopes = []string{"disabled", "local", "all"}

// AdminSettings is the request/response for GET/PUT /admin/settings (Monstera settings).
type AdminSettings struct {
	RegistrationMode      string   `json:"registration_mode"`
	InviteMaxUses         *int     `json:"invite_max_uses,omitempty"`
	InviteExpiresInDays   *int     `json:"invite_expires_in_days,omitempty"`
	ServerName            *string  `json:"server_name,omitempty"`
	ServerDescription     *string  `json:"server_description,omitempty"`
	ServerRules           []string `json:"server_rules,omitempty"`
	TrendingLinksScope    string   `json:"trending_links_scope"`
	TrendingTagsScope     string   `json:"trending_tags_scope"`
	TrendingStatusesScope string   `json:"trending_statuses_scope"`
}

func (a AdminSettings) Validate() error {
	if err := api.ValidateOneOf(a.RegistrationMode, allowedRegistrationModes, "registration_mode"); err != nil {
		return fmt.Errorf("registration_mode: %w", err)
	}
	if a.ServerName != nil && len(*a.ServerName) > 24 {
		return fmt.Errorf("server_name exceeds 24 characters: %w", domain.ErrValidation)
	}
	for _, f := range []struct {
		value string
		field string
	}{
		{a.TrendingLinksScope, "trending_links_scope"},
		{a.TrendingTagsScope, "trending_tags_scope"},
		{a.TrendingStatusesScope, "trending_statuses_scope"},
	} {
		if f.value != "" {
			if err := api.ValidateOneOf(f.value, allowedTrendingScopes, f.field); err != nil {
				return fmt.Errorf("%s: %w", f.field, err)
			}
		}
	}
	return nil
}

func (a AdminSettings) ToDomain() domain.MonsteraSettings {
	return domain.MonsteraSettings{
		RegistrationMode:      domain.MonsteraRegistrationMode(a.RegistrationMode),
		InviteMaxUses:         a.InviteMaxUses,
		InviteExpiresInDays:   a.InviteExpiresInDays,
		ServerName:            a.ServerName,
		ServerDescription:     a.ServerDescription,
		ServerRules:           a.ServerRules,
		TrendingLinksScope:    domain.MonsteraTrendingScope(a.TrendingLinksScope),
		TrendingTagsScope:     domain.MonsteraTrendingScope(a.TrendingTagsScope),
		TrendingStatusesScope: domain.MonsteraTrendingScope(a.TrendingStatusesScope),
	}
}

func AdminSettingsFromDomain(m domain.MonsteraSettings) AdminSettings {
	return AdminSettings{
		RegistrationMode:      string(m.RegistrationMode),
		InviteMaxUses:         m.InviteMaxUses,
		InviteExpiresInDays:   m.InviteExpiresInDays,
		ServerName:            m.ServerName,
		ServerDescription:     m.ServerDescription,
		ServerRules:           m.ServerRules,
		TrendingLinksScope:    string(m.TrendingLinksScope),
		TrendingTagsScope:     string(m.TrendingTagsScope),
		TrendingStatusesScope: string(m.TrendingStatusesScope),
	}
}
