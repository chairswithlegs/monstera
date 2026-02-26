package mastodon

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
)

// AccountsHandler handles account-related Mastodon API endpoints.
type AccountsHandler struct {
	deps Deps
}

// NewAccountsHandler returns a new AccountsHandler. deps.Follows may be nil to disable follow endpoints.
func NewAccountsHandler(deps Deps) *AccountsHandler {
	return &AccountsHandler{deps: deps}
}

// GETVerifyCredentials handles GET /api/v1/accounts/verify_credentials.
func (h *AccountsHandler) GETVerifyCredentials(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
		return
	}
	acc, user, err := h.deps.Accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
			return
		}
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	out := apimodel.ToAccountWithSource(acc, user, h.deps.InstanceDomain)
	api.WriteJSON(w, http.StatusOK, out)
}

// GETAccounts handles GETAccounts /api/v1/accounts/:id. Auth optional.
func (h *AccountsHandler) GETAccounts(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.WriteError(w, http.StatusNotFound, "Record not found")
		return
	}
	acc, err := h.deps.Accounts.GetByID(r.Context(), id)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	if acc.Suspended {
		api.WriteError(w, http.StatusNotFound, "Record not found")
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToAccount(acc, h.deps.InstanceDomain))
}

// POSTFollow handles POST /api/v1/accounts/:id/follow. Auth required.
func (h *AccountsHandler) POSTFollow(w http.ResponseWriter, r *http.Request) {
	if h.deps.Follows == nil {
		api.HandleError(w, r, h.deps.Logger, errors.New("follow service not configured"))
		return
	}
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.WriteError(w, http.StatusNotFound, "Record not found")
		return
	}
	rel, err := h.deps.Follows.Follow(r.Context(), account.ID, targetID)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// POSTUnfollow handles POST /api/v1/accounts/:id/unfollow. Auth required.
func (h *AccountsHandler) POSTUnfollow(w http.ResponseWriter, r *http.Request) {
	if h.deps.Follows == nil {
		api.HandleError(w, r, h.deps.Logger, errors.New("follow service not configured"))
		return
	}
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
		return
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.WriteError(w, http.StatusNotFound, "Record not found")
		return
	}
	rel, err := h.deps.Follows.Unfollow(r.Context(), account.ID, targetID)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// GETRelationships handles GET /api/v1/accounts/relationships?id[]=... Returns []Relationship for each requested id.
func (h *AccountsHandler) GETRelationships(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
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
		rel, err := h.deps.Accounts.GetRelationship(r.Context(), account.ID, targetID)
		if err != nil {
			api.HandleError(w, r, h.deps.Logger, err)
			return
		}
		out = append(out, apimodel.ToRelationship(rel))
	}
	api.WriteJSON(w, http.StatusOK, out)
}
