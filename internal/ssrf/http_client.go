// Package ssrf provides a locked-down HTTP client that is intended to provide
// defense in-depth against SSRF attacks by restricting outbound requests based
// on a number of pre-defined rules.
package ssrf

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"regexp"
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
	maxRedirects   = 3
	defaultTimeout = 5 * time.Second
)

type HTTPClientOptions struct {
	Timeout time.Duration
}

// NewHTTPClient returns a new HTTP client configured for secure egress.
func NewHTTPClient(opts HTTPClientOptions) *http.Client {
	if opts.Timeout == 0 {
		opts.Timeout = defaultTimeout
	}

	transport := newSecureEgressTransport(opts.Timeout)
	return &http.Client{
		Transport: transport,
		Timeout:   opts.Timeout,
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

func newSecureEgressTransport(timeout time.Duration) *secureEgressTransport {
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

// Special thanks to https://github.com/daenney/ssrf for providing the list.
// https://github.com/daenney/ssrf/blob/main/ssrf_gen.go
var (
	// ipv4Blocklist contains IPv4 special purpose IP prefixes from IANA
	// as well as a number of other prefixes we wish to block by default
	// https://www.iana.org/assignments/iana-ipv4-special-registry/iana-ipv4-special-registry.xhtml
	ipv4Blocklist = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),       // "This network" (RFC 791, Section 3.2)
		netip.MustParsePrefix("10.0.0.0/8"),      // Private-Use (RFC 1918)
		netip.MustParsePrefix("100.64.0.0/10"),   // Shared Address Space (RFC 6598)
		netip.MustParsePrefix("127.0.0.0/8"),     // Loopback (RFC 1122, Section 3.2.1.3)
		netip.MustParsePrefix("169.254.0.0/16"),  // Link Local (RFC 3927)
		netip.MustParsePrefix("172.16.0.0/12"),   // Private-Use (RFC 1918)
		netip.MustParsePrefix("192.0.0.0/24"),    // IETF Protocol Assignments (RFC 6890, Section 2.1)
		netip.MustParsePrefix("192.0.2.0/24"),    // Documentation (TEST-NET-1) (RFC 5737)
		netip.MustParsePrefix("192.31.196.0/24"), // AS112-v4 (RFC 7535)
		netip.MustParsePrefix("192.52.193.0/24"), // AMT (RFC 7450)
		netip.MustParsePrefix("192.88.99.0/24"),  // Deprecated (6to4 Relay Anycast) (RFC 7526)
		netip.MustParsePrefix("192.168.0.0/16"),  // Private-Use (RFC 1918)
		netip.MustParsePrefix("192.175.48.0/24"), // Direct Delegation AS112 Service (RFC 7534)
		netip.MustParsePrefix("198.18.0.0/15"),   // Benchmarking (RFC 2544)
		netip.MustParsePrefix("198.51.100.0/24"), // Documentation (TEST-NET-2) (RFC 5737)
		netip.MustParsePrefix("203.0.113.0/24"),  // Documentation (TEST-NET-3) (RFC 5737)
		netip.MustParsePrefix("240.0.0.0/4"),     // Reserved (RFC 1112, Section 4)
		netip.MustParsePrefix("224.0.0.0/4"),     // Multicast (RFC 1112, Section 4)
	}
)

// hostBlocklist contains a list of hostnames to block.
var hostBlocklist = []*regexp.Regexp{
	// Block localhost
	regexp.MustCompile(`^localhost$`),
	// Block .internal domains and subdomains
	regexp.MustCompile(`^[^.]+\.internal$`),
	regexp.MustCompile(`^[^.]+\.internal\..+$`),
	// Block .local domains and subdomains
	regexp.MustCompile(`^[^.]+\.local$`),
	regexp.MustCompile(`^[^.]+\.local\..+$`),
	// Block raw IP addresses
	regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$`),
}
