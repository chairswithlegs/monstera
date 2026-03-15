package vocab

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActor_JSONRoundTrip(t *testing.T) {
	actor := Actor{
		Context:           DefaultContext,
		ID:                "https://example.com/users/alice",
		Type:              ObjectTypePerson,
		PreferredUsername: "alice",
		Name:              "Alice",
		URL:               "https://example.com/@alice",
		Inbox:             "https://example.com/users/alice/inbox",
		Outbox:            "https://example.com/users/alice/outbox",
		Followers:         "https://example.com/users/alice/followers",
		Following:         "https://example.com/users/alice/following",
		PublicKey: PublicKey{
			ID:           "https://example.com/users/alice#main-key",
			Owner:        "https://example.com/users/alice",
			PublicKeyPem: "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----",
		},
		ManuallyApprovesFollowers: false,
	}
	data, err := json.Marshal(actor)
	require.NoError(t, err)
	var decoded Actor
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	require.Equal(t, actor.ID, decoded.ID)
	require.Equal(t, actor.PreferredUsername, decoded.PreferredUsername)
	require.Equal(t, actor.PublicKey.ID, decoded.PublicKey.ID)
}

func TestNote_JSONRoundTrip(t *testing.T) {
	note := Note{
		Context:      DefaultContext,
		ID:           "https://example.com/statuses/1",
		Type:         "Note",
		AttributedTo: "https://example.com/users/alice",
		Content:      "<p>Hello world</p>",
		To:           []string{PublicAddress},
		Published:    "2026-02-25T12:00:00Z",
		URL:          "https://example.com/statuses/1",
		Sensitive:    false,
	}
	data, err := json.Marshal(note)
	require.NoError(t, err)
	var decoded Note
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	require.Equal(t, note.ID, decoded.ID)
	require.Equal(t, note.Content, decoded.Content)
	require.Equal(t, note.AttributedTo, decoded.AttributedTo)
}

func TestOrderedCollection_JSONRoundTrip(t *testing.T) {
	coll := OrderedCollection{
		Context:    DefaultContext,
		ID:         "https://example.com/users/alice/outbox",
		Type:       "OrderedCollection",
		TotalItems: 42,
		First:      "https://example.com/users/alice/outbox?page=1",
	}
	data, err := json.Marshal(coll)
	require.NoError(t, err)
	var decoded OrderedCollection
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	require.Equal(t, coll.ID, decoded.ID)
	require.Equal(t, coll.TotalItems, decoded.TotalItems)
	require.Equal(t, coll.First, decoded.First)
}
