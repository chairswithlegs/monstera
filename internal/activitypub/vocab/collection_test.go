package vocab

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderedCollection_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	coll := NewOrderedCollection("https://example.com/users/alice/outbox", 42)
	coll.First = "https://example.com/users/alice/outbox?page=1"
	data, err := json.Marshal(coll)
	require.NoError(t, err)
	var decoded OrderedCollection
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, coll.ID, decoded.ID)
	assert.Equal(t, coll.TotalItems, decoded.TotalItems)
	assert.Equal(t, coll.First, decoded.First)
}

func TestNewOrderedCollection(t *testing.T) {
	t.Parallel()
	coll := NewOrderedCollection("https://example.com/users/alice/followers", 10)
	assert.Equal(t, DefaultContext, coll.Context)
	assert.Equal(t, "https://example.com/users/alice/followers", coll.ID)
	assert.Equal(t, ObjectTypeOrderedCollection, coll.Type)
	assert.Equal(t, 10, coll.TotalItems)
	assert.Nil(t, coll.OrderedItems)
	assert.Empty(t, coll.First)
}

func TestNewOrderedCollectionWithItems(t *testing.T) {
	t.Parallel()
	items := []json.RawMessage{json.RawMessage(`{"id":"1"}`), json.RawMessage(`{"id":"2"}`)}
	coll := NewOrderedCollectionWithItems("https://example.com/users/alice/collections/featured", items)
	assert.Equal(t, ObjectTypeOrderedCollection, coll.Type)
	assert.Equal(t, 2, coll.TotalItems)
	assert.Equal(t, items, coll.OrderedItems)
}

func TestNewOrderedCollectionPage(t *testing.T) {
	t.Parallel()
	items := []json.RawMessage{json.RawMessage(`{"id":"1"}`)}
	page := NewOrderedCollectionPage(
		"https://example.com/users/alice/outbox?page=true",
		"https://example.com/users/alice/outbox",
		items,
	)
	assert.Equal(t, DefaultContext, page.Context)
	assert.Equal(t, ObjectTypeOrderedCollectionPage, page.Type)
	assert.Equal(t, 1, page.TotalItems)
	assert.Equal(t, "https://example.com/users/alice/outbox", page.PartOf)
	assert.Equal(t, items, page.OrderedItems)
	assert.Empty(t, page.Next)

	page.Next = "https://example.com/users/alice/outbox?page=true&max_id=xyz"
	assert.Equal(t, "https://example.com/users/alice/outbox?page=true&max_id=xyz", page.Next)
}
