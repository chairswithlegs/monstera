package apimodel

import (
	"time"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
)

// AdminServerFilter is one server filter in the admin API.
type AdminServerFilter struct {
	ID        string    `json:"id"`
	Phrase    string    `json:"phrase"`
	Scope     string    `json:"scope"`
	Action    string    `json:"action"`
	WholeWord bool      `json:"whole_word"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToAdminServerFilter converts a domain server filter to the admin API shape.
func ToAdminServerFilter(f *domain.ServerFilter) AdminServerFilter {
	return AdminServerFilter{
		ID:        f.ID,
		Phrase:    f.Phrase,
		Scope:     f.Scope,
		Action:    f.Action,
		WholeWord: f.WholeWord,
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,
	}
}

// AdminServerFilterList is the response for GET /admin/content/filters.
type AdminServerFilterList struct {
	Filters []AdminServerFilter `json:"filters"`
}
