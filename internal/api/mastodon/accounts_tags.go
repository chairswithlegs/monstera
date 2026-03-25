package mastodon

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
)

// GETFollowedTags handles GET /api/v1/followed_tags.
func (h *AccountsHandler) GETFollowedTags(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	params := PageParamsFromRequest(r)
	limit := parseLimitParam(r, DefaultListLimit, MaxListLimit)
	tags, nextCursor, err := h.tagFollows.ListFollowedTags(r.Context(), account.ID, optionalString(params.MaxID), limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Tag, 0, len(tags))
	for i := range tags {
		out = append(out, apimodel.FollowedTagFromDomain(tags[i], h.instanceDomain))
	}
	if nextCursor != nil && *nextCursor != "" {
		if link := linkHeaderWithNext(AbsoluteRequestURL(r, h.instanceDomain), *nextCursor); link != "" {
			w.Header().Set("Link", link)
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// GETTag handles GET /api/v1/tags/{name}. Auth optional; returns following when authenticated.
func (h *AccountsHandler) GETTag(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(strings.ToLower(chi.URLParam(r, "name")))
	if name == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	tag, err := h.tagFollows.GetTagByName(r.Context(), name)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	following := false
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		following, _ = h.tagFollows.IsFollowingTag(r.Context(), account.ID, tag.ID)
	}
	api.WriteJSON(w, http.StatusOK, apimodel.TagFromDomain(tag, h.instanceDomain, following))
}

// POSTTagFollow handles POST /api/v1/tags/{name}/follow. Requires auth.
func (h *AccountsHandler) POSTTagFollow(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	name := strings.TrimSpace(strings.ToLower(chi.URLParam(r, "name")))
	if name == "" {
		api.HandleError(w, r, api.NewMissingRequiredParamError("name"))
		return
	}
	tag, err := h.tagFollows.FollowTag(r.Context(), account.ID, name)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.TagFromDomain(tag, h.instanceDomain, true))
}

// POSTTagUnfollow handles POST /api/v1/tags/{name}/unfollow. Requires auth.
func (h *AccountsHandler) POSTTagUnfollow(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	name := strings.TrimSpace(strings.ToLower(chi.URLParam(r, "name")))
	if name == "" {
		api.HandleError(w, r, api.NewMissingRequiredParamError("name"))
		return
	}
	tag, err := h.tagFollows.UnfollowTagByName(r.Context(), account.ID, name)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.TagFromDomain(tag, h.instanceDomain, false))
}
