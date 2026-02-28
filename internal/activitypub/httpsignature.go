package activitypub

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
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
	"github.com/chairswithlegs/monstera-fed/internal/config"
	"github.com/chairswithlegs/monstera-fed/internal/domain"
	"github.com/chairswithlegs/monstera-fed/internal/store"
)

// clockSkew is the maximum tolerated difference between the request's Date
// header and the server's clock. Requests outside this window are rejected
// even if the signature is otherwise valid.
const clockSkew = 30 * time.Second

// replayTTL is how long a (keyId, date, requestTarget) tuple is remembered
// to prevent replay attacks. Set to 60s — double the clock skew window.
const replayTTL = 60 * time.Second

// pubkeyCacheTTL is how long a remote actor's public key is cached.
const pubkeyCacheTTL = 1 * time.Hour

// HTTPSignatureService verifies HTTP Signatures and fetches remote public keys (with caching).
// For outgoing federation it can sign requests using a local account's key (SignWithSenderID); store and instanceDomain must be set.
type HTTPSignatureService struct {
	client         *http.Client
	cache          cache.Store
	store          store.Store
	instanceDomain string
}

// NewHTTPSignatureService returns an HTTPSignatureService that builds its HTTP client from cfg.
// When cfg.FederationInsecureSkipTLS is true, the client skips TLS verification (for development).
// If s is non-nil, SignWithSenderID can sign requests using the local account's key; instanceDomain is taken from cfg.InstanceDomain.
func NewHTTPSignatureService(cfg *config.Config, c cache.Store, s store.Store) *HTTPSignatureService {
	instanceDomain := ""
	if cfg != nil {
		instanceDomain = cfg.InstanceDomain
	}
	return &HTTPSignatureService{
		client:         federationHTTPClient(cfg),
		cache:          c,
		store:          s,
		instanceDomain: instanceDomain,
	}
}

// Verify verifies the HTTP Signature on an incoming ActivityPub request.
//
// Algorithm (draft-cavage-http-signatures-12, Mastodon-compatible):
//
//  1. Parse the Signature header to extract keyId, algorithm, headers, signature.
//  2. Check the Date header — reject if it is more than ±30 seconds from now.
//  3. If the request has a body (POST), verify the Digest header.
//  4. Reconstruct the signing string from the listed headers.
//  5. Fetch the signing actor's public key via s (cached).
//  6. Verify the RSA-SHA256 signature over the signing string.
//  7. On verification failure: evict cached key, re-fetch, retry once (key rotation).
//  8. Check for replay: (keyId, Date, requestTarget) stored in cache with replayTTL.
//
// Returns the keyId (actor key IRI) on success; an error otherwise.
func (s *HTTPSignatureService) Verify(ctx context.Context, r *http.Request) (keyID string, err error) {
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

	pubKey, err := s.FetchRemotePublicKey(ctx, sig.keyID)
	if err != nil {
		return "", fmt.Errorf("httpsig: fetch key %q: %w", sig.keyID, err)
	}

	hash := sha256.Sum256([]byte(signingString))
	sigBytes, err := base64.StdEncoding.DecodeString(sig.signature)
	if err != nil {
		return "", fmt.Errorf("httpsig: decode signature: %w", err)
	}

	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], sigBytes); err != nil {
		s.evictKey(ctx, sig.keyID)
		pubKey2, fetchErr := s.FetchRemotePublicKey(ctx, sig.keyID)
		if fetchErr != nil {
			return "", fmt.Errorf("httpsig: signature verification failed: %w", err)
		}
		if err := rsa.VerifyPKCS1v15(pubKey2, crypto.SHA256, hash[:], sigBytes); err != nil {
			return "", fmt.Errorf("httpsig: signature verification failed: %w", err)
		}
	}

	replayKey := replayCacheKey(sig.keyID, dateStr, requestTarget(r))
	exists, _ := s.cache.Exists(ctx, replayKey)
	if exists {
		return "", errors.New("httpsig: replay detected")
	}
	_ = s.cache.Set(ctx, replayKey, []byte("1"), replayTTL)

	return sig.keyID, nil
}

// FetchRemotePublicKey returns the RSA public key for keyID, from cache or by fetching the actor document.
func (s *HTTPSignatureService) FetchRemotePublicKey(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	ck := pubKeyCacheKey(keyID)
	b, err := s.cache.Get(ctx, ck)
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
		_ = s.cache.Delete(ctx, ck)
	}

	pubKey, err := s.fetchFromRemote(ctx, keyID)
	if err != nil {
		return nil, err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("httpsignature: marshal public key: %w", err)
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}
	_ = s.cache.Set(ctx, ck, pem.EncodeToMemory(block), pubkeyCacheTTL)
	return pubKey, nil
}

