package vocab

import (
	"encoding/json"
	"fmt"
)

// Activity is the generic AP Activity wrapper. Used for Follow, Like, Announce,
// Create, Delete, Update, Undo, Accept, Reject, and Block.
//
// The ObjectRaw field holds the polymorphic "object" — it can be a string
// (IRI reference) or an embedded JSON object (e.g. a full Note). Callers
// use the accessor methods to decode it.
type Activity struct {
	Context   any             `json:"@context,omitempty"`
	Type      ObjectType      `json:"type"`
	ObjectRaw json.RawMessage `json:"object"`
	ID        string          `json:"id"`           // Activity IRI
	Actor     string          `json:"actor"`        // Actor IRI
	To        []string        `json:"to,omitempty"` // list of target actor IRIs
	Cc        []string        `json:"cc,omitempty"` // list of CC actor IRIs
	Published string          `json:"published,omitempty"`
}

// ObjectID returns the object field as a plain IRI string.
// Returns ("", false) if the object is an embedded JSON object rather than a string.
func (a *Activity) ObjectID() (string, bool) {
	var id string
	if err := json.Unmarshal(a.ObjectRaw, &id); err != nil {
		return "", false
	}
	return id, true
}

// ObjectType peeks at the "type" field of the embedded object without fully
// unmarshalling it. Returns "" if the object is a plain string IRI.
func (a *Activity) ObjectType() ObjectType {
	var peek struct {
		Type ObjectType `json:"type"`
	}
	if err := json.Unmarshal(a.ObjectRaw, &peek); err != nil {
		return ObjectType("")
	}
	return peek.Type
}

// ObjectActivity unmarshals the object field as an embedded Activity.
// Used for Undo{Follow}, Accept{Follow}, Reject{Follow} where the object
// is the original Follow activity.
func (a *Activity) ObjectActivity() (*Activity, error) {
	var inner Activity
	if err := json.Unmarshal(a.ObjectRaw, &inner); err != nil {
		return nil, fmt.Errorf("object Activity: %w", err)
	}
	return &inner, nil
}

// ObjectNote unmarshals the object field as a Note.
// Used for Create{Note} and Update{Note}.
func (a *Activity) ObjectNote() (*Note, error) {
	var note Note
	if err := json.Unmarshal(a.ObjectRaw, &note); err != nil {
		return nil, fmt.Errorf("object Note: %w", err)
	}
	return &note, nil
}

// ObjectActor unmarshals the object field as an Actor.
// Used for Update{Person}.
func (a *Activity) ObjectActor() (*Actor, error) {
	var actor Actor
	if err := json.Unmarshal(a.ObjectRaw, &actor); err != nil {
		return nil, fmt.Errorf("object Actor: %w", err)
	}
	return &actor, nil
}

// NewDeleteActivity constructs a Delete activity with a Tombstone object.
func NewDeleteActivity(activityID, actorID, objectID string) (*Activity, error) {
	tombstone := struct {
		ID   string     `json:"id"`
		Type ObjectType `json:"type"`
	}{
		ID:   objectID,
		Type: ObjectTypeTombstone,
	}
	raw, err := json.Marshal(tombstone)
	if err != nil {
		return nil, fmt.Errorf("marshal tombstone: %w", err)
	}
	return &Activity{
		Context:   DefaultContext,
		ID:        activityID,
		Type:      ObjectTypeDelete,
		Actor:     actorID,
		ObjectRaw: raw,
	}, nil
}

// NewFollowActivity constructs a Follow activity. objectID is the target actor IRI.
func NewFollowActivity(activityID, actorID, objectID string) (*Activity, error) {
	objRaw, err := json.Marshal(objectID)
	if err != nil {
		return nil, fmt.Errorf("marshal follow object: %w", err)
	}
	return &Activity{
		Context:   DefaultContext,
		ID:        activityID,
		Type:      ObjectTypeFollow,
		Actor:     actorID,
		ObjectRaw: objRaw,
	}, nil
}

// NewUndoActivity constructs an Undo activity wrapping the given inner activity.
func NewUndoActivity(activityID, actorID string, inner *Activity) (*Activity, error) {
	raw, err := json.Marshal(inner)
	if err != nil {
		return nil, fmt.Errorf("marshal undo object: %w", err)
	}
	return &Activity{
		Context:   DefaultContext,
		ID:        activityID,
		Type:      ObjectTypeUndo,
		Actor:     actorID,
		ObjectRaw: raw,
	}, nil
}

// NewAcceptActivity wraps an inner activity (typically Follow) in an Accept.
func NewAcceptActivity(activityID, actorID string, inner *Activity) (*Activity, error) {
	raw, err := json.Marshal(inner)
	if err != nil {
		return nil, fmt.Errorf("marshal accept object: %w", err)
	}
	return &Activity{
		Context:   DefaultContext,
		ID:        activityID,
		Type:      ObjectTypeAccept,
		Actor:     actorID,
		ObjectRaw: raw,
	}, nil
}

// NewBlockActivity constructs a Block activity. objectID is the target actor IRI.
func NewBlockActivity(activityID, actorID, objectID string) (*Activity, error) {
	objRaw, err := json.Marshal(objectID)
	if err != nil {
		return nil, fmt.Errorf("marshal block object: %w", err)
	}
	return &Activity{
		Context:   DefaultContext,
		ID:        activityID,
		Type:      ObjectTypeBlock,
		Actor:     actorID,
		ObjectRaw: objRaw,
	}, nil
}

// NewCreateNoteActivity wraps a Note in a Create activity with the given activity ID.
func NewCreateNoteActivity(activityID string, note *Note) (*Activity, error) {
	raw, err := json.Marshal(note)
	if err != nil {
		return nil, fmt.Errorf("marshal note: %w", err)
	}
	return &Activity{
		Context:   DefaultContext,
		ID:        activityID,
		Type:      ObjectTypeCreate,
		Actor:     note.AttributedTo,
		ObjectRaw: raw,
		To:        note.To,
		Cc:        note.Cc,
		Published: note.Published,
	}, nil
}

// NewUpdateNoteActivity wraps a Note in an Update activity for federation.
func NewUpdateNoteActivity(activityID, actorID string, note *Note) (*Activity, error) {
	raw, err := json.Marshal(note)
	if err != nil {
		return nil, fmt.Errorf("marshal note: %w", err)
	}
	return &Activity{
		Context:   DefaultContext,
		ID:        activityID,
		Type:      ObjectTypeUpdate,
		Actor:     actorID,
		ObjectRaw: raw,
	}, nil
}

// NewUpdateActorActivity wraps an Actor in an Update activity for federation.
func NewUpdateActorActivity(activityID, actorID string, actor *Actor) (*Activity, error) {
	raw, err := json.Marshal(actor)
	if err != nil {
		return nil, fmt.Errorf("marshal actor: %w", err)
	}
	return &Activity{
		Context:   DefaultContext,
		ID:        activityID,
		Type:      ObjectTypeUpdate,
		Actor:     actorID,
		ObjectRaw: raw,
	}, nil
}
