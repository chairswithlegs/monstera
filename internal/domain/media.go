package domain

import (
	"encoding/json"
	"time"
)

type MediaAttachment struct {
	ID          string
	AccountID   string
	StatusID    *string
	Type        string
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
