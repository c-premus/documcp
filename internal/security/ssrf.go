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

// privateRanges defines CIDR blocks that are not allowed as request targets
// unless the caller opts in via allowPrivate. RFC 1918 (the classic private
// IPv4 space) is joined by several ranges that are equally unsafe but that
// Go's IsPrivate helper omits: carrier-grade NAT (RFC 6598, used by Tailscale
// and Oracle internal), IETF benchmark/test/documentation space, and
// multicast/future-reserved. Loopback (127/8, ::1) is still listed here for
// belt-and-braces even though checkIP also calls IsLoopback unconditionally.
// Link-local remains handled separately via IsLinkLocalUnicast/Multicast.
//
// Rationale for each addition (security.md M5):
//   - 100.64/10    — CGN (RFC 6598); common inside Tailscale, Oracle Cloud,
//                    and carrier networks; not "private" by Go's definition.
//   - 192.0.2/24,
//     198.51.100/24,
//     203.0.113/24 — TEST-NET-1/2/3 (RFC 5737); documentation ranges that
//                    some broken DNS resolvers hand out unexpectedly.
//   - 192.0.0/24   — IETF protocol assignments (RFC 6890); no legitimate
//                    destination value.
//   - 198.18/15    — benchmarking (RFC 2544); should never be a real target.
//   - 224/4        — multicast (RFC 5771); sending HTTP to a multicast
//                    address is a misconfiguration at best.
//   - 240/4        — reserved class E (RFC 1112); treat as untargetable.
var privateRanges = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"169.254.0.0/16",
	"127.0.0.0/8",
	"100.64.0.0/10",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"224.0.0.0/4",
	"240.0.0.0/4",
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
// services on internal networks (e.g. self-hosted Kiwix).
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

	var validCount int
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		// Skip unspecified (::, 0.0.0.0) — these are DNS artifacts from
		// misconfigured AAAA records, not real targets.
		if ip.IsUnspecified() {
			continue
		}
		if err := checkIP(ip, privateOK); err != nil {
			return fmt.Errorf("hostname %q resolves to blocked address: %w", hostname, err)
		}
		validCount++
	}

	if validCount == 0 {
		return fmt.Errorf("hostname %q has no usable IP addresses", hostname)
	}

	return nil
}

// SafeTransportAllowPrivate returns an *http.Transport whose DialContext hook
// re-validates every resolved IP against the SSRF block-list at connection
// time, preventing DNS rebinding attacks. Private RFC-1918 addresses are
// permitted for admin-configured services on internal networks (e.g.
// self-hosted Kiwix). Loopback and link-local addresses remain blocked.
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

		// Filter to usable IPs: skip unspecified (::, 0.0.0.0) DNS artifacts.
		usable := make([]net.IPAddr, 0, len(ips))
		for _, ipAddr := range ips {
			if ipAddr.IP.IsUnspecified() {
				continue
			}
			if err := checkIP(ipAddr.IP, allowPrivate); err != nil {
				return nil, fmt.Errorf("connection to %s blocked: %w", addr, err)
			}
			usable = append(usable, ipAddr)
		}

		if len(usable) == 0 {
			return nil, fmt.Errorf("host %q has no usable IP addresses", host)
		}

		// Connect to the first valid resolved IP to avoid TOCTOU with the OS resolver.
		return dialer.DialContext(ctx, network, net.JoinHostPort(usable[0].IP.String(), port))
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
