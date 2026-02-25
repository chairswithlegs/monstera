// Package local provides a filesystem-backed MediaStore implementation.
// Files are stored under a configurable base directory, mirroring the
// storage key structure. The Go HTTP server serves them at /system/...
//
// Not suitable for multi-replica deployments unless the base directory is
// on shared storage (NFS, EFS). Use the s3 driver for multi-replica setups.
package local

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/chairswithlegs/monstera-fed/internal/media"
)

// Store is the local filesystem MediaStore implementation.
type Store struct {
	basePath string
	baseURL  string
}

func init() {
	media.Register("local", func(cfg media.Config) (media.MediaStore, error) {
		return New(cfg.LocalPath, cfg.BaseURL)
	})
}

// New creates a local Store rooted at basePath.
// basePath must be an absolute path; the directory is created if it does not exist.
func New(basePath, baseURL string) (*Store, error) {
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("media/local: create base directory %q: %w", basePath, err)
	}
	return &Store{
		basePath: filepath.Clean(basePath),
		baseURL:  strings.TrimRight(baseURL, "/"),
	}, nil
}

// Put writes the content of r to the file at key under basePath.
// Write safety: data is first written to a sibling .tmp file, then atomically
// renamed to the final path.
func (s *Store) Put(_ context.Context, key string, r io.Reader, _ string) error {
	dst := filepath.Join(s.basePath, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return fmt.Errorf("media/local: mkdir for %q: %w", key, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".upload-*")
	if err != nil {
		return fmt.Errorf("media/local: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	ok := false
	defer func() {
		if !ok {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, r); err != nil {
		return fmt.Errorf("media/local: write %q: %w", key, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("media/local: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("media/local: rename to %q: %w", key, err)
	}

	ok = true
	return nil
}

// Get opens the file at key and returns it as an io.ReadCloser.
// Returns media.ErrNotFound if the file does not exist.
func (s *Store) Get(_ context.Context, key string) (io.ReadCloser, error) {
	path := filepath.Join(s.basePath, filepath.FromSlash(key))
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, media.ErrNotFound
		}
		return nil, fmt.Errorf("media/local: open %q: %w", key, err)
	}
	return f, nil
}

// Delete removes the file at key. A no-op if the file does not exist.
func (s *Store) Delete(_ context.Context, key string) error {
	path := filepath.Join(s.basePath, filepath.FromSlash(key))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("media/local: delete %q: %w", key, err)
	}
	return nil
}

// URL returns the public URL for the given storage key.
// Format: {baseURL}/system/{key}
func (s *Store) URL(_ context.Context, key string) (string, error) {
	return s.baseURL + "/system/" + key, nil
}

// ServeHTTP handles GET /system/{key...} requests for locally-stored media.
func (s *Store) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/system/")
	if key == "" || strings.Contains(key, "..") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	etag := fmt.Sprintf(`"%x"`, sha256.Sum256([]byte(key)))

	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	rc, err := s.Get(r.Context(), key)
	if err != nil {
		if errors.Is(err, media.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = rc.Close() }()

	ext := filepath.Ext(key)
	if ct := mime.TypeByExtension(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", etag)

	_, _ = io.Copy(w, rc)
}
