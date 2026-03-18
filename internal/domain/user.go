package domain

import "time"

// User is the local authentication and preferences record linked to an Account.
type User struct {
	ID                 string
	AccountID          string
	Email              string
	PasswordHash       string
	ConfirmedAt        *time.Time
	Role               string
	RegistrationReason *string
	DefaultPrivacy     string
	DefaultSensitive   bool
	DefaultLanguage    string
	DefaultQuotePolicy string // public | followers | nobody
	CreatedAt          time.Time
}

const (
	RoleUser      = "user"
	RoleModerator = "moderator"
	RoleAdmin     = "admin"
)
