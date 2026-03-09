package apimodel

import (
	"encoding/json"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

const placeholderAvatar = ""
const placeholderHeader = ""

// ToAccount converts a domain account to the Mastodon API account shape.
func ToAccount(a *domain.Account, instanceDomain string) Account {
	acct := a.Username
	if a.Domain != nil && *a.Domain != "" {
		acct = a.Username + "@" + *a.Domain
	}
	urlStr := a.APID
	if a.Domain == nil || *a.Domain == "" {
		urlStr = "https://" + instanceDomain + "/@" + a.Username
	}
	note := ""
	if a.Note != nil {
		note = *a.Note
	}
	displayName := ""
	if a.DisplayName != nil {
		displayName = *a.DisplayName
	}
	acc := Account{
		ID:             a.ID,
		Username:       a.Username,
		Acct:           acct,
		DisplayName:    displayName,
		Locked:         a.Locked,
		Bot:            a.Bot,
		CreatedAt:      a.CreatedAt.UTC().Format(time.RFC3339),
		Note:           note,
		URL:            urlStr,
		Avatar:         placeholderAvatar,
		AvatarStatic:   placeholderAvatar,
		Header:         placeholderHeader,
		HeaderStatic:   placeholderHeader,
		FollowersCount: a.FollowersCount,
		FollowingCount: a.FollowingCount,
		StatusesCount:  a.StatusesCount,
		Emojis:         []any{},
		Fields:         parseFields(a.Fields),
	}
	return acc
}

// ToAccountWithSource converts a domain account and user to the Mastodon API account shape with source.
func ToAccountWithSource(a *domain.Account, u *domain.User, instanceDomain string) Account {
	acc := ToAccount(a, instanceDomain)
	if u != nil {
		acc.Source = &Source{
			Note:        acc.Note,
			Privacy:     u.DefaultPrivacy,
			Sensitive:   u.DefaultSensitive,
			Language:    u.DefaultLanguage,
			QuotePolicy: u.DefaultQuotePolicy,
			Fields:      parseFieldsRaw(a.Fields),
		}
	}
	return acc
}

func parseFields(raw json.RawMessage) []Field {
	if len(raw) == 0 {
		return nil
	}
	var decoded []struct {
		Name       string  `json:"name"`
		Value      string  `json:"value"`
		VerifiedAt *string `json:"verified_at"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	out := make([]Field, 0, len(decoded))
	for _, f := range decoded {
		out = append(out, Field{
			Name:       f.Name,
			Value:      f.Value,
			VerifiedAt: f.VerifiedAt,
		})
	}
	return out
}

func parseFieldsRaw(raw json.RawMessage) []Field {
	return parseFields(raw)
}
