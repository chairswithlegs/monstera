package apimodel

import (
	"encoding/json"
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

type User struct {
	ID                 string          `json:"id"`
	AccountID          string          `json:"account_id"`
	Username           string          `json:"username"`
	Email              string          `json:"email"`
	ConfirmedAt        *time.Time      `json:"confirmed_at"`
	Role               string          `json:"role"`
	DefaultPrivacy     string          `json:"default_privacy"`
	DefaultSensitive   bool            `json:"default_sensitive"`
	DefaultLanguage    string          `json:"default_language"`
	DefaultQuotePolicy string          `json:"default_quote_policy"`
	CreatedAt          time.Time       `json:"created_at"`
	DisplayName        *string         `json:"display_name"`
	Note               *string         `json:"note"`
	Locked             bool            `json:"locked"`
	Bot                bool            `json:"bot"`
	Fields             json.RawMessage `json:"fields"`
}

func ToUser(u *domain.User, a *domain.Account) User {
	return User{
		ID:                 u.ID,
		AccountID:          u.AccountID,
		Username:           a.Username,
		Email:              u.Email,
		ConfirmedAt:        u.ConfirmedAt,
		Role:               u.Role,
		DefaultPrivacy:     u.DefaultPrivacy,
		DefaultSensitive:   u.DefaultSensitive,
		DefaultLanguage:    u.DefaultLanguage,
		DefaultQuotePolicy: u.DefaultQuotePolicy,
		CreatedAt:          u.CreatedAt,
		DisplayName:        a.DisplayName,
		Note:               a.Note,
		Locked:             a.Locked,
		Bot:                a.Bot,
		Fields:             a.Fields,
	}
}
