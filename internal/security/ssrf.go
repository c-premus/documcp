// Package security provides shared security utilities for URL validation,
// SSRF prevention, and other cross-cutting security concerns.
package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
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
var parsedPrivateRanges = mustParseCIDRs(privateRanges)

func mustParseCIDRs(cidrs []string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("invalid CIDR %q: %v", cidr, err))
		}
		nets = append(nets, network)
	}
	return nets
}

// ValidateExternalURL checks that a URL is safe to make requests to. It blocks
// localhost, loopback, and link-local addresses. Private RFC-1918 ranges are
// also blocked unless allowPrivate is true — use that for admin-configured
// services on internal networks (e.g. self-hosted Kiwix or Confluence).
func ValidateExternalURL(rawURL string, allowPrivate ...bool) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parsing URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme, got %q", parsed.Scheme)
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return errors.New("URL has no hostname")
	}

	lower := strings.ToLower(hostname)
	if lower == "localhost" {
		return errors.New("URL must not target localhost")
	}

	privateOK := len(allowPrivate) > 0 && allowPrivate[0]

	// Check if hostname is a literal IP.
	if ip := net.ParseIP(hostname); ip != nil {
		return checkIP(ip, privateOK)
	}

	// Resolve hostname and verify all resolved IPs.
	addrs, err := net.DefaultResolver.LookupHost(context.Background(), hostname)
	if err != nil {
		return fmt.Errorf("resolving hostname %q: %w", hostname, err)
	}

	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if err := checkIP(ip, privateOK); err != nil {
			return fmt.Errorf("hostname %q resolves to blocked address: %w", hostname, err)
		}
	}

	return nil
}

// SafeTransport returns an *http.Transport whose DialContext hook re-validates
// every resolved IP against the SSRF block-list at connection time, preventing
// DNS rebinding attacks. Private RFC-1918 ranges are blocked.
// The dialerTimeout controls the TCP connection timeout.
func SafeTransport(dialerTimeout time.Duration) *http.Transport {
	return newSafeTransport(false, dialerTimeout)
}

// SafeTransportAllowPrivate is like SafeTransport but permits connections to
// private RFC-1918 addresses. Use for admin-configured services on internal
// networks (e.g. self-hosted Kiwix or Confluence). Loopback and link-local
// addresses remain blocked.
// The dialerTimeout controls the TCP connection timeout.
func SafeTransportAllowPrivate(dialerTimeout time.Duration) *http.Transport {
	return newSafeTransport(true, dialerTimeout)
}

func newSafeTransport(allowPrivate bool, dialerTimeout time.Duration) *http.Transport {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		transport = &http.Transport{}
	}
	base := transport.Clone()
	dialer := &net.Dialer{Timeout: dialerTimeout}

	base.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("splitting host:port %q: %w", addr, err)
		}

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolving %q: %w", host, err)
		}

		for _, ipAddr := range ips {
			if err := checkIP(ipAddr.IP, allowPrivate); err != nil {
				return nil, fmt.Errorf("connection to %s blocked: %w", addr, err)
			}
		}

		// Connect to the first valid resolved IP to avoid TOCTOU with the OS resolver.
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
	return base
}

// checkIP returns an error if the given IP falls within a blocked range.
// Loopback, unspecified, and link-local addresses are always blocked.
// Private RFC-1918 ranges are only blocked when allowPrivate is false.
func checkIP(ip net.IP, allowPrivate bool) error {
	if ip.IsLoopback() {
		return fmt.Errorf("URL must not target loopback address %s", ip)
	}

	if ip.IsUnspecified() {
		return fmt.Errorf("URL must not target unspecified address %s", ip)
	}

	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("URL must not target link-local address %s", ip)
	}

	if !allowPrivate {
		for _, network := range parsedPrivateRanges {
			if network.Contains(ip) {
				return fmt.Errorf("URL must not target private address %s (%s)", ip, network)
			}
		}
	}

	return nil
}
