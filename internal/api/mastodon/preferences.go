package mastodon

import (
	"errors"
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// PreferencesResponse is the Mastodon API preferences response (flat key-value).
type PreferencesResponse struct {
	PostingDefaultVisibility string `json:"posting:default:visibility"`
	PostingDefaultSensitive  bool   `json:"posting:default:sensitive"`
	PostingDefaultLanguage   string `json:"posting:default:language"`
	ReadingExpandMedia       string `json:"reading:expand:media"`
	ReadingExpandSpoilers    bool   `json:"reading:expand:spoilers"`
}

// PreferencesHandler handles GET /api/v1/preferences.
type PreferencesHandler struct {
	accounts service.AccountService
}

// NewPreferencesHandler returns a new PreferencesHandler.
func NewPreferencesHandler(accounts service.AccountService) *PreferencesHandler {
	return &PreferencesHandler{accounts: accounts}
}

// GETPreferences handles GET /api/v1/preferences.
func (h *PreferencesHandler) GETPreferences(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	_, user, err := h.accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	visibility := user.DefaultPrivacy
	if visibility == "" {
		visibility = "public"
	}
	lang := user.DefaultLanguage
	resp := PreferencesResponse{
		PostingDefaultVisibility: visibility,
		PostingDefaultSensitive:  user.DefaultSensitive,
		PostingDefaultLanguage:   lang,
		ReadingExpandMedia:       "default",
		ReadingExpandSpoilers:    false,
	}
	api.WriteJSON(w, http.StatusOK, resp)
}
