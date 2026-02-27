package federation

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chairswithlegs/monstera-fed/internal/domain"
)

func TestParseRSAPrivateKeyPEM(t *testing.T) {
	t.Parallel()
	privPEM, err := generateTestKeyPair()
	require.NoError(t, err)
	key, err := parseRSAPrivateKeyPEM(privPEM)
	require.NoError(t, err)
	require.NotNil(t, key)
}

func TestParseRSAPrivateKeyPEM_invalid(t *testing.T) {
	t.Parallel()
	_, err := parseRSAPrivateKeyPEM("not-pem")
	require.Error(t, err)
	_, err = parseRSAPrivateKeyPEM("-----BEGIN RSA PRIVATE KEY-----\ninvalid\n-----END RSA PRIVATE KEY-----")
	require.Error(t, err)
}

func TestSubjectToActivityType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "create", subjectToActivityType("federation.deliver.create"))
	assert.Equal(t, "accept", subjectToActivityType("federation.deliver.accept"))
	assert.Equal(t, "unknown", subjectToActivityType("other.subject"))
}

func TestDomainFromURL(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "remote.example.com", domainFromURL("https://remote.example.com/inbox"))
	assert.Empty(t, domainFromURL("not-a-url"))
}

func TestActorKeyID(t *testing.T) {
	t.Parallel()
	acc := &domain.Account{APID: "https://example.com/users/alice", Username: "alice"}
	assert.Equal(t, "https://example.com/users/alice#main-key", actorKeyID(acc, "example.com"))
	acc.APID = ""
	assert.Equal(t, "https://example.com/users/alice#main-key", actorKeyID(acc, "example.com"))
}

// generateTestKeyPair returns PEM-encoded private key for testing.
func generateTestKeyPair() (privPEM string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return string(pem.EncodeToMemory(privBlock)), nil
}
