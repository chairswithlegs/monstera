package activitypub

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/activitypub/apimodel"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// WebFingerHandler handles GET /.well-known/webfinger?resource=acct:user@domain
//
// Returns a JRD (RFC 7033) document that maps an acct: URI to the AP Actor URL.
type WebFingerHandler struct {
	accounts        service.AccountService
	instanceDomain  string
	instanceBaseURL string
}

// NewWebFingerHandler returns a new WebFingerHandler.
func NewWebFingerHandler(accounts service.AccountService, instanceDomain string, instanceBaseURL string) *WebFingerHandler {
	return &WebFingerHandler{accounts: accounts, instanceDomain: instanceDomain, instanceBaseURL: instanceBaseURL}
}

// GETWebFinger handles the WebFinger request.
func (h *WebFingerHandler) GETWebFinger(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	if err := api.ValidateRequiredField(resource, "resource"); err != nil {
		api.HandleError(w, r, err)
		return
	}
	// Only "acct:username@domain" is supported.
	if !strings.HasPrefix(resource, "acct:") {
		err := api.NewInvalidValueError("resource")
		api.HandleError(w, r, err)
		return
	}
	acct := strings.TrimPrefix(resource, "acct:")
	parts := strings.SplitN(acct, "@", 2)
	if len(parts) != 2 {
		err := api.NewInvalidValueError("resource")
		api.HandleError(w, r, err)
		return
	}
	username := parts[0]
	acctDomain := parts[1]
	if !strings.EqualFold(acctDomain, h.instanceDomain) {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	account, err := h.accounts.GetActiveLocalAccount(r.Context(), username)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	actorURL := fmt.Sprintf("%s/users/%s", h.instanceBaseURL, account.Username)
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
