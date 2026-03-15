package ssrf

import (
	"context"
	"net/http"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSecureEgressHTTPClient(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient(HTTPClientOptions{})
	require.NotNil(t, client)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	res, err := client.Do(req)
	if res != nil {
		_ = res.Body.Close()
	}
	assert.NoError(t, err)
}

func TestNewSecureEgressHTTPClient_InvalidPort(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient(HTTPClientOptions{})
	require.NotNil(t, client)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com:8080", nil)
	require.NoError(t, err)

	res, err := client.Do(req)
	if res != nil {
		_ = res.Body.Close()
	}
	assert.ErrorIs(t, err, ErrInvalidPort)
}

func TestNewSecureEgressHTTPClient_InvalidAddress(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient(HTTPClientOptions{})
	require.NotNil(t, client)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)
	require.NoError(t, err)

	res, err := client.Do(req)
	if res != nil {
		_ = res.Body.Close()
	}
	require.ErrorIs(t, err, ErrInvalidAddress)

	req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1", nil)
	require.NoError(t, err)

	res, err = client.Do(req)
	if res != nil {
		_ = res.Body.Close()
	}
	assert.ErrorIs(t, err, ErrInvalidAddress)
}

func TestIsAllowedIPAddress(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, isAllowedIPAddress(netip.MustParseAddr("127.0.0.1")), ErrInvalidIPAddress)
	require.ErrorIs(t, isAllowedIPAddress(netip.MustParseAddr("192.168.1.1")), ErrInvalidIPAddress)
	require.ErrorIs(t, isAllowedIPAddress(netip.MustParseAddr("10.0.0.1")), ErrInvalidIPAddress)
	require.ErrorIs(t, isAllowedIPAddress(netip.MustParseAddr("172.16.0.1")), ErrInvalidIPAddress)
	require.ErrorIs(t, isAllowedIPAddress(netip.MustParseAddr("192.168.1.1")), ErrInvalidIPAddress)
	require.ErrorIs(t, isAllowedIPAddress(netip.MustParseAddr("192.168.1.1")), ErrInvalidIPAddress)
	assert.NoError(t, isAllowedIPAddress(netip.MustParseAddr("64.85.1.1")))
}
