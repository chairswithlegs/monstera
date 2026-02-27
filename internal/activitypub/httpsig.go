package activitypub

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/chairswithlegs/monstera-fed/internal/cache"
)

// KeyFetcher fetches the public key for a given key ID from the remote AP
// actor document. The key ID is typically the actor's `publicKey.id` field
// (e.g. "https://remote.example/users/alice#main-key").
//
// Implementation: fetches the actor document over HTTPS, extracts the
// publicKey.publicKeyPem field, parses the PEM-encoded RSA public key.
// Results are cached under "ap:pubkey:{keyID}".
type KeyFetcher func(ctx context.Context, keyID string) (*rsa.PublicKey, error)

// clockSkew is the maximum tolerated difference between the request's Date
// header and the server's clock. Requests outside this window are rejected
// even if the signature is otherwise valid.
const clockSkew = 30 * time.Second

// replayTTL is how long a (keyId, date, requestTarget) tuple is remembered
// to prevent replay attacks. Set to 60s — double the clock skew window.
const replayTTL = 60 * time.Second

// pubkeyCacheTTL is how long a fetched public key is cached.
const pubkeyCacheTTL = 1 * time.Hour

// Verify verifies the HTTP Signature on an incoming ActivityPub request.
//
// Algorithm (draft-cavage-http-signatures-12, Mastodon-compatible):
//
//  1. Parse the Signature header to extract keyId, algorithm, headers, signature.
//  2. Check the Date header — reject if it is more than ±30 seconds from now.
//  3. If the request has a body (POST), verify the Digest header.
//  4. Reconstruct the signing string from the listed headers.
//  5. Fetch the signing actor's public key via keyFetcher (cached).
//  6. Verify the RSA-SHA256 signature over the signing string.
//  7. On verification failure: evict cached key, re-fetch, retry once (key rotation).
//  8. Check for replay: (keyId, Date, requestTarget) stored in cache with replayTTL.
//
// Returns the keyId (actor key IRI) on success; an error otherwise.
func Verify(
	ctx context.Context,
	r *http.Request,
	keyFetcher KeyFetcher,
	c cache.Store,
) (keyID string, err error) {
	sig, err := parseSignatureHeader(r.Header.Get("Signature"))
	if err != nil {
		return "", fmt.Errorf("httpsig: parse header: %w", err)
	}
	if strings.ToLower(sig.algorithm) != "rsa-sha256" {
		return "", fmt.Errorf("httpsig: unsupported algorithm %q", sig.algorithm)
	}

	dateStr := r.Header.Get("Date")
	if dateStr == "" {
		return "", errors.New("httpsig: missing Date header")
	}
	requestDate, err := http.ParseTime(dateStr)
	if err != nil {
		return "", fmt.Errorf("httpsig: parse Date: %w", err)
	}
	drift := time.Since(requestDate)
	if drift < 0 {
		drift = -drift
	}
	if drift > clockSkew {
		return "", fmt.Errorf("httpsig: Date header drift %v exceeds ±%v", drift, clockSkew)
	}

	if r.Method == http.MethodPost && r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return "", fmt.Errorf("httpsig: read body: %w", err)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		expectedDigest := "SHA-256=" + base64.StdEncoding.EncodeToString(sha256Sum(body))
		actualDigest := r.Header.Get("Digest")
		if actualDigest != expectedDigest {
			return "", errors.New("httpsig: Digest mismatch")
		}
	}

	signingString := buildSigningString(r, sig.headers)

	pubKey, err := fetchPubKeyCached(ctx, sig.keyID, keyFetcher, c)
	if err != nil {
		return "", fmt.Errorf("httpsig: fetch key %q: %w", sig.keyID, err)
	}

	hash := sha256.Sum256([]byte(signingString))
	sigBytes, err := base64.StdEncoding.DecodeString(sig.signature)
	if err != nil {
		return "", fmt.Errorf("httpsig: decode signature: %w", err)
	}

	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], sigBytes); err != nil {
		_ = c.Delete(ctx, pubKeyCacheKey(sig.keyID))
		pubKey2, fetchErr := keyFetcher(ctx, sig.keyID)
		if fetchErr != nil {
			return "", fmt.Errorf("httpsig: signature verification failed: %w", err)
		}
		if err := rsa.VerifyPKCS1v15(pubKey2, crypto.SHA256, hash[:], sigBytes); err != nil {
			return "", fmt.Errorf("httpsig: signature verification failed: %w", err)
		}
	}

	replayKey := ReplayCacheKey(sig.keyID, dateStr, RequestTarget(r))
	exists, _ := c.Exists(ctx, replayKey)
	if exists {
		return "", errors.New("httpsig: replay detected")
	}
	_ = c.Set(ctx, replayKey, []byte("1"), replayTTL)

	return sig.keyID, nil
}

