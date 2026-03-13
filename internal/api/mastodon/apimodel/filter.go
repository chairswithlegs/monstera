package apimodel

import (
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// Filter is the Mastodon API v1 filter entity.
type Filter struct {
	ID           string   `json:"id"`
	Phrase       string   `json:"phrase"`
	Context      []string `json:"context"`
	WholeWord    bool     `json:"whole_word"`
	ExpiresAt    *string  `json:"expires_at,omitempty"`
	Irreversible bool     `json:"irreversible"`
}

// ToFilter converts a domain user filter to the Mastodon API Filter shape.
func ToFilter(f *domain.UserFilter) Filter {
	out := Filter{
		ID:           f.ID,
		Phrase:       f.Phrase,
		Context:      f.Context,
		WholeWord:    f.WholeWord,
		Irreversible: f.Irreversible,
	}
	if f.ExpiresAt != nil {
		s := f.ExpiresAt.UTC().Format(time.RFC3339)
		out.ExpiresAt = &s
	}
	return out
}
