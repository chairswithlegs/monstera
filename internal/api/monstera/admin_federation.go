package monstera

import (
	"net/http"
	"strconv"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/api/monstera/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
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
			offset = n
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

// GETDomainBlocks returns domain blocks.
func (h *AdminFederationHandler) GETDomainBlocks(w http.ResponseWriter, r *http.Request) {
	blocks, err := h.moderation.ListDomainBlocks(r.Context())
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.AdminDomainBlock, 0, len(blocks))
	for i := range blocks {
		out = append(out, apimodel.ToAdminDomainBlock(&blocks[i]))
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
	var body struct {
		Domain   string  `json:"domain"`
		Severity string  `json:"severity"`
		Reason   *string `json:"reason"`
	}
	if err := api.DecodeJSONBody(r, &body); err != nil {
		api.HandleError(w, r, api.NewBadRequestError("invalid JSON"))
		return
	}
	if body.Domain == "" {
		api.HandleError(w, r, api.NewBadRequestError("domain required"))
		return
	}
	if body.Severity != domain.DomainBlockSeveritySilence && body.Severity != domain.DomainBlockSeveritySuspend {
		body.Severity = domain.DomainBlockSeveritySilence
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
		api.HandleError(w, r, api.NewBadRequestError("domain required"))
		return
	}
	if err := h.moderation.DeleteDomainBlock(r.Context(), user.ID, domainName); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
