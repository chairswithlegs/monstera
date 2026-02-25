package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// ValidatePKCE verifies a PKCE code_verifier against the stored code_challenge.
//
// Only the S256 method is supported. The verification computes:
//
//	base64url_no_pad(sha256(code_verifier)) == code_challenge
//
// If code_challenge is empty (no PKCE was used during authorization), the
// verifier is not checked and nil is returned. This allows the non-PKCE
// Authorization Code flow for server-side clients.
//
// Returns a non-nil error if:
//   - code_challenge_method is non-empty and not "S256"
//   - code_verifier is empty when a challenge was issued
//   - the computed challenge does not match
func ValidatePKCE(codeChallenge, codeChallengeMethod, codeVerifier string) error {
	if codeChallenge == "" {
		return nil
	}

	if codeChallengeMethod != "S256" {
		return fmt.Errorf("unsupported code_challenge_method %q: only S256 is supported", codeChallengeMethod)
	}

	if codeVerifier == "" {
		return errors.New("code_verifier is required when code_challenge is present")
	}

	h := sha256.Sum256([]byte(codeVerifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])

	if computed != codeChallenge {
		return errors.New("code_verifier does not match code_challenge")
	}

	return nil
}

// GenerateCodeChallenge is a test helper that computes the S256 challenge
// for a given verifier. Not used in production code.
func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
