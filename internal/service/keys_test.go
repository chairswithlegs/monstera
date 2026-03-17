package service

import (
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateRSAKeyPair(t *testing.T) {
	t.Parallel()

	t.Run("generates valid PEM pair", func(t *testing.T) {
		t.Parallel()
		pub, priv, err := generateRSAKeyPair()
		require.NoError(t, err)
		assert.NotEmpty(t, pub)
		assert.NotEmpty(t, priv)

		pubBlock, _ := pem.Decode([]byte(pub))
		assert.NotNil(t, pubBlock)

		privBlock, _ := pem.Decode([]byte(priv))
		assert.NotNil(t, privBlock)
	})

	t.Run("public key is PKIX format", func(t *testing.T) {
		t.Parallel()
		pub, _, err := generateRSAKeyPair()
		require.NoError(t, err)

		block, _ := pem.Decode([]byte(pub))
		require.NotNil(t, block)
		assert.Equal(t, "PUBLIC KEY", block.Type)
	})

	t.Run("private key is PKCS1 format", func(t *testing.T) {
		t.Parallel()
		_, priv, err := generateRSAKeyPair()
		require.NoError(t, err)

		block, _ := pem.Decode([]byte(priv))
		require.NotNil(t, block)
		assert.Equal(t, "RSA PRIVATE KEY", block.Type)
	})
}
