package mastodon

import (
	"net/http"

	"github.com/chairswithlegs/monstera/internal/api"
)

// ConversationsHandler handles Mastodon conversations API endpoints.
type ConversationsHandler struct{}

// NewConversationsHandler returns a new ConversationsHandler.
func NewConversationsHandler() *ConversationsHandler {
	return &ConversationsHandler{}
}

// GETConversations handles GET /api/v1/conversations.
// Returns an empty array until direct-message conversations are implemented.
func (h *ConversationsHandler) GETConversations(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, http.StatusOK, []any{})
}
