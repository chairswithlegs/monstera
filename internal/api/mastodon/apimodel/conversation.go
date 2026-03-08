package apimodel

// ToConversation builds a Mastodon API Conversation from its parts.
// id is the conversation ID (account_conversations.conversation_id), unread from the row,
// accounts are the other participants (excluding the viewer), lastStatus may be nil.
func ToConversation(id string, unread bool, accounts []Account, lastStatus *Status) Conversation {
	c := Conversation{
		ID:       id,
		Unread:   unread,
		Accounts: accounts,
	}
	if lastStatus != nil {
		c.LastStatus = lastStatus
	}
	return c
}
