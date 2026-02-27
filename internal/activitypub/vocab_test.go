package activitypub

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActor_JSONRoundTrip(t *testing.T) {
	actor := Actor{
		Context:           DefaultContext,
		ID:                "https://example.com/users/alice",
		Type:              "Person",
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

func TestActivity_ObjectID(t *testing.T) {
	// object as string IRI
	raw := `{"id":"https://example.com/activities/1","type":"Follow","actor":"https://example.com/users/alice","object":"https://example.com/users/bob"}`
	var a Activity
	err := json.Unmarshal([]byte(raw), &a)
	require.NoError(t, err)
	id, ok := a.ObjectID()
	require.True(t, ok)
	require.Equal(t, "https://example.com/users/bob", id)
	require.Empty(t, a.ObjectType())
}

func TestActivity_ObjectNote(t *testing.T) {
	raw := `{"id":"https://example.com/activities/1","type":"Create","actor":"https://example.com/users/alice","object":{"type":"Note","id":"https://example.com/notes/1","content":"Hello","attributedTo":"https://example.com/users/alice","to":["https://www.w3.org/ns/activitystreams#Public"],"published":"2026-02-25T12:00:00Z","url":"https://example.com/notes/1"}}`
	var a Activity
	err := json.Unmarshal([]byte(raw), &a)
	require.NoError(t, err)
	note, err := a.ObjectNote()
	require.NoError(t, err)
	require.Equal(t, "Note", note.Type)
	require.Equal(t, "Hello", note.Content)
	require.Equal(t, "https://example.com/notes/1", note.ID)
}

func TestActivity_ObjectType(t *testing.T) {
	raw := `{"id":"x","type":"Create","actor":"https://example.com/users/alice","object":{"type":"Note","id":"https://example.com/notes/1"}}`
	var a Activity
	err := json.Unmarshal([]byte(raw), &a)
	require.NoError(t, err)
	require.Equal(t, "Note", a.ObjectType())
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

func TestDomainFromActorID(t *testing.T) {
	require.Equal(t, "remote.example.com", DomainFromActorID("https://remote.example.com/users/alice"))
	require.Empty(t, DomainFromActorID("not-a-url"))
}

func TestDomainFromKeyID(t *testing.T) {
	require.Equal(t, "remote.example.com", DomainFromKeyID("https://remote.example.com/users/alice#main-key"))
}

func TestNewAcceptActivity(t *testing.T) {
	inner, err := NewFollowActivity("https://example.com/activities/follow-1", "https://remote.example/users/bob", "https://example.com/users/alice")
	require.NoError(t, err)
	accept, err := NewAcceptActivity("https://example.com/activities/accept-1", "https://example.com/users/alice", inner)
	require.NoError(t, err)
	require.Equal(t, "Accept", accept.Type)
	require.Equal(t, "https://example.com/activities/accept-1", accept.ID)
	require.Equal(t, "https://example.com/users/alice", accept.Actor)
	require.NotEmpty(t, accept.ObjectRaw)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(accept.ObjectRaw, &decoded))
	require.Equal(t, "Follow", decoded["type"])
}
