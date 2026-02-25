package oauth

import (
	"net"
	"net/url"
)

// MatchRedirectURI checks whether requestURI matches one of the registered URIs.
// For localhost/loopback URIs, any port is allowed per RFC 8252 Section 7.3.
func MatchRedirectURI(requestURI string, registeredURIs []string) bool {
	req, err := url.Parse(requestURI)
	if err != nil {
		return false
	}

	for _, registered := range registeredURIs {
		reg, err := url.Parse(registered)
		if err != nil {
			continue
		}

		if isLoopback(reg.Hostname()) {
			// For loopback: scheme and path must match, port may differ
			if req.Scheme == reg.Scheme && req.Path == reg.Path && isLoopback(req.Hostname()) {
				return true
			}
		} else {
			// For non-loopback: exact match required
			if requestURI == registered {
				return true
			}
		}
	}
	return false
}

// isLoopback checks if a hostname is a loopback address (localhost, 127.0.0.1, [::1]).
func isLoopback(hostname string) bool {
	if hostname == "localhost" {
		return true
	}
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}
