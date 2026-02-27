package activitypub

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/activitypub/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// WebFingerHandler handles GET /.well-known/webfinger?resource=acct:user@domain
//
// Returns a JRD (RFC 7033) document that maps an acct: URI to the AP Actor URL.
type WebFingerHandler struct {
	accounts *service.AccountService
	config   *config.Config
}

// NewWebFingerHandler returns a new WebFingerHandler.
func NewWebFingerHandler(accounts *service.AccountService, config *config.Config) *WebFingerHandler {
	return &WebFingerHandler{accounts: accounts, config: config}
}

// GETWebFinger handles the WebFinger request.
func (h *WebFingerHandler) GETWebFinger(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	if err := api.ValidateRequiredString(resource); err != nil {
		api.HandleError(w, r, err)
		return
	}
	if !strings.HasPrefix(resource, "acct:") {
		err := api.NewBadRequestError("resource must use acct: scheme")
		api.HandleError(w, r, err)
		return
	}
	acct := strings.TrimPrefix(resource, "acct:")
	parts := strings.SplitN(acct, "@", 2)
	if len(parts) != 2 {
		err := api.NewBadRequestError("invalid acct URI")
		api.HandleError(w, r, err)
		return
	}
	username := parts[0]
	acctDomain := parts[1]
	if !strings.EqualFold(acctDomain, h.config.InstanceDomain) {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	account, err := h.accounts.GetLocalActorForFederation(r.Context(), username)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	actorURL := fmt.Sprintf("https://%s/users/%s", h.config.InstanceDomain, account.Username)
	resp := apimodel.WebFingerResponse{
		Subject: resource,
		Aliases: []string{actorURL},
		Links: []apimodel.WebFingerLink{
			{Rel: "self", Type: "application/activity+json", Href: actorURL},
		},
	}
	w.Header().Set("Cache-Control", "max-age=3600")
	api.WriteJRD(w, http.StatusOK, resp)
}
