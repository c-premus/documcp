package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mustParseCIDR is a test helper that parses a CIDR string or panics.
func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func TestExtractIP_TrustedProxies(t *testing.T) {
	trusted := []*net.IPNet{mustParseCIDR("10.0.0.0/8")}

	tests := []struct {
		name           string
		xRealIP        string
		xff            string
		remoteAddr     string
		trustedProxies []*net.IPNet
		want           string
	}{
		{
			name:           "X-Real-IP from trusted proxy",
			xRealIP:        "203.0.113.1",
			xff:            "203.0.113.2",
			remoteAddr:     "10.0.0.3:12345",
			trustedProxies: trusted,
			want:           "203.0.113.1",
		},
		{
			name:           "X-Forwarded-For from trusted proxy",
			xff:            "203.0.113.2",
			remoteAddr:     "10.0.0.3:12345",
			trustedProxies: trusted,
			want:           "203.0.113.2",
		},
		{
			name:           "X-Forwarded-For multiple uses rightmost untrusted",
			xff:            "203.0.113.1, 203.0.113.2, 203.0.113.3",
			remoteAddr:     "10.0.0.5:12345",
			trustedProxies: trusted,
			want:           "203.0.113.3",
		},
		{
			name:           "X-Forwarded-For spoofed IP ignored",
			xff:            "1.2.3.4, 203.0.113.5, 10.0.0.2",
			remoteAddr:     "10.0.0.1:12345",
			trustedProxies: trusted,
			want:           "203.0.113.5",
		},
		{
			name:           "RemoteAddr when trusted proxy but no headers",
			remoteAddr:     "10.0.0.1:54321",
			trustedProxies: trusted,
			want:           "10.0.0.1",
		},
		{
			name:           "headers ignored when RemoteAddr not in trusted CIDR",
			xRealIP:        "10.0.0.1",
			xff:            "10.0.0.2",
			remoteAddr:     "192.168.1.1:54321",
			trustedProxies: trusted,
			want:           "192.168.1.1",
		},
		{
			name:       "headers ignored when no trusted proxies",
			xRealIP:    "10.0.0.1",
			xff:        "10.0.0.2",
			remoteAddr: "192.168.1.1:54321",
			want:       "192.168.1.1",
		},
		{
			name:       "RemoteAddr when no trusted proxies",
			remoteAddr: "192.168.1.1:54321",
			want:       "192.168.1.1",
		},
		{
			name:           "invalid X-Real-IP falls back to RemoteAddr",
			xRealIP:        "not-an-ip",
			remoteAddr:     "10.0.0.1:54321",
			trustedProxies: trusted,
			want:           "10.0.0.1",
		},
		{
			name:           "invalid X-Forwarded-For falls back to RemoteAddr",
			xff:            "garbage, 10.0.0.1",
			remoteAddr:     "10.0.0.5:54321",
			trustedProxies: trusted,
			want:           "10.0.0.5",
		},
		{
			name:           "all X-Forwarded-For IPs trusted falls back to RemoteAddr",
			xff:            "10.0.0.2, 10.0.0.3",
			remoteAddr:     "10.0.0.1:54321",
			trustedProxies: trusted,
			want:           "10.0.0.1",
		},
		{
			name:           "multiple CIDRs - second matches",
			xRealIP:        "203.0.113.1",
			remoteAddr:     "172.16.0.1:12345",
			trustedProxies: []*net.IPNet{mustParseCIDR("10.0.0.0/8"), mustParseCIDR("172.16.0.0/12")},
			want:           "203.0.113.1",
		},
		{
			name:           "bare /32 trusted proxy",
			xRealIP:        "203.0.113.1",
			remoteAddr:     "10.0.0.1:12345",
			trustedProxies: []*net.IPNet{mustParseCIDR("10.0.0.1/32")},
			want:           "203.0.113.1",
		},
		{
			name:           "bare /32 does not match different IP",
			xRealIP:        "203.0.113.1",
			remoteAddr:     "10.0.0.2:12345",
			trustedProxies: []*net.IPNet{mustParseCIDR("10.0.0.1/32")},
			want:           "10.0.0.2",
		},
		// IPv6 tests.
		{
			name:       "IPv6 RemoteAddr with brackets and port",
			remoteAddr: "[2001:db8::1]:54321",
			want:       "2001:db8::1",
		},
		{
			name:       "IPv6 loopback RemoteAddr",
			remoteAddr: "[::1]:12345",
			want:       "::1",
		},
		{
			name:           "IPv6 X-Real-IP from trusted IPv6 proxy",
			xRealIP:        "2001:db8::99",
			remoteAddr:     "[fd00::1]:12345",
			trustedProxies: []*net.IPNet{mustParseCIDR("fd00::/8")},
			want:           "2001:db8::99",
		},
		{
			name:           "IPv6 X-Forwarded-For from trusted proxy",
			xff:            "2001:db8::42",
			remoteAddr:     "[fd00::1]:12345",
			trustedProxies: []*net.IPNet{mustParseCIDR("fd00::/8")},
			want:           "2001:db8::42",
		},
		{
			name:           "non-canonical IPv6 in X-Real-IP normalized",
			xRealIP:        "2001:0db8:0000:0000:0000:0000:0000:0001",
			remoteAddr:     "10.0.0.1:12345",
			trustedProxies: trusted,
			want:           "2001:db8::1",
		},
		{
			name:           "non-canonical IPv6 in X-Forwarded-For normalized",
			xff:            "2001:0db8:0000:0000:0000:0000:0000:0001",
			remoteAddr:     "10.0.0.1:12345",
			trustedProxies: trusted,
			want:           "2001:db8::1",
		},
		{
			name:           "IPv6 X-Forwarded-For multiple uses rightmost untrusted",
			xff:            "2001:db8::1, 2001:db8::2, 2001:db8::3",
			remoteAddr:     "[fd00::1]:12345",
			trustedProxies: []*net.IPNet{mustParseCIDR("fd00::/8")},
			want:           "2001:db8::3",
		},
		{
			name:           "IPv6 trusted proxy with IPv4 X-Real-IP",
			xRealIP:        "203.0.113.5",
			remoteAddr:     "[fd00::1]:12345",
			trustedProxies: []*net.IPNet{mustParseCIDR("fd00::/8")},
			want:           "203.0.113.5",
		},
		{
			name:           "IPv4 trusted proxy with IPv6 X-Real-IP",
			xRealIP:        "2001:db8::99",
			remoteAddr:     "10.0.0.1:12345",
			trustedProxies: trusted,
			want:           "2001:db8::99",
		},
		{
			name:           "headers ignored when IPv6 RemoteAddr not in trusted CIDR",
			xRealIP:        "203.0.113.1",
			remoteAddr:     "[2001:db8::1]:12345",
			trustedProxies: []*net.IPNet{mustParseCIDR("fd00::/8")},
			want:           "2001:db8::1",
		},
		{
			name:           "both X-Real-IP and X-Forwarded-For invalid falls back to RemoteAddr",
			xRealIP:        "not-an-ip",
			xff:            "also-not-an-ip",
			remoteAddr:     "10.0.0.1:12345",
			trustedProxies: trusted,
			want:           "10.0.0.1",
		},
		{
			name:           "X-Real-IP with whitespace trimmed",
			xRealIP:        "  203.0.113.1  ",
			remoteAddr:     "10.0.0.1:12345",
			trustedProxies: trusted,
			want:           "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tt.xRealIP != "" {
				r.Header.Set("X-Real-Ip", tt.xRealIP)
			}
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.remoteAddr != "" {
				r.RemoteAddr = tt.remoteAddr
			}
			if got := extractIP(r, tt.trustedProxies); got != tt.want {
				t.Errorf("extractIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRealIP_Middleware_SetsRemoteAddr(t *testing.T) {
	trusted := []*net.IPNet{mustParseCIDR("10.0.0.0/8")}

	var capturedAddr string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAddr = r.RemoteAddr
	})

	handler := RealIP(trusted)(inner)

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	r.Header.Set("X-Real-Ip", "203.0.113.50")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if capturedAddr != "203.0.113.50" {
		t.Errorf("RealIP middleware set RemoteAddr = %q, want %q", capturedAddr, "203.0.113.50")
	}
}

func TestRealIP_Middleware_NoTrustedProxies(t *testing.T) {
	var capturedAddr string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAddr = r.RemoteAddr
	})

	handler := RealIP(nil)(inner)

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:54321"
	r.Header.Set("X-Real-Ip", "1.2.3.4")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if capturedAddr != "192.168.1.1" {
		t.Errorf("RealIP middleware set RemoteAddr = %q, want %q", capturedAddr, "192.168.1.1")
	}
}

func TestIpInNets(t *testing.T) {
	nets := []*net.IPNet{
		mustParseCIDR("10.0.0.0/8"),
		mustParseCIDR("172.16.0.0/12"),
	}

	tests := []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", false},
		{"8.8.8.8", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if got := ipInNets(ip, nets); got != tt.want {
				t.Errorf("ipInNets(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestIpInNets_EmptyNets(t *testing.T) {
	ip := net.ParseIP("10.0.0.1")
	if ipInNets(ip, nil) {
		t.Error("ipInNets should return false for nil nets")
	}
	if ipInNets(ip, []*net.IPNet{}) {
		t.Error("ipInNets should return false for empty nets")
	}
}
