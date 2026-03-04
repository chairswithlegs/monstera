package service

import (
	"context"
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
	key := media.StorageKey(contentType)
	if err := svc.mediaStore.Put(ctx, key, limited, contentType); err != nil {
		return nil, fmt.Errorf("media Put: %w", err)
	}
	urlStr, err := svc.mediaStore.URL(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("media URL: %w", err)
	}
	id := uid.New()
	in := store.CreateMediaAttachmentInput{
		ID:          id,
		AccountID:   accountID,
		Type:        typeStr,
		StorageKey:  key,
		URL:         urlStr,
		PreviewURL:  nil,
		RemoteURL:   nil,
		Description: description,
		Blurhash:    nil,
		Meta:        nil,
	}
	att, err := svc.store.CreateMediaAttachment(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("CreateMediaAttachment: %w", err)
	}
	return &UploadResult{Attachment: att}, nil
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
