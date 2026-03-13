package cli

// signer.go — self-contained HTTP Signature signing using stdlib only.
// Replicates internal/activitypub/httpsignature.go sign() (lines 213–252).

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SignRequest signs r using keyID and privateKey following draft-cavage-http-signatures-12.
// Signed headers: (request-target), host, date, digest (when body present).
func SignRequest(r *http.Request, keyID string, privateKey *rsa.PrivateKey) error {
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

		sum := sha256.Sum256(body)
		digest := "SHA-256=" + base64.StdEncoding.EncodeToString(sum[:])
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

func buildSigningString(r *http.Request, headers []string) string {
	var lines []string
	for _, h := range headers {
		switch h {
		case "(request-target)":
			lines = append(lines, "(request-target): "+strings.ToLower(r.Method)+" "+r.URL.RequestURI())
		case "host":
			lines = append(lines, "host: "+r.Host)
		default:
			lines = append(lines, fmt.Sprintf("%s: %s", strings.ToLower(h), r.Header.Get(h)))
		}
	}
	return strings.Join(lines, "\n")
}
