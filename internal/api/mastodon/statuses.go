package mastodon

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// StatusesHandler handles status-related Mastodon API endpoints.
type StatusesHandler struct {
	deps Deps
}

// NewStatusesHandler returns a new StatusesHandler.
func NewStatusesHandler(deps Deps) *StatusesHandler {
	return &StatusesHandler{deps: deps}
}

// CreateStatusRequest is the request body for POST /api/v1/statuses.
type CreateStatusRequest struct {
	Status      string `json:"status"`
	Visibility  string `json:"visibility"`
	SpoilerText string `json:"spoiler_text"`
	Sensitive   bool   `json:"sensitive"`
	Language    string `json:"language"`
}

// POSTStatuses handles POST /api/v1/statuses.
func (h *StatusesHandler) POSTStatuses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
		return
	}

	req, err := parseCreateStatusRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	_, user, err := h.deps.Accounts.GetAccountWithUser(ctx, account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
			return
		}
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}

	defaultVisibility := ""
	if user != nil {
		defaultVisibility = user.DefaultPrivacy
	}

	result, err := h.deps.Statuses.CreateWithContent(ctx, service.CreateWithContentInput{
		AccountID:         account.ID,
		Username:          account.Username,
		Text:              req.Status,
		Visibility:        req.Visibility,
		DefaultVisibility: defaultVisibility,
		ContentWarning:    req.SpoilerText,
		Language:          req.Language,
		Sensitive:         req.Sensitive,
	})
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}

	out := createResultToAPIModel(result, h.deps.InstanceDomain)
	api.WriteJSON(w, http.StatusOK, out)
}

// GETStatuses handles GET /api/v1/statuses/:id. Auth optional.
func (h *StatusesHandler) GETStatuses(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		api.WriteError(w, http.StatusNotFound, "Record not found")
		return
	}
	result, err := h.deps.Statuses.GetByIDEnriched(r.Context(), id)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	out := createResultToAPIModel(result, h.deps.InstanceDomain)
	api.WriteJSON(w, http.StatusOK, out)
}

// DELETEStatuses handles DELETE /api/v1/statuses/:id. Auth required.
func (h *StatusesHandler) DELETEStatuses(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.WriteError(w, http.StatusNotFound, "Record not found")
		return
	}
	st, err := h.deps.Statuses.GetByID(r.Context(), id)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	if st.AccountID != account.ID {
		api.WriteError(w, http.StatusForbidden, "This action is not allowed")
		return
	}
	result, err := h.deps.Statuses.GetByIDEnriched(r.Context(), id)
	if err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	if err := h.deps.Statuses.Delete(r.Context(), id); err != nil {
		api.HandleError(w, r, h.deps.Logger, err)
		return
	}
	out := createResultToAPIModel(result, h.deps.InstanceDomain)
	api.WriteJSON(w, http.StatusOK, out)
}

// parseCreateStatusRequest parses JSON or form body into CreateStatusRequest.
// Returns an error with a client-safe message on validation or parse failure.
func parseCreateStatusRequest(r *http.Request) (CreateStatusRequest, error) {
	var req CreateStatusRequest
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return CreateStatusRequest{}, errors.New("invalid JSON")
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return CreateStatusRequest{}, errors.New("invalid form")
		}
		req.Status = r.FormValue("status")
		req.Visibility = r.FormValue("visibility")
		req.SpoilerText = r.FormValue("spoiler_text")
		req.Sensitive = r.FormValue("sensitive") == "true" || r.FormValue("sensitive") == "1"
		req.Language = r.FormValue("language")
	}
	req.Status = strings.TrimSpace(req.Status)
	if req.Status == "" {
		return CreateStatusRequest{}, errors.New("status cannot be blank")
	}
	return req, nil
}

// createResultToAPIModel maps service.CreateResult to apimodel.Status.
func createResultToAPIModel(result service.CreateResult, instanceDomain string) apimodel.Status {
	authorAcc := apimodel.ToAccount(result.Author, instanceDomain)
	mentionsResp := make([]apimodel.Mention, 0, len(result.Mentions))
	for _, a := range result.Mentions {
		mentionsResp = append(mentionsResp, apimodel.MentionFromAccount(a, instanceDomain))
	}
	tagsResp := make([]apimodel.Tag, 0, len(result.Tags))
	for _, t := range result.Tags {
		tagsResp = append(tagsResp, apimodel.TagFromName(t.Name, instanceDomain))
	}
	mediaResp := make([]apimodel.MediaAttachment, 0, len(result.Media))
	for i := range result.Media {
		mediaResp = append(mediaResp, apimodel.MediaFromDomain(&result.Media[i]))
	}
	return apimodel.ToStatus(result.Status, authorAcc, mentionsResp, tagsResp, mediaResp, instanceDomain)
}
