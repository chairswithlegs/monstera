package apimodel

import (
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

type User struct {
	ID               string     `json:"id"`
	AccountID        string     `json:"account_id"`
	Email            string     `json:"email"`
	ConfirmedAt      *time.Time `json:"confirmed_at"`
	Role             string     `json:"role"`
	DefaultPrivacy   string     `json:"default_privacy"`
	DefaultSensitive bool       `json:"default_sensitive"`
	DefaultLanguage  string     `json:"default_language"`
	CreatedAt        time.Time  `json:"created_at"`
}

func ToUser(user *domain.User) User {
	return User{
		ID:               user.ID,
		AccountID:        user.AccountID,
		Email:            user.Email,
		ConfirmedAt:      user.ConfirmedAt,
		Role:             user.Role,
		DefaultPrivacy:   user.DefaultPrivacy,
		DefaultSensitive: user.DefaultSensitive,
		DefaultLanguage:  user.DefaultLanguage,
		CreatedAt:        user.CreatedAt,
	}
}
