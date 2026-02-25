package ap

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chairswithlegs/monstera-fed/internal/cache"
	"github.com/stretchr/testify/require"
)

const testKeyID = "https://example.com#key"

func TestSignVerify_RoundTrip(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyID := "https://example.com/users/alice#main-key"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://inbox.example/inbox", bytes.NewReader([]byte(`{"type":"Create"}`)))
	require.NoError(t, err)
	req.Host = "inbox.example"

	err = Sign(req, keyID, key)
	require.NoError(t, err)
	require.NotEmpty(t, req.Header.Get("Signature"))
	require.NotEmpty(t, req.Header.Get("Date"))
	require.NotEmpty(t, req.Header.Get("Digest"))

	c, err := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()

	fetcher := func(ctx context.Context, id string) (*rsa.PublicKey, error) {
		if id != keyID {
			return nil, nil
		}
		return &key.PublicKey, nil
	}

	gotKeyID, err := Verify(context.Background(), req, fetcher, c)
	require.NoError(t, err)
	require.Equal(t, keyID, gotKeyID)
}

func TestVerify_missingDate(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	req.Header.Set("Signature", `keyId="https://example.com#key",algorithm="rsa-sha256",signature="abc"`)
	c, _ := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	defer func() { _ = c.Close() }()
	_, err := Verify(context.Background(), req, nil, c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing Date")
}

func TestVerify_unsupportedAlgorithm(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("Signature", `keyId="https://example.com#key",algorithm="hmac-sha256",signature="abc"`)
	c, _ := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	defer func() { _ = c.Close() }()
	_, err := Verify(context.Background(), req, nil, c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported algorithm")
}

func TestVerify_clockSkew(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	req.Header.Set("Date", time.Now().Add(clockSkew+time.Second).UTC().Format(http.TimeFormat))
	req.Header.Set("Host", "example.com")
	err := Sign(req, testKeyID, key)
	require.NoError(t, err)

	c, _ := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	defer func() { _ = c.Close() }()
	fetcher := func(ctx context.Context, id string) (*rsa.PublicKey, error) {
		return &key.PublicKey, nil
	}
	_, err = Verify(context.Background(), req, fetcher, c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "drift")
}

func TestVerify_digestMismatch(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	body := []byte(`{"type":"Create"}`)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/inbox", bytes.NewReader(body))
	req.Header.Set("Host", "example.com")
	err := Sign(req, testKeyID, key)
	require.NoError(t, err)
	require.NotEmpty(t, req.Header.Get("Digest"))

	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/inbox", bytes.NewReader(body))
	req2.Header.Set("Date", req.Header.Get("Date"))
	req2.Header.Set("Host", req.Header.Get("Host"))
	req2.Header.Set("Digest", "SHA-256=wrong")
	req2.Header.Set("Signature", req.Header.Get("Signature"))

	c, _ := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	defer func() { _ = c.Close() }()
	fetcher := func(ctx context.Context, id string) (*rsa.PublicKey, error) {
		return &key.PublicKey, nil
	}
	_, err = Verify(context.Background(), req2, fetcher, c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Digest")
}

func TestVerify_replayDetected(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	req.Host = "example.com"
	err := Sign(req, testKeyID, key)
	require.NoError(t, err)

	c, _ := cache.New(cache.Config{Driver: "memory", Logger: slog.Default()})
	defer func() { _ = c.Close() }()

	fetcher := func(_ context.Context, _ string) (*rsa.PublicKey, error) { //nolint:unparam
		return &key.PublicKey, nil
	}

	_, err = Verify(context.Background(), req, fetcher, c)
	require.NoError(t, err)

	// Second Verify with same (keyID, date, request-target) must detect replay.
	// Cache may apply writes asynchronously, so poll until replay is detected.
	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	req2.Host = "example.com"
	req2.Header.Set("Date", req.Header.Get("Date"))
	req2.Header.Set("Signature", req.Header.Get("Signature"))

	require.Eventually(t, func() bool {
		_, err := Verify(context.Background(), req2, fetcher, c)
		if err != nil && strings.Contains(err.Error(), "replay") {
			return true
		}
		// Rebuild request for next attempt (Verify may consume state)
		req2.Header.Set("Date", req.Header.Get("Date"))
		req2.Header.Set("Signature", req.Header.Get("Signature"))
		return false
	}, 500*time.Millisecond, 10*time.Millisecond, "second Verify should detect replay")
}

func TestParseSignatureHeader(t *testing.T) {
	header := `keyId="https://example.com#key",algorithm="rsa-sha256",headers="(request-target) host date",signature="abc123=="`
	sig, err := parseSignatureHeader(header)
	require.NoError(t, err)
	require.Equal(t, "https://example.com#key", sig.keyID)
	require.Equal(t, "abc123==", sig.signature)
	require.Equal(t, []string{"(request-target)", "host", "date"}, sig.headers)
}

func TestSplitSignatureParams_respectsQuotes(t *testing.T) {
	parts := splitSignatureParams(`keyId="https://x",signature="a+b=c"`)
	require.Len(t, parts, 2)
	require.Contains(t, parts[0], "keyId")
	require.Contains(t, parts[1], "a+b=c")
}
