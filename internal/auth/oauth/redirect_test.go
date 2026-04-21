package oauth

import "testing"

// ---------------------------------------------------------------------------
// TestIsLoopbackHost
// ---------------------------------------------------------------------------

func TestIsLoopbackHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		hostname string
		want     bool
	}{
		// RFC 8252 §7.3 — "localhost" is DNS-resolvable and therefore
		// hijackable; we only accept numeric loopback.
		{name: "localhost returns false (DNS-hijack risk)", hostname: "localhost", want: false},
		{name: "IPv4 loopback returns true", hostname: "127.0.0.1", want: true},
		{name: "IPv6 loopback returns true", hostname: "::1", want: true},
		{name: "public domain returns false", hostname: "example.com", want: false},
		{name: "empty string returns false", hostname: "", want: false},
		{name: "private IP returns false", hostname: "192.168.1.1", want: false},
		{name: "127.0.0.2 returns true", hostname: "127.0.0.2", want: true},
		{name: "0.0.0.0 returns false", hostname: "0.0.0.0", want: false},
		{name: "IPv6 non-loopback returns false", hostname: "::2", want: false},
		{name: "10.0.0.1 returns false", hostname: "10.0.0.1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsLoopbackHost(tt.hostname)
			if got != tt.want {
				t.Errorf("IsLoopbackHost(%q) = %v, want %v", tt.hostname, got, tt.want)
			}
		})
	}
}
