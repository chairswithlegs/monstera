package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/chairswithlegs/monstera/internal/media"
	"github.com/chairswithlegs/monstera/internal/store"
	"github.com/chairswithlegs/monstera/internal/uid"
)

// MediaService orchestrates media upload (storage + DB attachment record).
type MediaService interface {
	Upload(ctx context.Context, accountID string, body io.Reader, contentType string, description *string) (*UploadResult, error)
	Update(ctx context.Context, accountID, mediaID string, description *string, focusX, focusY *float64) (*domain.MediaAttachment, error)
	CreateRemote(ctx context.Context, accountID string, remoteURL string) (*domain.MediaAttachment, error)
}

type mediaService struct {
	store        store.Store
	mediaStore   media.MediaStore
	maxBytes     int64
	allowedTypes map[string]string // contentType -> Mastodon type ("image", "video", "audio", "gifv")
}

// NewMediaService returns a MediaService.
func NewMediaService(s store.Store, ms media.MediaStore, maxBytes int64) MediaService {
	return &mediaService{store: s, mediaStore: ms, maxBytes: maxBytes, allowedTypes: media.AllowedContentTypes}
}

// UploadResult is the result of a successful upload.
type UploadResult struct {
	Attachment *domain.MediaAttachment
}

// Upload reads the body (up to maxBytes), stores it, and creates a media_attachments row.
// contentType must be in allowedTypes; description is optional.
func (svc *mediaService) Upload(ctx context.Context, accountID string, body io.Reader, contentType string, description *string) (*UploadResult, error) {
	typeStr, ok := svc.allowedTypes[contentType]
	if !ok {
		return nil, fmt.Errorf("Upload: %w (content type %q not allowed)", domain.ErrValidation, contentType)
	}
	limited := io.LimitReader(body, svc.maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("Upload: read body: %w", err)
	}
	if int64(len(data)) > svc.maxBytes {
		return nil, fmt.Errorf("Upload: %w (body exceeds max size %d bytes)", domain.ErrValidation, svc.maxBytes)
	}

	var urlStr string
	var previewURLStr, blurhashStr *string
	key := media.StorageKey(contentType)

	if typeStr == domain.MediaTypeImage {
		img, err := media.DecodeImage(data, contentType)
		if err != nil {
			return nil, fmt.Errorf("Upload: %w", err)
		}
		cleanData, err := media.ScrubAndReencode(img, contentType)
		if err != nil {
			return nil, fmt.Errorf("Upload: %w", err)
		}
		storedContentType := contentType
		if contentType == "image/webp" {
			storedContentType = "image/png"
		}
		if err := svc.mediaStore.Put(ctx, key, bytes.NewReader(cleanData), storedContentType); err != nil {
			return nil, fmt.Errorf("media Put: %w", err)
		}
		urlStr, err = svc.mediaStore.URL(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("media URL: %w", err)
		}
		preview := media.GeneratePreview(img)
		previewBytes, err := media.EncodePreviewJPEG(preview)
		if err != nil {
			return nil, fmt.Errorf("Upload: %w", err)
		}
		previewKey := media.StorageKey("image/jpeg")
		if err := svc.mediaStore.Put(ctx, previewKey, bytes.NewReader(previewBytes), "image/jpeg"); err != nil {
			return nil, fmt.Errorf("media Put preview: %w", err)
		}
		previewURL, err := svc.mediaStore.URL(ctx, previewKey)
		if err != nil {
			return nil, fmt.Errorf("media URL preview: %w", err)
		}
		previewURLStr = &previewURL
		bh, err := media.ComputeBlurhash(preview)
		if err != nil {
			return nil, fmt.Errorf("Upload: %w", err)
		}
		blurhashStr = &bh
	} else {
		if err := svc.mediaStore.Put(ctx, key, bytes.NewReader(data), contentType); err != nil {
			return nil, fmt.Errorf("media Put: %w", err)
		}
		var err error
		urlStr, err = svc.mediaStore.URL(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("media URL: %w", err)
		}
	}

	id := uid.New()
	in := store.CreateMediaAttachmentInput{
		ID:          id,
		AccountID:   accountID,
		Type:        typeStr,
		StorageKey:  key,
		URL:         urlStr,
		PreviewURL:  previewURLStr,
		RemoteURL:   nil,
		Description: description,
		Blurhash:    blurhashStr,
		Meta:        nil,
	}
	att, err := svc.store.CreateMediaAttachment(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("CreateMediaAttachment: %w", err)
	}
	return &UploadResult{Attachment: att}, nil
}

// Update updates a media attachment's description and/or focus. Only unattached media (status_id IS NULL) may be updated.
func (svc *mediaService) Update(ctx context.Context, accountID, mediaID string, description *string, focusX, focusY *float64) (*domain.MediaAttachment, error) {
	att, err := svc.store.GetMediaAttachment(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("Update: %w", err)
	}
	if att.AccountID != accountID {
		return nil, domain.ErrNotFound
	}
	if att.StatusID != nil {
		return nil, fmt.Errorf("Update: media already attached to status: %w", domain.ErrUnprocessable)
	}

	newDesc := att.Description
	if description != nil {
		newDesc = description
	}

	meta := att.Meta
	if focusX != nil && focusY != nil {
		var metaMap map[string]any
		if len(meta) > 0 {
			if err := json.Unmarshal(meta, &metaMap); err != nil {
				return nil, fmt.Errorf("Update: invalid existing meta: %w", err)
			}
		} else {
			metaMap = make(map[string]any)
		}
		metaMap["focus"] = map[string]float64{"x": *focusX, "y": *focusY}
		var err error
		meta, err = json.Marshal(metaMap)
		if err != nil {
			return nil, fmt.Errorf("Update: marshal meta: %w", err)
		}
	}

	updated, err := svc.store.UpdateMediaAttachment(ctx, store.UpdateMediaAttachmentInput{
		ID:          mediaID,
		AccountID:   accountID,
		Description: newDesc,
		Meta:        []byte(meta),
	})
	if err != nil {
		return nil, fmt.Errorf("Update: %w", err)
	}
	return updated, nil
}

// CreateRemote creates a media attachment record for a remote URL (no upload). Used for incoming Note attachments.
func (svc *mediaService) CreateRemote(ctx context.Context, accountID string, remoteURL string) (*domain.MediaAttachment, error) {
	if remoteURL == "" {
		return nil, fmt.Errorf("CreateRemote: %w (empty URL)", domain.ErrValidation)
	}
	att, err := svc.store.CreateMediaAttachment(ctx, store.CreateMediaAttachmentInput{
		ID:         uid.New(),
		AccountID:  accountID,
		Type:       "image",
		StorageKey: "",
		URL:        remoteURL,
		RemoteURL:  &remoteURL,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateRemote: %w", err)
	}
	return att, nil
}
