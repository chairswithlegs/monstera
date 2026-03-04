package monstera

import (
	"net/http"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

type UserHandler struct{}

func NewUserHandler(accounts service.AccountService) *UserHandler {
	return &UserHandler{}
}

func (h *UserHandler) GETUser(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	out := apimodel.ToUser(user)
	api.WriteJSON(w, http.StatusOK, out)
}
