package mastodon

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	oauthpkg "github.com/chairswithlegs/monstera/internal/oauth"
	"github.com/chairswithlegs/monstera/internal/service"
)

var validPushPolicies = []string{"all", "followed", "follower", "none"}

// PushHandler handles the /api/v1/push/subscription endpoints.
type PushHandler struct {
	pushSvc        service.PushSubscriptionService
	vapidPublicKey string
}

// NewPushHandler returns a new PushHandler.
func NewPushHandler(pushSvc service.PushSubscriptionService, vapidPublicKey string) *PushHandler {
	return &PushHandler{pushSvc: pushSvc, vapidPublicKey: vapidPublicKey}
}

type pushSubscriptionRequest struct {
	Subscription struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256DH string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	} `json:"subscription"`
	Data struct {
		Alerts pushAlertsRequest `json:"alerts"`
		Policy string            `json:"policy"`
	} `json:"data"`
}

type pushAlertsRequest struct {
	Follow        *bool `json:"follow"`
	Favourite     *bool `json:"favourite"`
	Reblog        *bool `json:"reblog"`
	Mention       *bool `json:"mention"`
	Poll          *bool `json:"poll"`
	Status        *bool `json:"status"`
	Update        *bool `json:"update"`
	FollowRequest *bool `json:"follow_request"`
}

func (a pushAlertsRequest) toDomain() domain.PushAlerts {
	return domain.PushAlerts{
		Follow:        boolOrDefault(a.Follow, false),
		Favourite:     boolOrDefault(a.Favourite, false),
		Reblog:        boolOrDefault(a.Reblog, false),
		Mention:       boolOrDefault(a.Mention, false),
		Poll:          boolOrDefault(a.Poll, false),
		Status:        boolOrDefault(a.Status, false),
		Update:        boolOrDefault(a.Update, false),
		FollowRequest: boolOrDefault(a.FollowRequest, false),
	}
}

func boolOrDefault(v *bool, def bool) bool { //nolint:unparam // def is always false for now but keeps the helper reusable
	if v != nil {
		return *v
	}
	return def
}

type pushUpdateRequest struct {
	Data struct {
		Alerts pushAlertsRequest `json:"alerts"`
		Policy string            `json:"policy"`
	} `json:"data"`
}

type pushSubscriptionResponse struct {
	ID        string            `json:"id"`
	Endpoint  string            `json:"endpoint"`
	Alerts    domain.PushAlerts `json:"alerts"`
	ServerKey string            `json:"server_key"`
	Policy    string            `json:"policy"`
}

func pushResponse(ps *domain.PushSubscription, serverKey string) pushSubscriptionResponse {
	return pushSubscriptionResponse{
		ID:        ps.ID,
		Endpoint:  ps.Endpoint,
		Alerts:    ps.Alerts,
		ServerKey: serverKey,
		Policy:    ps.Policy,
	}
}

func requireTokenClaims(r *http.Request) (*oauthpkg.TokenClaims, error) {
	claims := middleware.TokenClaimsFromContext(r.Context())
	if claims == nil || claims.AccessTokenID == "" {
		return nil, api.ErrUnauthorized
	}
	return claims, nil
}

// POSTSubscription handles POST /api/v1/push/subscription.
func (h *PushHandler) POSTSubscription(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	claims, err := requireTokenClaims(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	var body pushSubscriptionRequest
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	if body.Subscription.Endpoint == "" {
		api.HandleError(w, r, api.NewBadRequestError("subscription[endpoint] is required"))
		return
	}
	policy := body.Data.Policy
	if policy == "" {
		policy = "all"
	}
	if err := api.ValidateOneOf(policy, validPushPolicies, "policy"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	ps, err := h.pushSvc.Create(r.Context(), claims.AccessTokenID, account.ID,
		body.Subscription.Endpoint, body.Subscription.Keys.P256DH, body.Subscription.Keys.Auth,
		body.Data.Alerts.toDomain(), policy)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, pushResponse(ps, h.vapidPublicKey))
}

// GETSubscription handles GET /api/v1/push/subscription.
func (h *PushHandler) GETSubscription(w http.ResponseWriter, r *http.Request) {
	if middleware.AccountFromContext(r.Context()) == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	claims, err := requireTokenClaims(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	ps, err := h.pushSvc.Get(r.Context(), claims.AccessTokenID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, pushResponse(ps, h.vapidPublicKey))
}

// PUTSubscription handles PUT /api/v1/push/subscription.
func (h *PushHandler) PUTSubscription(w http.ResponseWriter, r *http.Request) {
	if middleware.AccountFromContext(r.Context()) == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	claims, err := requireTokenClaims(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	var body pushUpdateRequest
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	policy := body.Data.Policy
	if policy == "" {
		policy = "all"
	}
	if err := api.ValidateOneOf(policy, validPushPolicies, "policy"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	ps, err := h.pushSvc.Update(r.Context(), claims.AccessTokenID, body.Data.Alerts.toDomain(), policy)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, pushResponse(ps, h.vapidPublicKey))
}

// DELETESubscription handles DELETE /api/v1/push/subscription.
func (h *PushHandler) DELETESubscription(w http.ResponseWriter, r *http.Request) {
	if middleware.AccountFromContext(r.Context()) == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	claims, err := requireTokenClaims(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if err := h.pushSvc.Delete(r.Context(), claims.AccessTokenID); err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]any{})
}
