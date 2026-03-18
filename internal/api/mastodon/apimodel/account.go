package apimodel

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/microcosm-cc/bluemonday"
)

// ToAccount converts a domain account to the Mastodon API account shape.
func ToAccount(a *domain.Account, instanceDomain string) Account {
	acct := a.Username
	if a.Domain != nil && *a.Domain != "" {
		acct = a.Username + "@" + *a.Domain
	}
	var urlStr string
	switch {
	case a.Domain == nil || *a.Domain == "":
		urlStr = "https://" + instanceDomain + "/@" + a.Username
	case a.ProfileURL != "":
		urlStr = a.ProfileURL
	default:
		urlStr = a.APID
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
		Avatar:         a.AvatarURL,
		AvatarStatic:   a.AvatarURL,
		Header:         a.HeaderURL,
		HeaderStatic:   a.HeaderURL,
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

type PostFollowedTagRequest struct {
	Name string `json:"name"`
}

func (r *PostFollowedTagRequest) Sanitize() {
	r.Name = bluemonday.StrictPolicy().Sanitize(r.Name)
}

func (r *PostFollowedTagRequest) Validate() error {
	if err := api.ValidateRequiredField(r.Name, "name"); err != nil {
		return fmt.Errorf("name: %w", err)
	}
	return nil
}

type RegisterAccountRequest struct {
	Username   string  `json:"username"`
	Email      string  `json:"email"`
	Password   string  `json:"password"`
	Agreement  bool    `json:"agreement"`
	Locale     string  `json:"locale"`
	Reason     *string `json:"reason"`
	InviteCode *string `json:"invite_code"`
}

func (req *RegisterAccountRequest) Sanitize() {
	if req.Reason != nil {
		*req.Reason = bluemonday.StrictPolicy().Sanitize(*req.Reason)
	}
}

func (req RegisterAccountRequest) Validate() error {
	if err := api.ValidateRequiredField(req.Username, "username"); err != nil {
		return fmt.Errorf("username: %w", err)
	}
	if err := api.ValidateRequiredField(req.Email, "email"); err != nil {
		return fmt.Errorf("email: %w", err)
	}
	if err := api.ValidateRequiredField(req.Password, "password"); err != nil {
		return fmt.Errorf("password: %w", err)
	}
	if !req.Agreement {
		return fmt.Errorf("%w", api.NewUnprocessableError("agreement must be accepted"))
	}
	return nil
}

type RegisterAccountResponse struct {
	Account Account `json:"account"`
	Pending bool    `json:"pending"`
}
