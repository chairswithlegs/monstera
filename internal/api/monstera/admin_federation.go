package monstera

import (
	"net/http"
	"strconv"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera/internal/service"
	"github.com/go-chi/chi/v5"
)

// AdminFederationHandler handles known instances and domain blocks.
type AdminFederationHandler struct {
	instance   service.InstanceService
	moderation service.ModerationService
}

// NewAdminFederationHandler returns a new AdminFederationHandler.
func NewAdminFederationHandler(instance service.InstanceService, moderation service.ModerationService) *AdminFederationHandler {
	return &AdminFederationHandler{instance: instance, moderation: moderation}
}

// GETInstances returns known federated instances.
func (h *AdminFederationHandler) GETInstances(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, _ := strconv.Atoi(l); n > 0 && n <= 100 {
			limit = n
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, _ := strconv.Atoi(o); n >= 0 {
			offset = api.ClampOffset(n)
		}
	}
	instances, err := h.instance.ListKnownInstances(r.Context(), limit, offset)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.AdminKnownInstance, 0, len(instances))
	for i := range instances {
		out = append(out, apimodel.ToAdminKnownInstance(&instances[i]))
	}
	api.WriteJSON(w, http.StatusOK, apimodel.AdminKnownInstanceList{Instances: out})
}

// GETDomainBlocks returns domain blocks, joined with async purge progress
// for severity=suspend entries (issue #104).
func (h *AdminFederationHandler) GETDomainBlocks(w http.ResponseWriter, r *http.Request) {
	rows, err := h.moderation.ListDomainBlocksWithPurge(r.Context())
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.AdminDomainBlock, 0, len(rows))
	for i := range rows {
		out = append(out, apimodel.ToAdminDomainBlockWithPurge(&rows[i].Block, rows[i].Purge, rows[i].AccountsRemaining))
	}
	api.WriteJSON(w, http.StatusOK, apimodel.AdminDomainBlockList{DomainBlocks: out})
}

// POSTDomainBlocks creates a domain block (admin only; route protected by RequireAdmin).
func (h *AdminFederationHandler) POSTDomainBlocks(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.ErrForbidden)
		return
	}
	var body apimodel.PostDomainBlocksRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	_, err := h.moderation.CreateDomainBlock(r.Context(), user.ID, service.CreateDomainBlockInput{
		Domain:   body.Domain,
		Severity: body.Severity,
		Reason:   body.Reason,
	})
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETEDomainBlock removes a domain block (admin only; route protected by RequireAdmin).
func (h *AdminFederationHandler) DELETEDomainBlock(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		api.HandleError(w, r, api.ErrForbidden)
		return
	}
	domainName := chi.URLParam(r, "domain")
	if domainName == "" {
		api.HandleError(w, r, api.NewMissingRequiredParamError("domain"))
		return
	}
	if err := h.moderation.DeleteDomainBlock(r.Context(), user.ID, domainName); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
