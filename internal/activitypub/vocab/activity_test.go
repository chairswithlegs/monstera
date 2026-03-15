package vocab

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAcceptActivity(t *testing.T) {
	inner, err := NewFollowActivity("https://example.com/activities/follow-1", "https://remote.example/users/bob", "https://example.com/users/alice")
	require.NoError(t, err)
	accept, err := NewAcceptActivity("https://example.com/activities/accept-1", "https://example.com/users/alice", inner)
	require.NoError(t, err)
	require.Equal(t, ObjectTypeAccept, accept.Type)
	require.Equal(t, "https://example.com/activities/accept-1", accept.ID)
	require.Equal(t, "https://example.com/users/alice", accept.Actor)
	require.NotEmpty(t, accept.ObjectRaw)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(accept.ObjectRaw, &decoded))
	require.Equal(t, "Follow", decoded["type"])
}

func TestNewDeleteActivity(t *testing.T) {
	act, err := NewDeleteActivity("https://example.com/activities/del-1", "https://example.com/users/alice", "https://example.com/statuses/123")
	require.NoError(t, err)
	require.Equal(t, ObjectTypeDelete, act.Type)
	require.Equal(t, "https://example.com/activities/del-1", act.ID)
	require.Equal(t, "https://example.com/users/alice", act.Actor)
	var obj map[string]any
	require.NoError(t, json.Unmarshal(act.ObjectRaw, &obj))
	require.Equal(t, "Tombstone", obj["type"])
	require.Equal(t, "https://example.com/statuses/123", obj["id"])
}

func TestNewFollowActivity(t *testing.T) {
	act, err := NewFollowActivity("https://example.com/activities/f-1", "https://a.com/users/alice", "https://b.com/users/bob")
	require.NoError(t, err)
	require.Equal(t, ObjectTypeFollow, act.Type)
	require.Equal(t, "https://example.com/activities/f-1", act.ID)
	require.Equal(t, "https://a.com/users/alice", act.Actor)
	var obj string
	require.NoError(t, json.Unmarshal(act.ObjectRaw, &obj))
	require.Equal(t, "https://b.com/users/bob", obj)
}

func TestNewUndoActivity(t *testing.T) {
	inner, _ := NewFollowActivity("https://example.com/f", "https://a.com/users/alice", "https://b.com/users/bob")
	act, err := NewUndoActivity("https://example.com/undo-1", "https://a.com/users/alice", inner)
	require.NoError(t, err)
	require.Equal(t, ObjectTypeUndo, act.Type)
	require.Equal(t, "https://example.com/undo-1", act.ID)
	require.Equal(t, "https://a.com/users/alice", act.Actor)
	innerDecoded, err := act.ObjectActivity()
	require.NoError(t, err)
	require.Equal(t, ObjectTypeFollow, innerDecoded.Type)
}

func TestNewCreateNoteActivity(t *testing.T) {
	note := &Note{
		ID:           "https://example.com/statuses/1",
		Type:         ObjectTypeNote,
		AttributedTo: "https://example.com/users/alice",
		Content:      "Hello",
		To:           []string{PublicAddress},
		Published:    "2026-02-25T12:00:00Z",
		URL:          "https://example.com/statuses/1",
	}
	act, err := NewCreateNoteActivity("https://example.com/activities/create-1", note)
	require.NoError(t, err)
	require.Equal(t, ObjectTypeCreate, act.Type)
	require.Equal(t, "https://example.com/activities/create-1", act.ID)
	require.Equal(t, "https://example.com/users/alice", act.Actor)
	var obj map[string]any
	require.NoError(t, json.Unmarshal(act.ObjectRaw, &obj))
	require.Equal(t, "Note", obj["type"])
	require.Equal(t, "Hello", obj["content"])
}

func TestNewUpdateNoteActivity(t *testing.T) {
	note := &Note{
		ID:           "https://example.com/statuses/1",
		Type:         ObjectTypeNote,
		AttributedTo: "https://example.com/users/alice",
		Content:      "Updated",
		To:           []string{PublicAddress},
		Published:    "2026-02-25T12:00:00Z",
		URL:          "https://example.com/statuses/1",
	}
	act, err := NewUpdateNoteActivity("https://example.com/activities/update-1", "https://example.com/users/alice", note)
	require.NoError(t, err)
	require.Equal(t, ObjectTypeUpdate, act.Type)
	require.Equal(t, "https://example.com/activities/update-1", act.ID)
	require.Equal(t, "https://example.com/users/alice", act.Actor)
	var obj map[string]any
	require.NoError(t, json.Unmarshal(act.ObjectRaw, &obj))
	require.Equal(t, "Updated", obj["content"])
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
	require.Equal(t, ObjectTypeNote, note.Type)
	require.Equal(t, "Hello", note.Content)
	require.Equal(t, "https://example.com/notes/1", note.ID)
}

func TestActivity_ObjectType(t *testing.T) {
	raw := `{"id":"x","type":"Create","actor":"https://example.com/users/alice","object":{"type":"Note","id":"https://example.com/notes/1"}}`
	var a Activity
	err := json.Unmarshal([]byte(raw), &a)
	require.NoError(t, err)
	require.Equal(t, ObjectTypeNote, a.ObjectType())
}

func TestActivity_ObjectActivity(t *testing.T) {
	inner := &Activity{Type: ObjectTypeFollow, ID: "https://ex.com/f1", Actor: "https://a.com/u", ObjectRaw: json.RawMessage(`"https://b.com/u"`)}
	raw, _ := json.Marshal(inner)
	outer := &Activity{Type: ObjectTypeAccept, ID: "https://ex.com/a1", Actor: "https://b.com/u", ObjectRaw: raw}
	decoded, err := outer.ObjectActivity()
	require.NoError(t, err)
	require.Equal(t, ObjectTypeFollow, decoded.Type)
	require.Equal(t, "https://ex.com/f1", decoded.ID)
}

func TestActivity_ObjectActor(t *testing.T) {
	actorJSON := `{"type":"Person","id":"https://example.com/users/bob","preferredUsername":"bob","name":"Bob"}`
	a := &Activity{Type: ObjectTypeUpdate, ObjectRaw: json.RawMessage(actorJSON)}
	actor, err := a.ObjectActor()
	require.NoError(t, err)
	require.Equal(t, ObjectTypePerson, actor.Type)
	require.Equal(t, "https://example.com/users/bob", actor.ID)
	require.Equal(t, "bob", actor.PreferredUsername)
	require.Equal(t, "Bob", actor.Name)
}
