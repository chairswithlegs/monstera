package mastodon

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/api"
	"github.com/chairswithlegs/monstera-fed/internal/api/mastodon/presenter"
	"github.com/chairswithlegs/monstera-fed/internal/api/middleware"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/service"
)

// StatusesHandler handles status-related Mastodon API endpoints.
type StatusesHandler struct {
	statuses *service.StatusService
	accounts *service.AccountService
	logger   *slog.Logger
	domain   string
}

// NewStatusesHandler returns a new StatusesHandler.
func NewStatusesHandler(statuses *service.StatusService, accounts *service.AccountService, logger *slog.Logger, instanceDomain string) *StatusesHandler {
	return &StatusesHandler{
		statuses: statuses,
		accounts: accounts,
		logger:   logger,
		domain:   instanceDomain,
	}
}

// CreateStatusRequest is the request body for POST /api/v1/statuses.
type CreateStatusRequest struct {
	Status      string `json:"status"`
	Visibility  string `json:"visibility"`
	SpoilerText string `json:"spoiler_text"`
	Sensitive   bool   `json:"sensitive"`
	Language    string `json:"language"`
}

// Create handles POST /api/v1/statuses.
func (h *StatusesHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	_, user, err := h.accounts.GetAccountWithUser(ctx, account.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.WriteError(w, http.StatusUnauthorized, "The access token is invalid")
			return
		}
		api.HandleError(w, r, h.logger, err)
		return
	}

	defaultVisibility := ""
	if user != nil {
		defaultVisibility = user.DefaultPrivacy
	}

	result, err := h.statuses.CreateWithContent(ctx, service.CreateWithContentInput{
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
		if errors.Is(err, domain.ErrValidation) {
			api.WriteError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		api.HandleError(w, r, h.logger, err)
		return
	}

	out := createResultToPresenter(result, h.domain)
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

// createResultToPresenter maps service.CreateResult to presenter.Status.
func createResultToPresenter(result service.CreateResult, instanceDomain string) presenter.Status {
	authorAcc := presenter.ToAccount(result.Author, instanceDomain)
	mentionsResp := make([]presenter.Mention, 0, len(result.Mentions))
	for _, a := range result.Mentions {
		mentionsResp = append(mentionsResp, presenter.MentionFromAccount(a, instanceDomain))
	}
	tagsResp := make([]presenter.Tag, 0, len(result.Tags))
	for _, t := range result.Tags {
		tagsResp = append(tagsResp, presenter.TagFromName(t.Name, instanceDomain))
	}
	mediaResp := make([]presenter.MediaAttachment, 0, len(result.Media))
	for i := range result.Media {
		mediaResp = append(mediaResp, presenter.MediaFromDomain(&result.Media[i]))
	}
	return presenter.ToStatus(result.Status, authorAcc, mentionsResp, tagsResp, mediaResp, instanceDomain)
}
