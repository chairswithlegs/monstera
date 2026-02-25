package mastodon

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/presenter"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// AccountsHandler handles account-related Mastodon API endpoints.
type AccountsHandler struct {
	accounts *service.AccountService
	logger   *slog.Logger
	domain   string
}

// NewAccountsHandler returns a new AccountsHandler.
func NewAccountsHandler(accounts *service.AccountService, logger *slog.Logger, instanceDomain string) *AccountsHandler {
	return &AccountsHandler{
		accounts: accounts,
		logger:   logger,
		domain:   instanceDomain,
	}
}

// VerifyCredentials handles GET /api/v1/accounts/verify_credentials.
func (h *AccountsHandler) VerifyCredentials(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
		return
	}
	acc, user, err := h.accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
			return
		}
		api.HandleError(w, r, h.logger, err)
		return
	}
	out := presenter.ToAccountWithSource(acc, user, h.domain)
	api.WriteJSON(w, http.StatusOK, out)
}
