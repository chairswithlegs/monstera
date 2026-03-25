package mastodon

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// PollsHandler handles GET /api/v1/polls/:id and POST /api/v1/polls/:id/votes.
type PollsHandler struct {
	statuses     service.StatusService
	interactions service.StatusInteractionService
}

// NewPollsHandler returns a new PollsHandler.
func NewPollsHandler(statuses service.StatusService, interactions service.StatusInteractionService) *PollsHandler {
	return &PollsHandler{statuses: statuses, interactions: interactions}
}

// GETPoll handles GET /api/v1/polls/:id. Optional auth; when authenticated, voted and own_votes are set.
func (h *PollsHandler) GETPoll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var viewerID *string
	if account := middleware.AccountFromContext(ctx); account != nil {
		viewerID = &account.ID
	}
	poll, err := h.statuses.GetPoll(ctx, id, viewerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.PollFromEnriched(poll))
}

// POSTVotesRequest is the body for POST /api/v1/polls/:id/votes.
type POSTVotesRequest struct {
	Choices []int `json:"choices"`
}

func (r *POSTVotesRequest) Validate() error {
	if len(r.Choices) == 0 {
		return fmt.Errorf("validate votes: %w", api.NewMissingRequiredFieldError("choices"))
	}
	seen := make(map[int]struct{}, len(r.Choices))
	for _, c := range r.Choices {
		if c < 0 {
			return fmt.Errorf("validate votes: %w", api.NewInvalidValueError("choices"))
		}
		if _, dup := seen[c]; dup {
			return fmt.Errorf("validate votes: %w", api.NewInvalidValueError("choices"))
		}
		seen[c] = struct{}{}
	}
	return nil
}

// POSTVotes handles POST /api/v1/polls/:id/votes.
func (h *PollsHandler) POSTVotes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	account := middleware.AccountFromContext(ctx)
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		api.HandleError(w, r, api.ErrNotFound)
		return
	}
	var req POSTVotesRequest
	if err := api.DecodeAndValidateJSON(r, &req); err != nil {
		api.HandleError(w, r, err)
		return
	}
	poll, err := h.interactions.RecordVote(ctx, id, account.ID, req.Choices)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrUnprocessable) {
			api.HandleError(w, r, api.NewPollEndedError())
			return
		}
		if errors.Is(err, domain.ErrValidation) {
			api.HandleError(w, r, api.NewInvalidValueError("choices"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, apimodel.PollFromEnriched(poll))
}
