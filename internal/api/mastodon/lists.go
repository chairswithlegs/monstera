package mastodon

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// ListsHandler handles GET/POST/PUT/DELETE /api/v1/lists and list accounts.
type ListsHandler struct {
	lists          service.ListService
	accounts       service.AccountService
	instanceDomain string
}

// NewListsHandler returns a new ListsHandler.
func NewListsHandler(lists service.ListService, accounts service.AccountService, instanceDomain string) *ListsHandler {
	return &ListsHandler{
		lists:          lists,
		accounts:       accounts,
		instanceDomain: instanceDomain,
	}
}

// GETLists handles GET /api/v1/lists.
func (h *ListsHandler) GETLists(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	lists, err := h.lists.ListLists(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.List, 0, len(lists))
	for i := range lists {
		out = append(out, apimodel.ToList(&lists[i]))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETList handles GET /api/v1/lists/:id.
func (h *ListsHandler) GETList(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	l, err := h.lists.GetList(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToList(l))
}

// POSTListsRequest is the body for POST /api/v1/lists.
type POSTListsRequest struct {
	Title         string `json:"title"`
	RepliesPolicy string `json:"replies_policy"`
	Exclusive     bool   `json:"exclusive"`
}

// POSTLists handles POST /api/v1/lists.
func (h *ListsHandler) POSTLists(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	var body POSTListsRequest
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	l, err := h.lists.CreateList(r.Context(), account.ID, body.Title, body.RepliesPolicy, body.Exclusive)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			api.HandleError(w, r, api.NewMissingRequiredFieldError("title"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToList(l))
}

// PUTListRequest is the body for PUT /api/v1/lists/:id.
type PUTListRequest struct {
	Title         string `json:"title"`
	RepliesPolicy string `json:"replies_policy"`
	Exclusive     bool   `json:"exclusive"`
}

// PUTList handles PUT /api/v1/lists/:id.
func (h *ListsHandler) PUTList(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var body PUTListRequest
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	l, err := h.lists.UpdateList(r.Context(), account.ID, id, body.Title, body.RepliesPolicy, body.Exclusive)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToList(l))
}

// DELETEList handles DELETE /api/v1/lists/:id.
func (h *ListsHandler) DELETEList(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	if err := h.lists.DeleteList(r.Context(), account.ID, id); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// GETListAccounts handles GET /api/v1/lists/:id/accounts.
func (h *ListsHandler) GETListAccounts(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	accounts, err := h.lists.GetListAccounts(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Account, 0, len(accounts))
	for i := range accounts {
		out = append(out, apimodel.ToAccount(&accounts[i], h.instanceDomain))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTListAccounts handles POST /api/v1/lists/:id/accounts.
func (h *ListsHandler) POSTListAccounts(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	accountIDs, err := parseListAccountIDsRequest(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if err := h.lists.AddAccountsToList(r.Context(), account.ID, id, accountIDs); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// DELETEListAccounts handles DELETE /api/v1/lists/:id/accounts.
func (h *ListsHandler) DELETEListAccounts(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	accountIDs, err := parseListAccountIDsRequest(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if err := h.lists.RemoveAccountsFromList(r.Context(), account.ID, id, accountIDs); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			api.HandleError(w, r, api.ErrForbidden)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// parseListAccountIDsRequest parses account_ids from a JSON or form-encoded body.
// Mastodon clients send account_ids[] as form params; JSON clients send {"account_ids":[...]}.
func parseListAccountIDsRequest(r *http.Request) ([]string, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			AccountIDs []string `json:"account_ids"`
		}
		if err := api.DecodeJSONBody(r, &body); err != nil {
			return nil, err
		}
		return body.AccountIDs, nil
	}
	// r.ParseForm only reads the body for POST/PUT/PATCH, not DELETE.
	// Read the body directly so form-encoded DELETE requests work too.
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, api.NewInvalidRequestBodyError()
	}
	form, err := url.ParseQuery(string(raw))
	if err != nil {
		return nil, api.NewInvalidRequestBodyError()
	}
	if ids := form["account_ids[]"]; len(ids) > 0 {
		return ids, nil
	}
	if ids := form["account_ids"]; len(ids) > 0 {
		return ids, nil
	}
	return nil, nil
}
