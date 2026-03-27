package mastodon

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
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
	var following *bool
	if account := middleware.AccountFromContext(r.Context()); account != nil {
		f, err := h.tagFollows.IsFollowingTag(r.Context(), account.ID, tag.ID)
		if err != nil {
			slog.WarnContext(r.Context(), "failed to check tag follow state", slog.String("tag", name), slog.Any("error", err))
		}
		following = &f
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
	f := true
	api.WriteJSON(w, http.StatusOK, apimodel.TagFromDomain(tag, h.instanceDomain, &f))
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
	f := false
	api.WriteJSON(w, http.StatusOK, apimodel.TagFromDomain(tag, h.instanceDomain, &f))
}

// GETAccountFeaturedTags handles GET /api/v1/accounts/:id/featured_tags.
// Returns the featured hashtags for the given account. No authentication required.
func (h *AccountsHandler) GETAccountFeaturedTags(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	target, err := h.accounts.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	list, err := h.featuredTags.ListFeaturedTags(r.Context(), target.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	baseURL := "https://" + h.instanceDomain + "/@" + target.Username + "/tagged/"
	out := make([]apimodel.FeaturedTag, 0, len(list))
	for i := range list {
		out = append(out, apimodel.FeaturedTagFromDomain(list[i], baseURL+list[i].Name))
	}
	api.WriteJSON(w, http.StatusOK, out)
}
