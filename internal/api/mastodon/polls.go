package mastodon

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// PollsHandler handles GET /api/v1/polls/:id and POST /api/v1/polls/:id/votes.
type PollsHandler struct {
	statuses     service.StatusService
	statusWrites service.StatusWriteService
}

// NewPollsHandler returns a new PollsHandler.
func NewPollsHandler(statuses service.StatusService, statusWrites service.StatusWriteService) *PollsHandler {
	return &PollsHandler{statuses: statuses, statusWrites: statusWrites}
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
	api.WriteJSON(w, http.StatusOK, enrichedPollToAPIModel(poll))
}

// POSTVotesRequest is the body for POST /api/v1/polls/:id/votes.
type POSTVotesRequest struct {
	Choices []int `json:"choices"`
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
	if err := api.DecodeJSONBody(r, &req); err != nil {
		api.HandleError(w, r, err)
		return
	}
	poll, err := h.statusWrites.RecordVote(ctx, id, account.ID, req.Choices)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		if errors.Is(err, domain.ErrUnprocessable) {
			api.HandleError(w, r, api.NewUnprocessableError("The poll has already ended"))
			return
		}
		if errors.Is(err, domain.ErrValidation) {
			api.HandleError(w, r, api.NewUnprocessableError("Validation failed: invalid choice or already voted"))
			return
		}
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, enrichedPollToAPIModel(poll))
}
