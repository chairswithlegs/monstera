package apimodel

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
