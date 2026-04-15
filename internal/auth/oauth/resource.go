package oauth

import (
	"errors"
	"fmt"
	"net/url"
)

// ErrInvalidResource is returned when a `resource` parameter fails RFC 8707
// validation: not an absolute URI, contains a fragment, uses a forbidden
// scheme, or does not match the server's allowlist.
var ErrInvalidResource = errors.New("invalid resource indicator")

// ValidateResource checks a single RFC 8707 `resource` parameter value.
//
// The value must be an absolute URI without a fragment (RFC 8707 §2), use
// either https or loopback http (RFC 8707 §2.1 plus the OAuth 2.1 transport
// requirement), and exactly match one of the configured allowed resources.
//
// Returns the canonical form (the matched allowlist entry) on success.
func ValidateResource(raw string, allowed []string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("%w: empty value", ErrInvalidResource)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: parse: %w", ErrInvalidResource, err)
	}
	if !u.IsAbs() {
		return "", fmt.Errorf("%w: must be absolute URI", ErrInvalidResource)
	}
	if u.Fragment != "" || u.RawFragment != "" {
		return "", fmt.Errorf("%w: must not contain a fragment", ErrInvalidResource)
	}
	switch u.Scheme {
	case "https":
		// always allowed
	case "http":
		if !isLoopback(u.Hostname()) {
			return "", fmt.Errorf("%w: http only permitted for loopback", ErrInvalidResource)
		}
	default:
		return "", fmt.Errorf("%w: unsupported scheme %q", ErrInvalidResource, u.Scheme)
	}

	for _, candidate := range allowed {
		if raw == candidate {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%w: not in allowlist", ErrInvalidResource)
}
