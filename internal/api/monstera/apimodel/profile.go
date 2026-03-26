package apimodel

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/microcosm-cc/bluemonday"
)

// bcp47Re matches valid BCP-47 language tags (e.g. "en", "en-US", "zh-Hans-CN").
// Empty string is also accepted (clears the preference).
var bcp47Re = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})*$`)

type PatchProfileRequest struct {
	DisplayName        *string         `json:"display_name"`
	Note               *string         `json:"note"`
	Locked             bool            `json:"locked"`
	Bot                bool            `json:"bot"`
	Fields             json.RawMessage `json:"fields"`
	DefaultQuotePolicy *string         `json:"default_quote_policy"`
}

func (b *PatchProfileRequest) Sanitize() {
	if b.DisplayName != nil {
		*b.DisplayName = bluemonday.StrictPolicy().Sanitize(*b.DisplayName)
	}
	if b.Note != nil {
		*b.Note = bluemonday.UGCPolicy().Sanitize(*b.Note)
	}
}

func (b *PatchProfileRequest) Validate() error { return nil }

type PatchPreferencesRequest struct {
	DefaultPrivacy     string `json:"default_privacy"`
	DefaultSensitive   bool   `json:"default_sensitive"`
	DefaultLanguage    string `json:"default_language"`
	DefaultQuotePolicy string `json:"default_quote_policy"`
}

func (b *PatchPreferencesRequest) Sanitize() {
	b.DefaultPrivacy = bluemonday.StrictPolicy().Sanitize(b.DefaultPrivacy)
	b.DefaultLanguage = bluemonday.StrictPolicy().Sanitize(b.DefaultLanguage)
}

func (b *PatchPreferencesRequest) Validate() error {
	if err := api.ValidateOneOf(b.DefaultPrivacy, []string{"public", "unlisted", "private", "direct"}, "default_privacy"); err != nil {
		return fmt.Errorf("default_privacy: %w", err)
	}
	if err := api.ValidateOneOf(b.DefaultQuotePolicy, []string{"public", "followers", "nobody"}, "default_quote_policy"); err != nil {
		return fmt.Errorf("default_quote_policy: %w", err)
	}
	if b.DefaultLanguage != "" && !bcp47Re.MatchString(b.DefaultLanguage) {
		return fmt.Errorf("default_language: %w", api.NewInvalidValueError("default_language"))
	}
	return nil
}

type PatchEmailRequest struct {
	Email string `json:"email"`
}

func (b *PatchEmailRequest) Sanitize() {
	b.Email = bluemonday.StrictPolicy().Sanitize(b.Email)
}

func (b *PatchEmailRequest) Validate() error {
	if err := api.ValidateRequiredField(b.Email, "email"); err != nil {
		return fmt.Errorf("email: %w", err)
	}
	return nil
}

type PatchPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (b *PatchPasswordRequest) Validate() error {
	if err := api.ValidateRequiredField(b.CurrentPassword, "current_password"); err != nil {
		return fmt.Errorf("current_password: %w", err)
	}
	if err := api.ValidateRequiredField(b.NewPassword, "new_password"); err != nil {
		return fmt.Errorf("new_password: %w", err)
	}
	return nil
}
