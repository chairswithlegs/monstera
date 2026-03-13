package apimodel

import (
	"fmt"
	"time"

	"github.com/chairswithlegs/monstera/internal/api"
	"github.com/microcosm-cc/bluemonday"
)

// AdminAnnouncement is the admin API announcement shape (list/create/update).
type AdminAnnouncement struct {
	ID          string  `json:"id"`
	Content     string  `json:"content"`
	StartsAt    *string `json:"starts_at,omitempty"`
	EndsAt      *string `json:"ends_at,omitempty"`
	AllDay      bool    `json:"all_day"`
	PublishedAt string  `json:"published_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type PostAnnouncementRequest struct {
	Content        string  `json:"content"`
	StartsAt       *string `json:"starts_at"`
	EndsAt         *string `json:"ends_at"`
	AllDay         bool    `json:"all_day"`
	ParsedStartsAt *time.Time
	ParsedEndsAt   *time.Time
}

func (r *PostAnnouncementRequest) Validate() error {
	if err := api.ValidateRequiredField(r.Content, "content"); err != nil {
		return fmt.Errorf("content: %w", err)
	}
	var err error
	r.ParsedStartsAt, err = api.ValidateRFC3339Optional(r.StartsAt, "starts_at")
	if err != nil {
		return fmt.Errorf("starts_at: %w", err)
	}
	r.ParsedEndsAt, err = api.ValidateRFC3339Optional(r.EndsAt, "ends_at")
	if err != nil {
		return fmt.Errorf("ends_at: %w", err)
	}
	return nil
}

func (r *PostAnnouncementRequest) Sanitize() {
	r.Content = bluemonday.UGCPolicy().Sanitize(r.Content)
}
