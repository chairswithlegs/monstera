package monstera

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
	"github.com/go-chi/chi/v5"
)

// AdminUsersHandler handles admin user management.
type AdminUsersHandler struct {
	accounts   service.AccountService
	moderation service.ModerationService
}

// NewAdminUsersHandler returns a new AdminUsersHandler.
func NewAdminUsersHandler(accounts service.AccountService, moderation service.ModerationService) *AdminUsersHandler {
	return &AdminUsersHandler{accounts: accounts, moderation: moderation}
}

func (h *AdminUsersHandler) moderatorUserID(r *http.Request) (string, error) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		return "", api.ErrForbidden
	}
	_, user, err := h.accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		return "", fmt.Errorf("GetAccountWithUser: %w", err)
	}
	if user.Role != domain.RoleAdmin && user.Role != domain.RoleModerator {
		return "", api.ErrForbidden
	}
	return user.ID, nil
}

func (h *AdminUsersHandler) requireAdmin(r *http.Request) bool {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		return false
	}
	_, user, err := h.accounts.GetAccountWithUser(r.Context(), account.ID)
	if err != nil {
		return false
	}
	return user.Role == domain.RoleAdmin
}

// GETUsers returns a paginated list of local users.
func (h *AdminUsersHandler) GETUsers(w http.ResponseWriter, r *http.Request) {
	if _, err := h.moderatorUserID(r); err != nil {
		api.HandleError(w, r, err)
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, _ := strconv.Atoi(l); n > 0 && n <= 100 {
			limit = n
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, _ := strconv.Atoi(o); n >= 0 {
			offset = n
		}
	}
	users, err := h.accounts.ListLocalUsers(r.Context(), limit, offset)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.AdminUser, 0, len(users))
	for _, u := range users {
		acc, _ := h.accounts.GetByID(r.Context(), u.AccountID)
		suspended := acc != nil && acc.Suspended
		silenced := acc != nil && acc.Silenced
		username := ""
		if acc != nil {
			username = acc.Username
		}
		out = append(out, apimodel.AdminUserFromDomain(&u, username, suspended, silenced))
	}
	api.WriteJSON(w, http.StatusOK, apimodel.AdminUserList{Users: out})
}

// GETUser returns a single user by account ID.
func (h *AdminUsersHandler) GETUser(w http.ResponseWriter, r *http.Request) {
	if _, err := h.moderatorUserID(r); err != nil {
		api.HandleError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	acc, err := h.accounts.GetByID(r.Context(), id)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	_, user, err := h.accounts.GetAccountWithUser(r.Context(), id)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.AdminUserFromDomain(user, acc.Username, acc.Suspended, acc.Silenced))
}

// POSTSuspend suspends an account (moderator or admin).
func (h *AdminUsersHandler) POSTSuspend(w http.ResponseWriter, r *http.Request) {
	modID, err := h.moderatorUserID(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	if err := h.moderation.SuspendAccount(r.Context(), modID, id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POSTUnsuspend unsuspends an account.
func (h *AdminUsersHandler) POSTUnsuspend(w http.ResponseWriter, r *http.Request) {
	modID, err := h.moderatorUserID(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	if err := h.moderation.UnsuspendAccount(r.Context(), modID, id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POSTSilence silences an account.
func (h *AdminUsersHandler) POSTSilence(w http.ResponseWriter, r *http.Request) {
	modID, err := h.moderatorUserID(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	if err := h.moderation.SilenceAccount(r.Context(), modID, id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POSTUnsilence unsilences an account.
func (h *AdminUsersHandler) POSTUnsilence(w http.ResponseWriter, r *http.Request) {
	modID, err := h.moderatorUserID(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	if err := h.moderation.UnsilenceAccount(r.Context(), modID, id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUTRole sets a user's role (admin only).
func (h *AdminUsersHandler) PUTRole(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		api.HandleError(w, r, api.ErrForbidden)
		return
	}
	modID, err := h.moderatorUserID(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	var body struct {
		Role string `json:"role"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	if body.Role != domain.RoleUser && body.Role != domain.RoleModerator && body.Role != domain.RoleAdmin {
		api.HandleError(w, r, api.NewBadRequestError("invalid role"))
		return
	}
	user, err := h.accounts.GetUserByID(r.Context(), id)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if err := h.moderation.SetUserRole(r.Context(), modID, user.ID, body.Role); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETEUser deletes an account and user (admin only).
func (h *AdminUsersHandler) DELETEUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		api.HandleError(w, r, api.ErrForbidden)
		return
	}
	modID, err := h.moderatorUserID(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.NewBadRequestError("id required"))
		return
	}
	acc, err := h.accounts.GetByID(r.Context(), id)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if acc.Domain != nil && *acc.Domain != "" {
		api.HandleError(w, r, api.NewBadRequestError("cannot delete remote account"))
		return
	}

	// Error is ignored since the user may not exist and we want the endpoint to be idempotent.
	_, user, _ := h.accounts.GetAccountWithUser(r.Context(), id)
	if user != nil {
		if err := h.moderation.DeleteAccount(r.Context(), modID, id); err != nil {
			api.HandleError(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
