package cli

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignRequest_HeadersPresent(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	body := []byte(`{"type":"Create"}`)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"http://example.com/users/alice/inbox", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/activity+json")

	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	_ = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	keyID := "http://actor.example.com/users/bob#main-key"
	require.NoError(t, SignRequest(req, keyID, key))

	// Date and Host must be set.
	require.NotEmpty(t, req.Header.Get("Date"))
	require.NotEmpty(t, req.Header.Get("Host"))

	// Digest must be set for a POST with body.
	digest := req.Header.Get("Digest")
	require.True(t, strings.HasPrefix(digest, "SHA-256="), "Digest header: %s", digest)

	// Verify the digest value.
	sum := sha256.Sum256(body)
	expected := "SHA-256=" + base64.StdEncoding.EncodeToString(sum[:])
	require.Equal(t, expected, digest)

	// Signature header must be present and parseable.
	sig := req.Header.Get("Signature")
	require.NotEmpty(t, sig)
	require.Contains(t, sig, `keyId="`+keyID+`"`)
	require.Contains(t, sig, `algorithm="rsa-sha256"`)
	require.Contains(t, sig, `headers="(request-target) host date digest"`)
}

func TestSignRequest_SignatureVerifies(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	body := []byte(`{"type":"Follow"}`)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"http://example.com/users/alice/inbox", bytes.NewReader(body))
	require.NoError(t, err)

	keyID := "http://remote.example.com/users/carol#main-key"
	require.NoError(t, SignRequest(req, keyID, key))

	// Parse the Signature header manually.
	sigHeader := req.Header.Get("Signature")
	require.NotEmpty(t, sigHeader)

	params := parseSignatureHeaderForTest(t, sigHeader)

	// Reconstruct the signing string.
	signingStr := buildSigningString(req, params.headers)
	hash := sha256.Sum256([]byte(signingStr))

	sigBytes, err := base64.StdEncoding.DecodeString(params.signature)
	require.NoError(t, err)

	err = rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hash[:], sigBytes)
	require.NoError(t, err, "RSA-SHA256 signature must verify with the public key")
}

func TestSignRequest_NoBody(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://example.com/users/alice", nil)
	require.NoError(t, err)

	require.NoError(t, SignRequest(req, "http://remote.example.com/users/bob#main-key", key))

	// No body → no Digest header.
	require.Empty(t, req.Header.Get("Digest"))

	// Signature must NOT include digest in the headers list.
	sig := req.Header.Get("Signature")
	require.NotContains(t, sig, "digest")
}

// signatureTestParams holds parsed Signature header fields.
type signatureTestParams struct {
	keyID     string
	algorithm string
	headers   []string
	signature string
}

func parseSignatureHeaderForTest(t *testing.T, header string) signatureTestParams {
	t.Helper()
	params := signatureTestParams{}
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"`)
		switch k {
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
	require.NotEmpty(t, params.keyID, "keyId must be present")
	require.NotEmpty(t, params.signature, "signature must be present")
	return params
}
