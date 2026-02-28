// Package security provides shared security utilities for URL validation,
// SSRF prevention, and other cross-cutting security concerns.
package security

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// privateRanges defines CIDR blocks that are not allowed as request targets.
var privateRanges = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"169.254.0.0/16",
	"127.0.0.0/8",
	"::1/128",
	"fc00::/7",
	"fe80::/10",
}

// parsedPrivateRanges holds pre-parsed *net.IPNet entries for privateRanges.
var parsedPrivateRanges []*net.IPNet

func init() {
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("invalid CIDR %q: %v", cidr, err))
		}
		parsedPrivateRanges = append(parsedPrivateRanges, network)
	}
}

// ValidateExternalURL checks that a URL is safe to make requests to. It blocks
// localhost, loopback, link-local, and private/internal IP ranges to prevent
// SSRF attacks. Both HTTP and HTTPS schemes are allowed (unlike git which
// requires HTTPS only).
func ValidateExternalURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parsing URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme, got %q", parsed.Scheme)
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL has no hostname")
	}

	lower := strings.ToLower(hostname)
	if lower == "localhost" {
		return fmt.Errorf("URL must not target localhost")
	}

	// Check if hostname is a literal IP.
	if ip := net.ParseIP(hostname); ip != nil {
		return checkIP(ip)
	}

	// Resolve hostname and verify all resolved IPs.
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return fmt.Errorf("resolving hostname %q: %w", hostname, err)
	}

	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if err := checkIP(ip); err != nil {
			return fmt.Errorf("hostname %q resolves to blocked address: %w", hostname, err)
		}
	}

	return nil
}

// checkIP returns an error if the given IP falls within a private or blocked range.
func checkIP(ip net.IP) error {
	if ip.IsLoopback() {
		return fmt.Errorf("URL must not target loopback address %s", ip)
	}

	if ip.IsUnspecified() {
		return fmt.Errorf("URL must not target unspecified address %s", ip)
	}

	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("URL must not target link-local address %s", ip)
	}

	for _, network := range parsedPrivateRanges {
		if network.Contains(ip) {
			return fmt.Errorf("URL must not target private address %s (%s)", ip, network)
		}
	}

	return nil
}
