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

// IsLoopbackHost checks if a hostname is a numeric loopback address
// (127.0.0.1 or ::1). The literal "localhost" is deliberately rejected —
// RFC 8252 §7.3 prefers numeric loopback because "localhost" is
// DNS-resolvable and overridable by LAN DHCP, a malicious hosts file, or
// browser extensions, creating a DNS-hijack redirect path.
func IsLoopbackHost(hostname string) bool {
	return isLoopback(hostname)
}

// isLoopback returns true only for numeric loopback addresses.
func isLoopback(hostname string) bool {
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}
