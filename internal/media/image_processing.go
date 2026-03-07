package media

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"

	"github.com/buckket/go-blurhash"
	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/disintegration/imaging"
	"golang.org/x/image/webp"
)

const (
	previewMaxDimension = 400
	previewJPEGQuality  = 80
	originalJPEGQuality = 95
	blurhashXComponents = 4
	blurhashYComponents = 3
	contentTypePNG      = "image/png"
	contentTypeWebP     = "image/webp"
)

// DecodeImage decodes JPEG, PNG, GIF, or WebP bytes into an image.Image.
// Returns domain.ErrValidation if the bytes are not a valid image.
func DecodeImage(data []byte, contentType string) (image.Image, error) {
	r := bytes.NewReader(data)
	switch contentType {
	case "image/jpeg", contentTypePNG, "image/gif":
		img, _, err := image.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", contentType, domain.ErrValidation)
		}
		return img, nil
	case contentTypeWebP:
		img, err := webp.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", contentTypeWebP, domain.ErrValidation)
		}
		return img, nil
	default:
		return nil, fmt.Errorf("unsupported image type %q: %w", contentType, domain.ErrValidation)
	}
}

// ScrubAndReencode re-encodes img in the same format, dropping all metadata
// (EXIF, XMP, ICC profiles). Quality 95 for JPEG, lossless for PNG/GIF.
// WebP is re-encoded as PNG (no WebP encoder in stdlib/x/image).
func ScrubAndReencode(img image.Image, contentType string) ([]byte, error) {
	var buf bytes.Buffer
	switch contentType {
	case "image/jpeg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: originalJPEGQuality}); err != nil {
			return nil, fmt.Errorf("jpeg encode: %w", err)
		}
		return buf.Bytes(), nil
	case contentTypePNG:
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("png encode: %w", err)
		}
		return buf.Bytes(), nil
	case "image/gif":
		if err := gif.Encode(&buf, img, nil); err != nil {
			return nil, fmt.Errorf("gif encode: %w", err)
		}
		return buf.Bytes(), nil
	case contentTypeWebP:
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("png encode (webp->png): %w", err)
		}
		return buf.Bytes(), nil
	default:
		return nil, fmt.Errorf("unsupported image type %q", contentType)
	}
}

// GeneratePreview resizes img so neither dimension exceeds previewMaxDimension,
// preserving aspect ratio. Returns the scaled image.
func GeneratePreview(img image.Image) image.Image {
	return imaging.Fit(img, previewMaxDimension, previewMaxDimension, imaging.Lanczos)
}

// EncodePreviewJPEG encodes img as JPEG bytes at quality 80.
func EncodePreviewJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: previewJPEGQuality}); err != nil {
		return nil, fmt.Errorf("preview jpeg encode: %w", err)
	}
	return buf.Bytes(), nil
}

// ComputeBlurhash returns a blurhash string for img using 4×3 components
// (Mastodon default).
func ComputeBlurhash(img image.Image) (string, error) {
	str, err := blurhash.Encode(blurhashXComponents, blurhashYComponents, img)
	if err != nil {
		return "", fmt.Errorf("blurhash encode: %w", err)
	}
	return str, nil
}
