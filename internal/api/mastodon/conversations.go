package mastodon

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/api/mastodon/apimodel"
	"github.com/chairswithlegs/monstera/internal/api/middleware"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/service"
)

// ConversationsHandler handles Mastodon conversations API endpoints.
type ConversationsHandler struct {
	conversations  service.ConversationService
	instanceDomain string
}

// NewConversationsHandler returns a new ConversationsHandler.
func NewConversationsHandler(conversations service.ConversationService, instanceDomain string) *ConversationsHandler {
	return &ConversationsHandler{
		conversations:  conversations,
		instanceDomain: instanceDomain,
	}
}

// GETConversations handles GET /api/v1/conversations.
func (h *ConversationsHandler) GETConversations(w http.ResponseWriter, r *http.Request) {
	account := middleware.AccountFromContext(r.Context())
	if account == nil {
		api.HandleError(w, r, api.ErrUnauthorized)
		return
	}
	params := PageParamsFromRequest(r)
	limit := parseLimitParam(r, DefaultListLimit, MaxListLimit)
	results, nextCursor, err := h.conversations.ListConversations(r.Context(), account.ID, optionalString(params.MaxID), limit)
	if err != nil {
		api.HandleError(w, r, err)
		return
	}
	out := make([]apimodel.Conversation, 0, len(results))
	for _, res := range results {
		accounts := make([]apimodel.Account, 0, len(res.Participants))
		for _, a := range res.Participants {
			if a != nil {
				accounts = append(accounts, apimodel.ToAccount(a, h.instanceDomain))
			}
		}
		var lastStatus *apimodel.Status
		if res.LastStatus != nil {
			s := apimodel.StatusFromEnriched(*res.LastStatus, h.instanceDomain)
			lastStatus = &s
		}
		out = append(out, apimodel.ToConversation(res.AccountConversation.ConversationID, res.AccountConversation.Unread, accounts, lastStatus))
	}
	if nextCursor != nil && *nextCursor != "" {
		if link := linkHeaderWithNext(AbsoluteRequestURL(r, h.instanceDomain), *nextCursor); link != "" {
			w.Header().Set("Link", link)
		}
	}
	api.WriteJSON(w, http.StatusOK, out)
}

// DELETEConversation handles DELETE /api/v1/conversations/:id.
func (h *ConversationsHandler) DELETEConversation(w http.ResponseWriter, r *http.Request) {
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
	if err := h.conversations.RemoveConversation(r.Context(), account.ID, id); err != nil {
		api.HandleError(w, r, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]any{})
}

// POSTConversationRead handles POST /api/v1/conversations/:id/read.
func (h *ConversationsHandler) POSTConversationRead(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.conversations.MarkConversationRead(r.Context(), account.ID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			api.HandleError(w, r, api.ErrNotFound)
			return
		}
		api.HandleError(w, r, err)
		return
	}
	accounts := make([]apimodel.Account, 0, len(result.Participants))
	for _, a := range result.Participants {
		if a != nil {
			accounts = append(accounts, apimodel.ToAccount(a, h.instanceDomain))
		}
	}
	var lastStatus *apimodel.Status
	if result.LastStatus != nil {
		s := apimodel.StatusFromEnriched(*result.LastStatus, h.instanceDomain)
		lastStatus = &s
	}
	api.WriteJSON(w, http.StatusOK, apimodel.ToConversation(result.AccountConversation.ConversationID, result.AccountConversation.Unread, accounts, lastStatus))
}
