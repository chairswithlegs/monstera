package vocab

import "encoding/json"

// OrderedCollection represents an AP OrderedCollection.
// Used for outbox, followers, following, and featured endpoints.
// When OrderedItems is non-nil, it is serialized (inline items); otherwise First may point to a page.
type OrderedCollection struct {
	Context      any               `json:"@context,omitempty"`
	ID           string            `json:"id"`
	Type         ObjectType        `json:"type"` // "OrderedCollection"
	TotalItems   int               `json:"totalItems"`
	First        string            `json:"first,omitempty"`        // URL of first page
	OrderedItems []json.RawMessage `json:"orderedItems,omitempty"` // inline items when present
}

// OrderedCollectionPage represents a page within an OrderedCollection.
type OrderedCollectionPage struct {
	Context      any               `json:"@context,omitempty"`
	ID           string            `json:"id"`
	Type         ObjectType        `json:"type"` // "OrderedCollectionPage"
	TotalItems   int               `json:"totalItems"`
	PartOf       string            `json:"partOf"`
	Next         string            `json:"next,omitempty"`
	Prev         string            `json:"prev,omitempty"`
	OrderedItems []json.RawMessage `json:"orderedItems"`
}

// NewOrderedCollection builds an OrderedCollection with a total item count and no inline items.
// Use for endpoints that report counts only (followers, following) or link to a first page (outbox).
// Set the First field on the returned value when a first-page URL is needed.
func NewOrderedCollection(id string, totalItems int) *OrderedCollection {
	return &OrderedCollection{
		Context:    DefaultContext,
		ID:         id,
		Type:       ObjectTypeOrderedCollection,
		TotalItems: totalItems,
	}
}

// NewOrderedCollectionWithItems builds an OrderedCollection with inline ordered items.
// TotalItems is set to the length of items. Use for endpoints that return all items inline (featured).
func NewOrderedCollectionWithItems(id string, items []json.RawMessage) *OrderedCollection {
	return &OrderedCollection{
		Context:      DefaultContext,
		ID:           id,
		Type:         ObjectTypeOrderedCollection,
		TotalItems:   len(items),
		OrderedItems: items,
	}
}

// NewOrderedCollectionPage builds an OrderedCollectionPage.
// TotalItems is set to the length of items. Set Next or Prev on the returned value for pagination.
func NewOrderedCollectionPage(id, partOf string, items []json.RawMessage) *OrderedCollectionPage {
	return &OrderedCollectionPage{
		Context:      DefaultContext,
		ID:           id,
		Type:         ObjectTypeOrderedCollectionPage,
		TotalItems:   len(items),
		PartOf:       partOf,
		OrderedItems: items,
	}
}
