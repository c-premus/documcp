package security_test

import (
	"context"
	"net/http"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/security"
)

func TestValidateExternalURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		// Valid URLs
		{"valid https", "https://example.com", false},
		{"valid http", "http://example.com", false},
		{"valid with port", "https://example.com:8443/path", false},
		{"valid with path and query", "https://example.com/some/path?q=1", false},
		{"valid IP outside private ranges", "http://8.8.8.8", false},
		{"valid IP 1.1.1.1", "http://1.1.1.1", false},

		// Scheme validation
		{"ftp scheme blocked", "ftp://example.com", true},
		{"file scheme blocked", "file:///etc/passwd", true},
		{"javascript scheme blocked", "javascript:alert(1)", true},
		{"no scheme", "example.com", true},
		{"empty string", "", true},
		{"data scheme blocked", "data:text/html,<h1>hi</h1>", true},
		{"gopher scheme blocked", "gopher://evil.com", true},

		// Localhost
		{"localhost blocked", "http://localhost", true},
		{"localhost with port", "http://localhost:8080", true},
		{"LOCALHOST uppercase", "http://LOCALHOST", true},
		{"LocalHost mixed case", "http://LocalHost", true},

		// Loopback IPs (127.0.0.0/8)
		{"127.0.0.1 blocked", "http://127.0.0.1", true},
		{"127.0.0.2 blocked", "http://127.0.0.2", true},
		{"127.255.255.255 blocked", "http://127.255.255.255", true},
		{"IPv6 loopback blocked", "http://[::1]", true},

		// Private ranges - 10.0.0.0/8
		{"10.0.0.0 blocked", "http://10.0.0.0", true},
		{"10.0.0.1 blocked", "http://10.0.0.1", true},
		{"10.255.255.255 blocked", "http://10.255.255.255", true},
		{"11.0.0.0 allowed (just outside 10/8)", "http://11.0.0.0", false},
		{"9.255.255.255 allowed (just before 10/8)", "http://9.255.255.255", false},

		// Private ranges - 172.16.0.0/12
		{"172.16.0.0 blocked", "http://172.16.0.0", true},
		{"172.16.0.1 blocked", "http://172.16.0.1", true},
		{"172.31.255.255 blocked", "http://172.31.255.255", true},
		{"172.15.255.255 allowed (just outside range)", "http://172.15.255.255", false},
		{"172.32.0.1 allowed (just outside range)", "http://172.32.0.1", false},

		// Private ranges - 192.168.0.0/16
		{"192.168.0.0 blocked", "http://192.168.0.0", true},
		{"192.168.0.1 blocked", "http://192.168.0.1", true},
		{"192.168.255.255 blocked", "http://192.168.255.255", true},
		{"192.167.255.255 allowed (just before range)", "http://192.167.255.255", false},
		{"192.169.0.0 allowed (just after range)", "http://192.169.0.0", false},

		// Link-local - 169.254.0.0/16
		{"169.254.0.1 blocked", "http://169.254.0.1", true},
		{"169.254.169.254 blocked (AWS metadata)", "http://169.254.169.254", true},
		{"169.254.255.255 blocked", "http://169.254.255.255", true},
		{"169.253.255.255 allowed", "http://169.253.255.255", false},
		{"169.255.0.0 allowed", "http://169.255.0.0", false},

		// Unspecified address
		{"0.0.0.0 blocked", "http://0.0.0.0", true},
		{"[::] blocked", "http://[::]", true},

		// IPv6 private (fc00::/7 covers fc00:: and fd00::)
		{"fc00::1 blocked", "http://[fc00::1]", true},
		{"fd00::1 blocked", "http://[fd00::1]", true},
		{"fdff:ffff::1 blocked", "http://[fdff:ffff::1]", true},

		// IPv6 link-local (fe80::/10)
		{"fe80::1 link-local blocked", "http://[fe80::1]", true},
		{"febf::1 link-local blocked", "http://[febf::1]", true},

		// Empty/missing hostname
		{"no hostname", "http://", true},
		{"just scheme with path", "https:///path", true},

		// Edge cases
		{"URL with userinfo blocked private IP", "http://user:pass@10.0.0.1", true},
		{"IPv4-mapped IPv6 loopback", "http://[::ffff:127.0.0.1]", true},
		{"IPv4-mapped IPv6 private", "http://[::ffff:192.168.1.1]", true},
		{"IPv4-mapped IPv6 link-local", "http://[::ffff:169.254.1.1]", true},

		// Public IPv6 should be allowed
		{"public IPv6", "http://[2001:db8::1]", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := security.ValidateExternalURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExternalURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestSafeTransport(t *testing.T) {
	t.Run("returns a non-nil transport", func(t *testing.T) {
		tr := security.SafeTransport()
		if tr == nil {
			t.Fatal("SafeTransport() returned nil")
		}
		if tr.DialContext == nil {
			t.Fatal("SafeTransport() should set a custom DialContext")
		}
	})

	t.Run("blocks connection to loopback", func(t *testing.T) {
		client := &http.Client{Transport: security.SafeTransport()}
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:1/test", nil)
		resp, err := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		if err == nil {
			t.Fatal("expected error connecting to loopback via SafeTransport")
		}
	})

	t.Run("blocks connection to private IP", func(t *testing.T) {
		client := &http.Client{Transport: security.SafeTransport()}
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://10.0.0.1:1/test", nil)
		resp, err := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		if err == nil {
			t.Fatal("expected error connecting to private IP via SafeTransport")
		}
	})
}
