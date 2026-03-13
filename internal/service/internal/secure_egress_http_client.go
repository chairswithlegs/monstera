package internal

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

var (
	ErrInvalidProtocol  = errors.New("invalid protocol")
	ErrInvalidNetwork   = errors.New("invalid network")
	ErrInvalidAddress   = errors.New("invalid address")
	ErrInvalidIPAddress = errors.New("invalid IP address")
	ErrInvalidPort      = errors.New("invalid port")
)

const (
	maxRedirects = 3
	timeout      = 5 * time.Second
)

// NewSecureEgressHTTPClient returns a new HTTP client configured for secure egress.
func NewSecureEgressHTTPClient() *http.Client {
	transport := newSecureEgressTransport()
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		Jar:       nil, // no cookies
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// secureEgressTransport is a transport that validates the request for secure egress.
// It enforces:
// - HTTP or HTTPS scheme
// - Allowed hostnames
type secureEgressTransport struct {
	http.Transport
}

func newSecureEgressTransport() *secureEgressTransport {
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: secureEgressControl,
	}

	return &secureEgressTransport{
		Transport: http.Transport{
			DialContext: dialer.DialContext,
		},
	}
}

func (t *secureEgressTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return nil, fmt.Errorf("%w: scheme %s is not allowed", ErrInvalidProtocol, req.URL.Scheme)
	}

	if req.URL.Host == "" {
		return nil, fmt.Errorf("%w: host is required", ErrInvalidAddress)
	}

	for _, re := range hostBlocklist {
		if re.MatchString(req.URL.Host) {
			return nil, fmt.Errorf("%w: host %s is in the blocked list", ErrInvalidAddress, req.URL.Host)
		}
	}

	res, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("secure egress transport round trip: %w", err)
	}
	return res, nil
}

// secureEgressControl validates the network and address for secure egress.
// It enforces:
// - TCP only
// - Allowed ports: 80, 443
// - Denied IP prefixes
// - Limited redirects
// - No cookies
func secureEgressControl(network, addr string, _ syscall.RawConn) error {
	if network != "tcp4" && network != "tcp6" {
		return fmt.Errorf("%w: network %s is not allowed", ErrInvalidNetwork, network)
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidAddress, err)
	}

	if port != "80" && port != "443" {
		return fmt.Errorf("%w: port %s is not allowed", ErrInvalidPort, port)
	}

	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidAddress, err)
	}

	return isAllowedIPAddress(ip)
}

func isAllowedIPAddress(ip netip.Addr) error {
	if ip.Is4() {
		for _, prefix := range ipv4Blocklist {
			if prefix.Contains(ip) {
				return fmt.Errorf("%w: IP address %s is in the denied prefix list", ErrInvalidIPAddress, ip)
			}
		}
	} else {
		// Note: this also includes IPv6, which is not supported
		return fmt.Errorf("%w: %s", ErrInvalidIPAddress, ip)
	}
	return nil
}
