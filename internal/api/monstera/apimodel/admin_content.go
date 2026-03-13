package apimodel

import (
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/microcosm-cc/bluemonday"
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

type PostServerFilterRequest struct {
	Phrase    string `json:"phrase"`
	Scope     string `json:"scope"`
	Action    string `json:"action"`
	WholeWord bool   `json:"whole_word"`
}

func (r *PostServerFilterRequest) Sanitize() {
	r.Phrase = bluemonday.StrictPolicy().Sanitize(r.Phrase)
}

func (r *PostServerFilterRequest) Validate() error {
	if err := api.ValidateRequiredField(r.Phrase, "phrase"); err != nil {
		return fmt.Errorf("phrase: %w", err)
	}
	if r.Scope == "" {
		r.Scope = domain.ServerFilterScopeAll
	}
	if r.Action == "" {
		r.Action = domain.ServerFilterActionHide
	}
	return nil
}

type PutServerFilterRequest struct {
	Phrase    string `json:"phrase"`
	Scope     string `json:"scope"`
	Action    string `json:"action"`
	WholeWord bool   `json:"whole_word"`
}

func (r *PutServerFilterRequest) Sanitize() {
	r.Phrase = bluemonday.StrictPolicy().Sanitize(r.Phrase)
}

func (r *PutServerFilterRequest) Validate() error {
	if err := api.ValidateRequiredField(r.Phrase, "phrase"); err != nil {
		return fmt.Errorf("phrase: %w", err)
	}
	if r.Scope == "" {
		r.Scope = domain.ServerFilterScopeAll
	}
	if r.Action == "" {
		r.Action = domain.ServerFilterActionHide
	}
	return nil
}
