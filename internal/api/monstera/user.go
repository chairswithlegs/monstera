package monstera

import (
	"errors"
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

type UserHandler struct {
	accounts service.AccountService
}

func NewUserHandler(accounts service.AccountService) *UserHandler {
	return &UserHandler{accounts: accounts}
}

func (h *UserHandler) GETUser(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}

	_, user, err := h.accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.NewUnauthorizedError("Invalid account"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := apimodel.ToUser(user)
	api.WriteJSON(w, http.StatusOK, out)
}
