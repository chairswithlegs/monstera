package monstera

import (
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
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToUser(user, account))
}

func (h *UserHandler) PATCHProfile(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	var body apimodel.PatchProfileRequest
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

func (h *UserHandler) PATCHPreferences(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	var body apimodel.PatchPreferencesRequest
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

func (h *UserHandler) PATCHEmail(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	var body apimodel.PatchEmailRequest
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

func (h *UserHandler) PATCHPassword(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	var body apimodel.PatchPasswordRequest
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
