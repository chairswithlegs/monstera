package domain

import "time"

// Marker holds a timeline read position (e.g. home, notifications).
type Marker struct {
	LastReadID string
	Version    int
	UpdatedAt  time.Time
}
