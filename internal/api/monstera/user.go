package monstera

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
)

type UserHandler struct {
	accounts service.AccountService
}

func NewUserHandler(accounts service.AccountService) *UserHandler {
	return &UserHandler{accounts: accounts}
}

func (h *UserHandler) GETUser(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToUser(user, account))
}

type patchProfileRequest struct {
	DisplayName        *string         `json:"display_name"`
	Note               *string         `json:"note"`
	Locked             bool            `json:"locked"`
	Bot                bool            `json:"bot"`
	Fields             json.RawMessage `json:"fields"`
	DefaultQuotePolicy *string         `json:"default_quote_policy"`
}

func (b *patchProfileRequest) Validate() error { return nil }

func (h *UserHandler) PATCHProfile(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	var body patchProfileRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	updatedAccount, updatedUser, err := h.accounts.UpdateCredentials(r.Context(), service.UpdateCredentialsInput{
		AccountID:          account.ID,
		DisplayName:        body.DisplayName,
		Note:               body.Note,
		Locked:             body.Locked,
		Bot:                body.Bot,
		Fields:             body.Fields,
		DefaultQuotePolicy: body.DefaultQuotePolicy,
	})
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToUser(updatedUser, updatedAccount))
}

type patchPreferencesRequest struct {
	DefaultPrivacy     string `json:"default_privacy"`
	DefaultSensitive   bool   `json:"default_sensitive"`
	DefaultLanguage    string `json:"default_language"`
	DefaultQuotePolicy string `json:"default_quote_policy"`
}

func (b *patchPreferencesRequest) Validate() error {
	if err := api.ValidateOneOf(b.DefaultPrivacy, []string{"public", "unlisted", "private", "direct"}, "default_privacy"); err != nil {
		return fmt.Errorf("default_privacy: %w", err)
	}
	if err := api.ValidateOneOf(b.DefaultQuotePolicy, []string{"public", "followers", "nobody"}, "default_quote_policy"); err != nil {
		return fmt.Errorf("default_quote_policy: %w", err)
	}
	if len(b.DefaultLanguage) > 35 {
		return fmt.Errorf("default_language: %w", api.NewUnprocessableError("too long"))
	}
	return nil
}

func (h *UserHandler) PATCHPreferences(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	var body patchPreferencesRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	updatedUser, err := h.accounts.UpdatePreferences(r.Context(), user.ID, service.UpdatePreferencesInput{
		DefaultPrivacy:     body.DefaultPrivacy,
		DefaultSensitive:   body.DefaultSensitive,
		DefaultLanguage:    body.DefaultLanguage,
		DefaultQuotePolicy: body.DefaultQuotePolicy,
	})
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	account := middleware.AccountFromContext(r.Context())
	api.WriteJSON(w, http.StatusOK, apimodel.ToUser(updatedUser, account))
}

type patchEmailRequest struct {
	Email string `json:"email"`
}

func (b *patchEmailRequest) Validate() error {
	if err := api.ValidateRequiredField(b.Email, "email"); err != nil {
		return fmt.Errorf("email: %w", err)
	}
	return nil
}

func (h *UserHandler) PATCHEmail(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	var body patchEmailRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	updatedUser, err := h.accounts.ChangeEmail(r.Context(), user.ID, body.Email)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	account := middleware.AccountFromContext(r.Context())
	api.WriteJSON(w, http.StatusOK, apimodel.ToUser(updatedUser, account))
}

type patchPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (b *patchPasswordRequest) Validate() error {
	if err := api.ValidateRequiredField(b.CurrentPassword, "current_password"); err != nil {
		return fmt.Errorf("current_password: %w", err)
	}
	if err := api.ValidateRequiredField(b.NewPassword, "new_password"); err != nil {
		return fmt.Errorf("new_password: %w", err)
	}
	return nil
}

func (h *UserHandler) PATCHPassword(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	var body patchPasswordRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	if err := h.accounts.ChangePassword(r.Context(), user.ID, body.CurrentPassword, body.NewPassword); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
