package activitypub

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/api"
)

// WebFingerHandler handles GET /.well-known/webfinger?resource=acct:user@domain
//
// Returns a JRD (RFC 7033) document that maps an acct: URI to the AP Actor URL.
type WebFingerHandler struct {
	deps Deps
}

// NewWebFingerHandler constructs a WebFingerHandler.
func NewWebFingerHandler(deps Deps) *WebFingerHandler {
	return &WebFingerHandler{deps: deps}
}

type webFingerResponse struct {
	Subject string          `json:"subject"`
	Aliases []string        `json:"aliases"`
	Links   []webFingerLink `json:"links"`
}

type webFingerLink struct {
	Rel  string `json:"rel"`
	Type string `json:"type,omitempty"`
	Href string `json:"href,omitempty"`
}

// ServeHTTP handles the WebFinger request.
func (h *WebFingerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	if resource == "" {
		api.WriteError(w, http.StatusBadRequest, "missing resource parameter")
		return
	}
	if !strings.HasPrefix(resource, "acct:") {
		api.WriteError(w, http.StatusBadRequest, "resource must use acct: scheme")
		return
	}
	acct := strings.TrimPrefix(resource, "acct:")
	parts := strings.SplitN(acct, "@", 2)
	if len(parts) != 2 {
		api.WriteError(w, http.StatusBadRequest, "invalid acct URI")
		return
	}
	username := parts[0]
	acctDomain := parts[1]
	if !strings.EqualFold(acctDomain, h.deps.Config.InstanceDomain) {
		api.WriteError(w, http.StatusNotFound, "account not found")
		return
	}
	account, err := h.deps.Accounts.GetLocalActorForFederation(r.Context(), username)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	actorURL := fmt.Sprintf("https://%s/users/%s", h.deps.Config.InstanceDomain, account.Username)
	resp := webFingerResponse{
		Subject: resource,
		Aliases: []string{actorURL},
		Links: []webFingerLink{
			{Rel: "self", Type: "application/activity+json", Href: actorURL},
		},
	}
	w.Header().Set("Cache-Control", "max-age=3600")
	writeJRD(w, http.StatusOK, resp)
}
