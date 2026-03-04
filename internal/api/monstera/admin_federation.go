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

// AdminFederationHandler handles known instances and domain blocks.
type AdminFederationHandler struct {
	accounts   service.AccountService
	instance   service.InstanceService
	moderation service.ModerationService
}

// NewAdminFederationHandler returns a new AdminFederationHandler.
func NewAdminFederationHandler(accounts service.AccountService, instance service.InstanceService, moderation service.ModerationService) *AdminFederationHandler {
	return &AdminFederationHandler{accounts: accounts, instance: instance, moderation: moderation}
}

func (h *AdminFederationHandler) requireAdmin(r *http.Request) bool {
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

func (h *AdminFederationHandler) moderatorUserID(r *http.Request) (string, error) {
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

// GETInstances returns known federated instances.
func (h *AdminFederationHandler) GETInstances(w http.ResponseWriter, r *http.Request) {
	if _, err := h.moderatorUserID(r); err != nil {
		api.HandleError(w, r, err)
		return
	}
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
	if _, err := h.moderatorUserID(r); err != nil {
		api.HandleError(w, r, err)
		return
	}
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

// POSTDomainBlocks creates a domain block (admin only).
func (h *AdminFederationHandler) POSTDomainBlocks(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		api.HandleError(w, r, api.ErrForbidden)
		return
	}
	modID, err := h.moderatorUserID(r)
	if err != nil {
		api.HandleError(w, r, err)
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
	_, err = h.moderation.CreateDomainBlock(r.Context(), modID, service.CreateDomainBlockInput{
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

// DELETEDomainBlock removes a domain block (admin only).
func (h *AdminFederationHandler) DELETEDomainBlock(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(r) {
		api.HandleError(w, r, api.ErrForbidden)
		return
	}
	modID, err := h.moderatorUserID(r)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	domainName := chi.URLParam(r, "domain")
	if domainName == "" {
		api.HandleError(w, r, api.NewBadRequestError("domain required"))
		return
	}
	if err := h.moderation.DeleteDomainBlock(r.Context(), modID, domainName); err != nil {
		api.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