// Sign signs an outgoing HTTP request with the given RSA private key.
//
// Signed headers: (request-target), host, date, digest (if body present).
//
// The Date header is set to the current time if not already present.
// If the request has a body, the Digest header is computed and set.
// The Signature header is constructed per draft-cavage-http-signatures-12.
func (s *HTTPSignatureService) Sign(r *http.Request, keyID string, privateKey *rsa.PrivateKey) error {
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

// SignWithSenderID looks up the local account by senderID, parses its private key, and signs r.
// Returns an error if store is nil, account is not found, account has no private key, or the key is invalid.
func (s *HTTPSignatureService) SignWithSenderID(ctx context.Context, r *http.Request, senderID string) error {
	if s.store == nil {
		return errors.New("httpsignature: store not configured for signing")
	}
	account, err := s.store.GetAccountByID(ctx, senderID)
	if err != nil {
		return fmt.Errorf("httpsignature: get account %s: %w", senderID, err)
	}
	if account == nil {
		return fmt.Errorf("httpsignature: sender not found: %s", senderID)
	}
	if account.PrivateKey == nil || *account.PrivateKey == "" {
		return fmt.Errorf("httpsignature: sender has no private key: %s", senderID)
	}
	privateKey, err := parseRSAPrivateKeyPEM(*account.PrivateKey)
	if err != nil {
		return fmt.Errorf("httpsignature: invalid private key for %s: %w", senderID, err)
	}
	keyID := actorKeyID(account, s.instanceDomain)
	return s.Sign(r, keyID, privateKey)
}

// actorKeyID returns the ActivityPub key ID for the account (APID + "#main-key").
func actorKeyID(account *domain.Account, instanceDomain string) string {
	base := account.APID
	if base == "" {
		base = fmt.Sprintf("https://%s/users/%s", instanceDomain, account.Username)
	}
	return base + "#main-key"
}

// parseRSAPrivateKeyPEM decodes a PEM-encoded RSA private key (PKCS1).
func parseRSAPrivateKeyPEM(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS1 private key: %w", err)
	}
	return key, nil
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
			lines = append(lines, "(request-target): "+requestTarget(r))
		case "host":
			lines = append(lines, "host: "+r.Host)
		default:
			lines = append(lines, fmt.Sprintf("%s: %s",
				strings.ToLower(h), r.Header.Get(h)))
		}
	}
	return strings.Join(lines, "\n")
}

func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// EvictKey removes the cached key for keyID so the next FetchPublicKey will refetch from remote.
func (s *HTTPSignatureService) evictKey(ctx context.Context, keyID string) {
	_ = s.cache.Delete(ctx, pubKeyCacheKey(keyID))
}

func (s *HTTPSignatureService) fetchFromRemote(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	actorURL := keyID
	if idx := strings.Index(keyID, "#"); idx >= 0 {
		actorURL = keyID[:idx]
	}
	if actorURL == "" {
		return nil, errors.New("httpsignature: empty key ID")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actorURL, nil)
	if err != nil {
		return nil, fmt.Errorf("httpsignature: new request: %w", err)
	}
	req.Header.Set("Accept", "application/activity+json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpsignature: fetch actor: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("httpsignature: actor fetch status %d", resp.StatusCode)
	}
	var actor struct {
		PublicKey struct {
			ID           string `json:"id"`
			PublicKeyPem string `json:"publicKeyPem"`
		} `json:"publicKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&actor); err != nil {
		return nil, fmt.Errorf("httpsignature: decode actor: %w", err)
	}
	if actor.PublicKey.PublicKeyPem == "" {
		return nil, errors.New("httpsignature: actor has no publicKeyPem")
	}
	block, _ := pem.Decode([]byte(actor.PublicKey.PublicKeyPem))
	if block == nil {
		return nil, errors.New("httpsignature: invalid PEM in publicKeyPem")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		pk, errRSA := x509.ParsePKCS1PublicKey(block.Bytes)
		if errRSA != nil {
			return nil, fmt.Errorf("httpsignature: parse public key: %w", err)
		}
		return pk, nil
	}
	pk, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("httpsignature: public key is not RSA")
	}
	return pk, nil
}

func federationHTTPClient(cfg *config.Config) *http.Client {
	if cfg == nil || !cfg.FederationInsecureSkipTLS {
		return http.DefaultClient
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			//nolint:gosec // G402: intentional for development federation with self-signed certs
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func pubKeyCacheKey(keyID string) string {
	h := sha256.Sum256([]byte(keyID))
	return "ap:pubkey:" + hex.EncodeToString(h[:16])
}

// replayCacheKey returns the cache key for replay detection.
func replayCacheKey(keyID, date, reqTarget string) string {
	h := sha256.Sum256([]byte(keyID + ":" + date + ":" + reqTarget))
	return "httpsig:" + hex.EncodeToString(h[:16])
}

// requestTarget returns the (request-target) value for HTTP Signature.
func requestTarget(r *http.Request) string {
	return strings.ToLower(r.Method) + " " + r.URL.RequestURI()
}
