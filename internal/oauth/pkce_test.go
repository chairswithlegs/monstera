package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePKCE_noChallenge(t *testing.T) {
	err := ValidatePKCE("", "", "")
	require.NoError(t, err)
	err = ValidatePKCE("", "S256", "any")
	require.NoError(t, err)
}

func TestValidatePKCE_S256_roundTrip(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := GenerateCodeChallenge(verifier)
	require.NotEmpty(t, challenge)
	err := ValidatePKCE(challenge, "S256", verifier)
	require.NoError(t, err)
}

func TestValidatePKCE_wrongVerifier(t *testing.T) {
	verifier := "correct_verifier"
	challenge := GenerateCodeChallenge(verifier)
	err := ValidatePKCE(challenge, "S256", "wrong_verifier")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestValidatePKCE_unsupportedMethod(t *testing.T) {
	err := ValidatePKCE("challenge", "plain", "verifier")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only S256 is supported")
	err = ValidatePKCE("challenge", "HS256", "verifier")
	require.Error(t, err)
}

func TestValidatePKCE_emptyVerifierWithChallenge(t *testing.T) {
	err := ValidatePKCE("someChallenge", "S256", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "code_verifier is required")
}

func TestGenerateCodeChallenge_deterministic(t *testing.T) {
	v := "verifier123"
	ch1 := GenerateCodeChallenge(v)
	ch2 := GenerateCodeChallenge(v)
	assert.Equal(t, ch1, ch2)
}

func TestGenerateCodeChallenge_base64urlNoPad(t *testing.T) {
	challenge := GenerateCodeChallenge("x")
	require.NotEmpty(t, challenge)
	assert.NotContains(t, challenge, "+")
	assert.NotContains(t, challenge, "/")
	assert.NotContains(t, challenge, "=")
}
