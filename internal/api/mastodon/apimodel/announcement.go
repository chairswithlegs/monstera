package apimodel

import (
	"time"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// ToAnnouncement converts a domain announcement with read state and reactions to the API shape.
func ToAnnouncement(a domain.Announcement, read bool, reactions []domain.AnnouncementReactionCount) Announcement {
	out := Announcement{
		ID:          a.ID,
		Content:     a.Content,
		AllDay:      a.AllDay,
		PublishedAt: a.PublishedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   a.UpdatedAt.UTC().Format(time.RFC3339),
		Read:        read,
		Mentions:    []AccountRef{},
		Statuses:    []StatusRef{},
		Tags:        []Tag{},
		Emojis:      []any{},
		Reactions:   make([]Reaction, 0, len(reactions)),
	}
	if a.StartsAt != nil {
		s := a.StartsAt.UTC().Format(time.RFC3339)
		out.StartsAt = &s
	}
	if a.EndsAt != nil {
		e := a.EndsAt.UTC().Format(time.RFC3339)
		out.EndsAt = &e
	}
	for _, r := range reactions {
		out.Reactions = append(out.Reactions, Reaction{
			Name:  r.Name,
			Count: r.Count,
			Me:    r.Me,
		})
	}
	return out
}
