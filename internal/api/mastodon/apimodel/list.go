package apimodel

import (
	"github.com/chairswithlegs/monstera/internal/domain"
)

// List is the Mastodon API list entity.
type List struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	RepliesPolicy string `json:"replies_policy"`
	Exclusive     bool   `json:"exclusive"`
}

// ToList converts a domain list to the Mastodon API List shape.
func ToList(l *domain.List) List {
	return List{
		ID:            l.ID,
		Title:         l.Title,
		RepliesPolicy: l.RepliesPolicy,
		Exclusive:     l.Exclusive,
	}
}
