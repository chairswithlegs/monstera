package domain

import (
	"encoding/json"
	"time"
)

// MediaAttachment is an image, video, or audio file attached to a status or account.
type MediaAttachment struct {
	ID          string
	AccountID   string
	StatusID    *string
	Type        string  // Mastodon category: "image", "video", "audio", "gifv"
	ContentType *string // MIME type: "image/jpeg", "video/mp4", etc.
	StorageKey  string
	URL         string
	PreviewURL  *string
	RemoteURL   *string
	Description *string
	Blurhash    *string
	Meta        json.RawMessage
	CreatedAt   time.Time
}

const (
	MediaTypeImage = "image"
	MediaTypeVideo = "video"
	MediaTypeAudio = "audio"
	MediaTypeGifv  = "gifv"
)
