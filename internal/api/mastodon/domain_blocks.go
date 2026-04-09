package mastodon

import (
	"net/http"
	"strings"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
)

// GETDomainBlocks handles GET /api/v1/domain_blocks. Returns paginated blocked domains for the authenticated user.
func (h *AccountsHandler) GETDomainBlocks(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	params := PageParamsFromRequest(r)
	limit := parseLimitParam(r, DefaultListLimit, MaxListLimit)
	domains, nextCursor, err := h.follows.ListDomainBlocks(r.Context(), account.ID, optionalString(params.MaxID), limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	if nextCursor != nil && *nextCursor != "" {
		if link := linkHeaderWithNext(AbsoluteRequestURL(r, h.instanceDomain), *nextCursor); link != "" {
			w.Header().Set("Link", link)
		}
	}
	api.WriteJSON(w, http.StatusOK, domains)
}

// POSTDomainBlock handles POST /api/v1/domain_blocks. Blocks a domain for the authenticated user.
func (h *AccountsHandler) POSTDomainBlock(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	domain := parseDomainParam(r)
	if domain == "" {
		api.HandleError(w, r, api.NewMissingRequiredParamError("domain"))
		return
	}
	if err := h.follows.DomainBlock(r.Context(), account.ID, domain); err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, struct{}{})
}

// DELETEDomainBlock handles DELETE /api/v1/domain_blocks. Unblocks a domain for the authenticated user.
func (h *AccountsHandler) DELETEDomainBlock(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	domain := parseDomainParam(r)
	if domain == "" {
		api.HandleError(w, r, api.NewMissingRequiredParamError("domain"))
		return
	}
	if err := h.follows.DomainUnblock(r.Context(), account.ID, domain); err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, struct{}{})
}

// parseDomainParam extracts the "domain" parameter from the form body (POST) or
// query string (DELETE). ParseForm is called explicitly so that query parameters
// are available via FormValue regardless of HTTP method.
func parseDomainParam(r *http.Request) string {
	_ = r.ParseForm()
	if d := r.FormValue("domain"); d != "" {
		return strings.TrimSpace(d)
	}
	return ""
}
