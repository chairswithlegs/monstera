package internal

import (
	"net/netip"
	"regexp"
)

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
