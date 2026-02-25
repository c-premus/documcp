package git

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// privateRanges defines CIDR blocks that are not allowed as clone targets.
var privateRanges = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"169.254.0.0/16",
	"127.0.0.0/8",
	"::1/128",
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

// ValidateRepositoryURL checks that a repository URL is safe to clone.
// It blocks localhost, loopback, private IPs, and non-HTTPS URLs.
func ValidateRepositoryURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parsing repository URL: %w", err)
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("repository URL must use https scheme, got %q", parsed.Scheme)
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return fmt.Errorf("repository URL has no hostname")
	}

	lower := strings.ToLower(hostname)
	if lower == "localhost" {
		return fmt.Errorf("repository URL must not target localhost")
	}

	// Check if hostname is a literal IP.
	if ip := net.ParseIP(hostname); ip != nil {
		if err := checkIP(ip); err != nil {
			return err
		}
		return nil
	}

	// Resolve hostname and verify all resolved IPs.
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return fmt.Errorf("resolving repository hostname %q: %w", hostname, err)
	}

	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if err := checkIP(ip); err != nil {
			return fmt.Errorf("repository hostname %q resolves to blocked address: %w", hostname, err)
		}
	}

	return nil
}

// checkIP returns an error if the given IP falls within a private or blocked range.
func checkIP(ip net.IP) error {
	if ip.IsLoopback() {
		return fmt.Errorf("repository URL must not target loopback address %s", ip)
	}

	if ip.IsUnspecified() {
		return fmt.Errorf("repository URL must not target unspecified address %s", ip)
	}

	for _, network := range parsedPrivateRanges {
		if network.Contains(ip) {
			return fmt.Errorf("repository URL must not target private address %s (%s)", ip, network)
		}
	}

	return nil
}

// sensitivePatterns matches URLs, bearer tokens, and base64-encoded credentials
// that may appear in git error output.
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`https?://[^\s]+`),
	regexp.MustCompile(`(?i)token\s*[=:]\s*\S+`),
	regexp.MustCompile(`(?i)authorization:\s*\S+`),
	regexp.MustCompile(`(?i)bearer\s+\S+`),
	regexp.MustCompile(`[A-Za-z0-9+/]{40,}={0,2}`),
}

// SanitizeGitError removes URLs, tokens, and base64 credentials from git error messages.
func SanitizeGitError(errMsg string) string {
	sanitized := errMsg
	for _, re := range sensitivePatterns {
		sanitized = re.ReplaceAllString(sanitized, "[REDACTED]")
	}
	return sanitized
}
