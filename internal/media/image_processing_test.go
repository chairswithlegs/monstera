package media

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/chairswithlegs/monstera/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeImage(t *testing.T) {
	t.Parallel()

	miniJPEG := func() []byte {
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, color.RGBA{R: 0, G: 0, B: 0, A: 255})
		var buf bytes.Buffer
		require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}))
		return buf.Bytes()
	}
	miniPNG := func() []byte {
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, color.RGBA{R: 0, G: 0, B: 0, A: 255})
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, img))
		return buf.Bytes()
	}
	miniGIF := func() []byte {
		img := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.Black, color.White})
		img.SetColorIndex(0, 0, 0)
		var buf bytes.Buffer
		require.NoError(t, gif.Encode(&buf, img, nil))
		return buf.Bytes()
	}

	tests := []struct {
		name        string
		data        []byte
		contentType string
		wantErr     bool
		errIs       error
	}{
		{"jpeg", miniJPEG(), "image/jpeg", false, nil},
		{"png", miniPNG(), "image/png", false, nil},
		{"gif", miniGIF(), "image/gif", false, nil},
		{"invalid bytes", []byte("not an image"), "image/jpeg", true, domain.ErrValidation},
		{"unsupported type", miniPNG(), "image/bmp", true, domain.ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			img, err := DecodeImage(tt.data, tt.contentType)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errIs != nil {
					require.ErrorIs(t, err, tt.errIs)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, img)
			assert.NotZero(t, img.Bounds().Dx())
			assert.NotZero(t, img.Bounds().Dy())
		})
	}
}

func TestDecodeImage_webp(t *testing.T) {
	t.Parallel()
	// Minimal 1x1 lossy WebP (base64 from known minimal fixture)
	b, err := base64.StdEncoding.DecodeString("UklGRiQAAABXRUJQVlA4IBgAAAAwAQCdASoBAAEAAwA0JaQAA3AA/vu2AAA=")
	require.NoError(t, err)
	img, err := DecodeImage(b, "image/webp")
	require.NoError(t, err)
	require.NotNil(t, img)
	assert.NotZero(t, img.Bounds().Dx())
	assert.NotZero(t, img.Bounds().Dy())
}

func TestGeneratePreview(t *testing.T) {
	t.Parallel()
	img := image.NewRGBA(image.Rect(0, 0, previewMaxDimension*2, previewMaxDimension*1.5))
	preview := GeneratePreview(img)
	require.NotNil(t, preview)
	b := preview.Bounds()
	assert.Positive(t, b.Dx())
	assert.Positive(t, b.Dy())
	// Should not exceed previewMaxDimension
	assert.LessOrEqual(t, b.Dx(), previewMaxDimension)
	assert.LessOrEqual(t, b.Dy(), previewMaxDimension)
	// Should keep aspect ratio
	assert.Equal(t, previewMaxDimension, b.Dx())
	assert.Equal(t, int(previewMaxDimension*0.75), b.Dy())
}

func TestGeneratePreview_small_image_unchanged(t *testing.T) {
	t.Parallel()
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	preview := GeneratePreview(img)
	require.NotNil(t, preview)
	assert.Equal(t, 50, preview.Bounds().Dx())
	assert.Equal(t, 50, preview.Bounds().Dy())
}

func TestComputeBlurhash(t *testing.T) {
	t.Parallel()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 25), G: uint8(y * 25), B: 128, A: 255})
		}
	}
	str, err := ComputeBlurhash(img)
	require.NoError(t, err)
	assert.NotEmpty(t, str)
	assert.Greater(t, len(str), 6)
}

func TestEncodePreviewJPEG(t *testing.T) {
	t.Parallel()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	data, err := EncodePreviewJPEG(img)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, 2, decoded.Bounds().Dx())
	assert.Equal(t, 2, decoded.Bounds().Dy())
}

func TestResizeForAvatar(t *testing.T) {
	t.Parallel()

	t.Run("oversized image is resized within bounds", func(t *testing.T) {
		t.Parallel()
		img := image.NewRGBA(image.Rect(0, 0, AvatarMaxDimension*2, AvatarMaxDimension*2))
		out := ResizeForAvatar(img)
		require.NotNil(t, out)
		assert.LessOrEqual(t, out.Bounds().Dx(), AvatarMaxDimension)
		assert.LessOrEqual(t, out.Bounds().Dy(), AvatarMaxDimension)
	})

	t.Run("wide oversized image preserves aspect ratio", func(t *testing.T) {
		t.Parallel()
		// 800×400 → should scale to 400×200
		img := image.NewRGBA(image.Rect(0, 0, 800, 400))
		out := ResizeForAvatar(img)
		assert.Equal(t, AvatarMaxDimension, out.Bounds().Dx())
		assert.Equal(t, AvatarMaxDimension/2, out.Bounds().Dy())
	})

	t.Run("image within bounds is returned unchanged", func(t *testing.T) {
		t.Parallel()
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		out := ResizeForAvatar(img)
		assert.Equal(t, 100, out.Bounds().Dx())
		assert.Equal(t, 100, out.Bounds().Dy())
	})
}

func TestResizeForHeader(t *testing.T) {
	t.Parallel()

	t.Run("oversized image is resized within bounds", func(t *testing.T) {
		t.Parallel()
		img := image.NewRGBA(image.Rect(0, 0, HeaderMaxWidth*2, HeaderMaxHeight*2))
		out := ResizeForHeader(img)
		require.NotNil(t, out)
		assert.LessOrEqual(t, out.Bounds().Dx(), HeaderMaxWidth)
		assert.LessOrEqual(t, out.Bounds().Dy(), HeaderMaxHeight)
	})

	t.Run("wide oversized image preserves aspect ratio", func(t *testing.T) {
		t.Parallel()
		// 3000×500 → width-limited to 1500×250
		img := image.NewRGBA(image.Rect(0, 0, 3000, 500))
		out := ResizeForHeader(img)
		assert.Equal(t, HeaderMaxWidth, out.Bounds().Dx())
		assert.Equal(t, HeaderMaxHeight/2, out.Bounds().Dy())
	})

	t.Run("image within bounds is returned unchanged", func(t *testing.T) {
		t.Parallel()
		img := image.NewRGBA(image.Rect(0, 0, 800, 200))
		out := ResizeForHeader(img)
		assert.Equal(t, 800, out.Bounds().Dx())
		assert.Equal(t, 200, out.Bounds().Dy())
	})
}

func TestScrubAndReencode(t *testing.T) {
	t.Parallel()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	for _, ct := range []string{"image/jpeg", "image/png", "image/gif"} {
		data, err := ScrubAndReencode(img, ct)
		require.NoError(t, err, ct)
		require.NotEmpty(t, data, ct)
	}
	webpData, err := ScrubAndReencode(img, "image/webp")
	require.NoError(t, err)
	require.NotEmpty(t, webpData)
	_, err = png.Decode(bytes.NewReader(webpData))
	require.NoError(t, err)
}
