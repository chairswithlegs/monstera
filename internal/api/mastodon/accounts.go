package mastodon

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// AccountsHandler handles account-related Mastodon API endpoints.
type AccountsHandler struct {
	accounts       *service.AccountService
	follows        *service.FollowService
	instanceDomain string
}

// NewAccountsHandler returns a new AccountsHandler. follows may be nil to disable follow endpoints.
func NewAccountsHandler(accounts *service.AccountService, follows *service.FollowService, instanceDomain string) *AccountsHandler {
	return &AccountsHandler{accounts: accounts, follows: follows, instanceDomain: instanceDomain}
}

// GETVerifyCredentials handles GET /api/v1/accounts/verify_credentials.
func (h *AccountsHandler) GETVerifyCredentials(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
		return
	}
	acc, user, err := h.accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.NewUnauthorizedError("The access token is invalid"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := apimodel.ToAccountWithSource(acc, user, h.instanceDomain)
	api.WriteJSON(w, http.StatusOK, out)
}

// GETAccounts handles GETAccounts /api/v1/accounts/:id. Auth optional.
func (h *AccountsHandler) GETAccounts(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := api.ValidateRequiredString(id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	acc, err := h.accounts.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	if acc.Suspended {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToAccount(acc, h.instanceDomain))
}

// POSTFollow handles POST /api/v1/accounts/:id/follow. Auth required.
func (h *AccountsHandler) POSTFollow(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if err := api.ValidateRequiredString(targetID); err != nil {
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Follow(r.Context(), account.ID, targetID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// POSTUnfollow handles POST /api/v1/accounts/:id/unfollow. Auth required.
func (h *AccountsHandler) POSTUnfollow(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	targetID := chi.URLParam(r, "id")
	if err := api.ValidateRequiredString(targetID); err != nil {
		api.HandleError(w, r, err)
		return
	}
	rel, err := h.follows.Unfollow(r.Context(), account.ID, targetID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// GETRelationships handles GET /api/v1/accounts/relationships?id[]=... Returns []Relationship for each requested id.
func (h *AccountsHandler) GETRelationships(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	ids := r.URL.Query()["id[]"]
	if len(ids) == 0 {
		api.WriteJSON(w, http.StatusOK, []apimodel.Relationship{})
		return
	}
	out := make([]apimodel.Relationship, 0, len(ids))
	for _, targetID := range ids {
		if targetID == "" {
			continue
		}
		rel, err := h.accounts.GetRelationship(r.Context(), account.ID, targetID)
		if err != nil {
			api.HandleError(w, r, err)
			return
		}
		out = append(out, apimodel.ToRelationship(rel))
	}
	api.WriteJSON(w, http.StatusOK, out)
}
