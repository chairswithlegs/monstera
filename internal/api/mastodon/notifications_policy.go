package mastodon

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

const maxNotificationRequestBulkLimit = 40

// NotificationsPolicyHandler handles notification policy and request Mastodon API endpoints.
type NotificationsPolicyHandler struct {
	policy         service.NotificationPolicyService
	accounts       service.AccountService
	statuses       service.StatusService
	instanceDomain string
}

// NewNotificationsPolicyHandler returns a new NotificationsPolicyHandler.
func NewNotificationsPolicyHandler(
	policy service.NotificationPolicyService,
	accounts service.AccountService,
	statuses service.StatusService,
	instanceDomain string,
) *NotificationsPolicyHandler {
	return &NotificationsPolicyHandler{
		policy:         policy,
		accounts:       accounts,
		statuses:       statuses,
		instanceDomain: instanceDomain,
	}
}

// GETPolicy handles GET /api/v1/notifications/policy.
func (h *NotificationsPolicyHandler) GETPolicy(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	p, err := h.policy.GetOrCreatePolicy(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	pendingRequests, pendingNotifications, err := h.policy.PolicySummary(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToNotificationPolicyResponse(p, pendingRequests, pendingNotifications))
}

// patchPolicyRequest is the request body for PATCH /api/v1/notifications/policy.
// Pointer fields implement partial-update semantics: omitted fields are left unchanged.
type patchPolicyRequest struct {
	FilterNotFollowing    *bool `json:"filter_not_following"`
	FilterNotFollowers    *bool `json:"filter_not_followers"`
	FilterNewAccounts     *bool `json:"filter_new_accounts"`
	FilterPrivateMentions *bool `json:"filter_private_mentions"`
}

// PATCHPolicy handles PATCH /api/v1/notifications/policy.
func (h *NotificationsPolicyHandler) PATCHPolicy(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	var body patchPolicyRequest
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	p, err := h.policy.UpdatePolicy(r.Context(), service.UpdateNotificationPolicyInput{
		AccountID:             account.ID,
		FilterNotFollowing:    body.FilterNotFollowing,
		FilterNotFollowers:    body.FilterNotFollowers,
		FilterNewAccounts:     body.FilterNewAccounts,
		FilterPrivateMentions: body.FilterPrivateMentions,
	})
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	pendingRequests, pendingNotifications, err := h.policy.PolicySummary(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToNotificationPolicyResponse(p, pendingRequests, pendingNotifications))
}

// GETRequests handles GET /api/v1/notifications/requests.
func (h *NotificationsPolicyHandler) GETRequests(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	params := PageParamsFromRequest(r)
	maxID := optionalString(params.MaxID)
	list, err := h.policy.ListRequests(r.Context(), account.ID, maxID, params.Limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.NotificationRequestResponse, 0, len(list))
	for i := range list {
		req := &list[i]
		fromAcc, _ := h.accounts.GetByID(r.Context(), req.FromAccountID)
		var lastStatus *apimodel.Status
		if h.statuses != nil && req.LastStatusID != nil {
			if enriched, err := h.statuses.GetByIDEnriched(r.Context(), *req.LastStatusID, &account.ID); err == nil {
				s := apimodel.StatusFromEnriched(enriched, h.instanceDomain)
				lastStatus = &s
			}
		}
		out = append(out, apimodel.ToNotificationRequestResponse(req, fromAcc, lastStatus, h.instanceDomain))
	}
	firstID, lastID := firstLastRequestIDs(list)
	if link := LinkHeader(AbsoluteRequestURL(r, h.instanceDomain), firstID, lastID); link != "" {
		w.Header().Set("Link", link)
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETRequest handles GET /api/v1/notifications/requests/:id.
func (h *NotificationsPolicyHandler) GETRequest(w http.ResponseWriter, r *http.Request) {
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
	req, err := h.policy.GetRequest(r.Context(), id, account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	fromAcc, _ := h.accounts.GetByID(r.Context(), req.FromAccountID)
	var lastStatus *apimodel.Status
	if h.statuses != nil && req.LastStatusID != nil {
		if enriched, err := h.statuses.GetByIDEnriched(r.Context(), *req.LastStatusID, &account.ID); err == nil {
			s := apimodel.StatusFromEnriched(enriched, h.instanceDomain)
			lastStatus = &s
		}
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToNotificationRequestResponse(req, fromAcc, lastStatus, h.instanceDomain))
}

// POSTAcceptRequest handles POST /api/v1/notifications/requests/:id/accept.
func (h *NotificationsPolicyHandler) POSTAcceptRequest(w http.ResponseWriter, r *http.Request) {
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
	if err := h.policy.AcceptRequest(r.Context(), id, account.ID); err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{})
}

// POSTDismissRequest handles POST /api/v1/notifications/requests/:id/dismiss.
func (h *NotificationsPolicyHandler) POSTDismissRequest(w http.ResponseWriter, r *http.Request) {
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
	if err := h.policy.DismissRequest(r.Context(), id, account.ID); err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{})
}

// bulkRequestsBody is the request body for bulk accept/dismiss endpoints.
type bulkRequestsBody struct {
	IDs []string `json:"id"`
}

// POSTAcceptRequests handles POST /api/v1/notifications/requests/accept.
func (h *NotificationsPolicyHandler) POSTAcceptRequests(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	var body bulkRequestsBody
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	if len(body.IDs) > maxNotificationRequestBulkLimit {
		api.HandleError(w, r, fmt.Errorf("too many ids: %w", api.ErrBadRequest))
		return
	}
	if err := h.policy.AcceptRequestsByIDs(r.Context(), account.ID, body.IDs); err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{})
}

// POSTDismissRequests handles POST /api/v1/notifications/requests/dismiss.
func (h *NotificationsPolicyHandler) POSTDismissRequests(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	var body bulkRequestsBody
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	if len(body.IDs) > maxNotificationRequestBulkLimit {
		api.HandleError(w, r, fmt.Errorf("too many ids: %w", api.ErrBadRequest))
		return
	}
	if err := h.policy.DismissRequestsByIDs(r.Context(), account.ID, body.IDs); err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]interface{}{})
}

// GETMerged handles GET /api/v1/notifications/requests/merged.
// Monstera processes notifications synchronously; they are always merged.
func (h *NotificationsPolicyHandler) GETMerged(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]bool{"merged": true})
}

func firstLastRequestIDs(list []domain.NotificationRequest) (firstID, lastID string) {
	if len(list) == 0 {
		return "", ""
	}
	return list[0].ID, list[len(list)-1].ID
}
