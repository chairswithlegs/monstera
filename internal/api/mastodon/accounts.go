package mastodon

import (
	"errors"
	"log/slog"
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
	accounts *service.AccountService
	follows  *service.FollowService
	logger   *slog.Logger
	domain   string
}

// NewAccountsHandler returns a new AccountsHandler. follows may be nil to disable follow endpoints.
func NewAccountsHandler(accounts *service.AccountService, follows *service.FollowService, logger *slog.Logger, instanceDomain string) *AccountsHandler {
	return &AccountsHandler{
		accounts: accounts,
		follows:  follows,
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
	out := apimodel.ToAccountWithSource(acc, user, h.domain)
	api.WriteJSON(w, http.StatusOK, out)
}

// Get handles GET /api/v1/accounts/:id. Auth optional.
func (h *AccountsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.WriteError(w, http.StatusNotFound, "Record not found")
		return
	}
	acc, err := h.accounts.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.WriteError(w, http.StatusNotFound, "Record not found")
			return
		}
		api.HandleError(w, r, h.logger, err)
		return
	}
	if acc.Suspended {
		api.WriteError(w, http.StatusNotFound, "Record not found")
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToAccount(acc, h.domain))
}

// Follow handles POST /api/v1/accounts/:id/follow. Auth required.
func (h *AccountsHandler) Follow(w http.ResponseWriter, r *http.Request) {
	if h.follows == nil {
		api.HandleError(w, r, h.logger, errors.New("follow service not configured"))
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
	rel, err := h.follows.Follow(r.Context(), account.ID, targetID)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			api.WriteError(w, http.StatusBadRequest, "You cannot follow yourself")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			api.WriteError(w, http.StatusNotFound, "Record not found")
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			api.WriteError(w, http.StatusForbidden, "This action is not allowed")
			return
		}
		api.HandleError(w, r, h.logger, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// Unfollow handles POST /api/v1/accounts/:id/unfollow. Auth required.
func (h *AccountsHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
	if h.follows == nil {
		api.HandleError(w, r, h.logger, errors.New("follow service not configured"))
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
	rel, err := h.follows.Unfollow(r.Context(), account.ID, targetID)
	if err != nil {
		api.HandleError(w, r, h.logger, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// Relationships handles GET /api/v1/accounts/relationships?id[]=... Returns []Relationship for each requested id.
func (h *AccountsHandler) Relationships(w http.ResponseWriter, r *http.Request) {
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
		rel, err := h.accounts.GetRelationship(r.Context(), account.ID, targetID)
		if err != nil {
			api.HandleError(w, r, h.logger, err)
			return
		}
		out = append(out, apimodel.ToRelationship(rel))
	}
	api.WriteJSON(w, http.StatusOK, out)
}
