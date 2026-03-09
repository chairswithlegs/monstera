package domain

import "time"

// AccountConversation is the per-user view of a direct-message conversation.
// One row per (account_id, conversation_id) in account_conversations.
type AccountConversation struct {
	ID             string
	AccountID      string
	ConversationID string
	LastStatusID   *string
	Unread         bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
