package git

import (
	"fmt"
	"net/url"
	"regexp"

	"git.999.haus/chris/DocuMCP-go/internal/security"
)

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

	if err := security.ValidateExternalURL(rawURL); err != nil {
		return fmt.Errorf("repository URL blocked: %w", err)
	}

	return nil
}

// sensitivePatterns matches URLs, bearer tokens, and base64-encoded credentials
// that may appear in git error output.
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`https?://\S+`),
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