// Sign signs an outgoing HTTP request with the given RSA private key.
//
// Signed headers: (request-target), host, date, digest (if body present).
//
// The Date header is set to the current time if not already present.
// If the request has a body, the Digest header is computed and set.
// The Signature header is constructed per draft-cavage-http-signatures-12.
func Sign(r *http.Request, keyID string, privateKey *rsa.PrivateKey) error {
	if r.Header.Get("Date") == "" {
		r.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	}

	if r.Header.Get("Host") == "" {
		r.Header.Set("Host", r.URL.Host)
	}

	headers := []string{"(request-target)", "host", "date"}

	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("httpsig: read body for digest: %w", err)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))

		digest := "SHA-256=" + base64.StdEncoding.EncodeToString(sha256Sum(body))
		r.Header.Set("Digest", digest)
		headers = append(headers, "digest")
	}

	signingString := buildSigningString(r, headers)

	hash := sha256.Sum256([]byte(signingString))
	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return fmt.Errorf("httpsig: sign: %w", err)
	}

	sig := base64.StdEncoding.EncodeToString(sigBytes)

	r.Header.Set("Signature",
		fmt.Sprintf(`keyId="%s",algorithm="rsa-sha256",headers="%s",signature="%s"`,
			keyID, strings.Join(headers, " "), sig))

	return nil
}

type signatureParams struct {
	keyID     string
	algorithm string
	headers   []string
	signature string
}

func parseSignatureHeader(header string) (*signatureParams, error) {
	if header == "" {
		return nil, errors.New("empty Signature header")
	}

	params := &signatureParams{}
	for _, part := range splitSignatureParams(header) {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"`)
		switch strings.TrimSpace(k) {
		case "keyId":
			params.keyID = v
		case "algorithm":
			params.algorithm = v
		case "headers":
			params.headers = strings.Fields(v)
		case "signature":
			params.signature = v
		}
	}

	if params.keyID == "" || params.signature == "" {
		return nil, errors.New("missing required Signature fields (keyId, signature)")
	}

	if len(params.headers) == 0 {
		params.headers = []string{"date"}
	}

	return params, nil
}

func splitSignatureParams(header string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	for _, ch := range header {
		switch {
		case ch == '"':
			inQuote = !inQuote
			current.WriteRune(ch)
		case ch == ',' && !inQuote:
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}
	return parts
}

func buildSigningString(r *http.Request, headers []string) string {
	var lines []string
	for _, h := range headers {
		switch h {
		case "(request-target)":
			lines = append(lines, "(request-target): "+RequestTarget(r))
		case "host":
			lines = append(lines, "host: "+r.Host)
		default:
			lines = append(lines, fmt.Sprintf("%s: %s",
				strings.ToLower(h), r.Header.Get(h)))
		}
	}
	return strings.Join(lines, "\n")
}

// RequestTarget returns the (request-target) value for HTTP Signature.
// Exported for use in tests.
func RequestTarget(r *http.Request) string {
	return strings.ToLower(r.Method) + " " + r.URL.RequestURI()
}

func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func fetchPubKeyCached(
	ctx context.Context,
	keyID string,
	keyFetcher KeyFetcher,
	c cache.Store,
) (*rsa.PublicKey, error) {
	ck := pubKeyCacheKey(keyID)
	b, err := c.Get(ctx, ck)
	if err == nil {
		block, _ := pem.Decode(b)
		if block != nil {
			key, errParse := x509.ParsePKIXPublicKey(block.Bytes)
			if errParse == nil {
				if pk, ok := key.(*rsa.PublicKey); ok {
					return pk, nil
				}
			}
		}
		_ = c.Delete(ctx, ck)
	}

	pubKey, err := keyFetcher(ctx, keyID)
	if err != nil {
		return nil, err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("httpsig: marshal public key: %w", err)
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}
	_ = c.Set(ctx, ck, pem.EncodeToMemory(block), pubkeyCacheTTL)
	return pubKey, nil
}

func pubKeyCacheKey(keyID string) string {
	h := sha256.Sum256([]byte(keyID))
	return "ap:pubkey:" + hex.EncodeToString(h[:16])
}

// ReplayCacheKey returns the cache key for replay detection.
// Exported for use in tests.
func ReplayCacheKey(keyID, date, reqTarget string) string {
	h := sha256.Sum256([]byte(keyID + ":" + date + ":" + reqTarget))
	return "httpsig:" + hex.EncodeToString(h[:16])
}

// DefaultKeyFetcher returns a KeyFetcher that fetches the remote Actor document
// (from the key ID, stripping the fragment) and extracts publicKey.publicKeyPem.
func DefaultKeyFetcher(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	actorURL := keyID
	if idx := strings.Index(keyID, "#"); idx >= 0 {
		actorURL = keyID[:idx]
	}
	if actorURL == "" {
		return nil, errors.New("httpsig: empty key ID")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actorURL, nil)
	if err != nil {
		return nil, fmt.Errorf("httpsig: new request: %w", err)
	}
	req.Header.Set("Accept", "application/activity+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpsig: fetch actor: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("httpsig: actor fetch status %d", resp.StatusCode)
	}
	var actor struct {
		PublicKey struct {
			ID           string `json:"id"`
			PublicKeyPem string `json:"publicKeyPem"`
		} `json:"publicKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&actor); err != nil {
		return nil, fmt.Errorf("httpsig: decode actor: %w", err)
	}
	if actor.PublicKey.PublicKeyPem == "" {
		return nil, errors.New("httpsig: actor has no publicKeyPem")
	}
	block, _ := pem.Decode([]byte(actor.PublicKey.PublicKeyPem))
	if block == nil {
		return nil, errors.New("httpsig: invalid PEM in publicKeyPem")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		pk, errRSA := x509.ParsePKCS1PublicKey(block.Bytes)
		if errRSA != nil {
			return nil, fmt.Errorf("httpsig: parse public key: %w", err)
		}
		return pk, nil
	}
	pk, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("httpsig: public key is not RSA")
	}
	return pk, nil
}
