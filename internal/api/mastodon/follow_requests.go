package mastodon

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// FollowRequestsHandler handles GET/POST /api/v1/follow_requests.
type FollowRequestsHandler struct {
	follows        service.FollowService
	accounts       service.AccountService
	instanceDomain string
}

// NewFollowRequestsHandler returns a new FollowRequestsHandler.
func NewFollowRequestsHandler(follows service.FollowService, accounts service.AccountService, instanceDomain string) *FollowRequestsHandler {
	return &FollowRequestsHandler{
		follows:        follows,
		accounts:       accounts,
		instanceDomain: instanceDomain,
	}
}

// GETFollowRequests handles GET /api/v1/follow_requests.
func (h *FollowRequestsHandler) GETFollowRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}

	params := PageParamsFromRequest(r)
	maxID := optionalString(params.MaxID)
	accounts, nextCursor, err := h.follows.ListPendingFollowRequests(ctx, account.ID, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}

	out := make([]apimodel.Account, 0, len(accounts))
	for i := range accounts {
		out = append(out, apimodel.ToAccount(&accounts[i], h.instanceDomain))
	}

	if nextCursor != nil && *nextCursor != "" {
<<<<<<< HEAD
		if link := linkHeaderWithNext(AbsoluteRequestURL(r, h.instanceDomain), *nextCursor); link != "" {
=======
		if link := linkHeaderWithNext(r.URL.String(), *nextCursor); link != "" {
>>>>>>> main
			w.Header().Set("Link", link)
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// POSTAuthorize handles POST /api/v1/follow_requests/:id/authorize. :id is the requester's account_id.
func (h *FollowRequestsHandler) POSTAuthorize(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}

	requesterID := chi.URLParam(r, "id")
	if requesterID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}

	err := h.follows.AuthorizeFollowRequest(ctx, account.ID, requesterID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}

	rel, err := h.accounts.GetRelationship(ctx, account.ID, requesterID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}

// POSTReject handles POST /api/v1/follow_requests/:id/reject. :id is the requester's account_id.
func (h *FollowRequestsHandler) POSTReject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}

	requesterID := chi.URLParam(r, "id")
	if requesterID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}

	err := h.follows.RejectFollowRequest(ctx, account.ID, requesterID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}

	rel, _ := h.accounts.GetRelationship(ctx, account.ID, requesterID)
	if rel == nil {
		rel = &domain.Relationship{TargetID: requesterID}
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToRelationship(rel))
}
