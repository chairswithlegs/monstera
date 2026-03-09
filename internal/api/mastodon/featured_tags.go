package mastodon

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// FeaturedTagsHandler handles GET/POST/DELETE /api/v1/featured_tags and GET suggestions.
type FeaturedTagsHandler struct {
	featuredTags   service.FeaturedTagService
	accounts       service.AccountService
	instanceDomain string
}

// NewFeaturedTagsHandler returns a new FeaturedTagsHandler.
func NewFeaturedTagsHandler(featuredTags service.FeaturedTagService, accounts service.AccountService, instanceDomain string) *FeaturedTagsHandler {
	return &FeaturedTagsHandler{
		featuredTags:   featuredTags,
		accounts:       accounts,
		instanceDomain: instanceDomain,
	}
}

// GETFeaturedTags handles GET /api/v1/featured_tags.
func (h *FeaturedTagsHandler) GETFeaturedTags(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	list, err := h.featuredTags.ListFeaturedTags(r.Context(), account.ID)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	baseURL := "https://" + h.instanceDomain + "/@" + account.Username + "/tagged/"
	out := make([]apimodel.FeaturedTag, 0, len(list))
	for i := range list {
		out = append(out, apimodel.FeaturedTagFromDomain(list[i], baseURL+list[i].Name))
	}
	api.WriteJSON(w, http.StatusOK, out)
}

type postFeaturedTagRequest struct {
	Name string `json:"name"`
}

func (r *postFeaturedTagRequest) Validate() error {
	if err := api.ValidateRequiredField(r.Name, "name"); err != nil {
		return fmt.Errorf("name: %w", err)
	}
	return nil
}

// POSTFeaturedTags handles POST /api/v1/featured_tags. Body: { "name": "hashtag" }.
func (h *FeaturedTagsHandler) POSTFeaturedTags(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	var body postFeaturedTagRequest
	if err := api.DecodeAndValidateJSON(r, &body); err != nil {
		api.HandleError(w, r, err)
		return
	}
	ft, err := h.featuredTags.CreateFeaturedTag(r.Context(), account.ID, body.Name)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			api.HandleError(w, r, api.NewUnprocessableError("Validation failed: Tag is invalid"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	baseURL := "https://" + h.instanceDomain + "/@" + account.Username + "/tagged/"
	api.WriteJSON(w, http.StatusOK, apimodel.FeaturedTagFromDomain(*ft, baseURL+ft.Name))
}

// DELETEFeaturedTag handles DELETE /api/v1/featured_tags/:id.
func (h *FeaturedTagsHandler) DELETEFeaturedTag(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	if err := h.featuredTags.DeleteFeaturedTag(r.Context(), account.ID, id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]any{})
}

// GETFeaturedTagSuggestions handles GET /api/v1/featured_tags/suggestions.
func (h *FeaturedTagsHandler) GETFeaturedTagSuggestions(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 40 {
			limit = n
		}
	}
	tags, counts, err := h.featuredTags.GetSuggestions(r.Context(), account.ID, limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	type suggestionTag struct {
		ID      string        `json:"id"`
		Name    string        `json:"name"`
		URL     string        `json:"url"`
		History []interface{} `json:"history"`
	}
	out := make([]suggestionTag, 0, len(tags))
	for i := range tags {
		out = append(out, suggestionTag{
			ID:      tags[i].ID,
			Name:    tags[i].Name,
			URL:     "https://" + h.instanceDomain + "/tags/" + tags[i].Name,
			History: []interface{}{},
		})
		_ = counts[i]
	}
	api.WriteJSON(w, http.StatusOK, out)
}
