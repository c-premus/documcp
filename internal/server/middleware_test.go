package server

import (
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
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
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
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

// ---------------------------------------------------------------------------
// internalTokenAuth
// ---------------------------------------------------------------------------

func TestInternalTokenAuth_ValidToken(t *testing.T) {
	t.Parallel()

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := internalTokenAuth("my-secret-token")(inner)

	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Bearer my-secret-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !called {
		t.Error("expected inner handler to be called")
	}
}

func TestInternalTokenAuth_WrongToken(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	})

	handler := internalTokenAuth("correct-token")(inner)

	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestInternalTokenAuth_NoHeader(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	})

	handler := internalTokenAuth("token")(inner)

	r := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestInternalTokenAuth_NotBearerScheme(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	})

	handler := internalTokenAuth("token")(inner)

	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// ---------------------------------------------------------------------------
// SecurityHeaders
// ---------------------------------------------------------------------------

func TestSecurityHeaders_SetsAllHeaders(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := SecurityHeaders(inner)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	want := map[string]string{
		"X-Frame-Options":         "DENY",
		"X-Content-Type-Options":  "nosniff",
		"X-XSS-Protection":       "0",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Permissions-Policy":      "camera=(), microphone=(), geolocation=()",
		"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'",
	}

	for header, expected := range want {
		if got := w.Header().Get(header); got != expected {
			t.Errorf("%s = %q, want %q", header, got, expected)
		}
	}
}

func TestSecurityHeaders_HSTS_SetOverTLS(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := SecurityHeaders(inner)

	r := httptest.NewRequest("GET", "/", nil)
	r.TLS = &tls.ConnectionState{} // simulate TLS connection
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	got := w.Header().Get("Strict-Transport-Security")
	if got != "max-age=63072000; includeSubDomains" {
		t.Errorf("Strict-Transport-Security = %q, want HSTS header set for TLS request", got)
	}
}

func TestSecurityHeaders_HSTS_SetViaXForwardedProto(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := SecurityHeaders(inner)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	got := w.Header().Get("Strict-Transport-Security")
	if got != "max-age=63072000; includeSubDomains" {
		t.Errorf("Strict-Transport-Security = %q, want HSTS header set for X-Forwarded-Proto=https", got)
	}
}

func TestSecurityHeaders_HSTS_NotSetForPlainHTTP(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := SecurityHeaders(inner)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	got := w.Header().Get("Strict-Transport-Security")
	if got != "" {
		t.Errorf("Strict-Transport-Security = %q, want empty for plain HTTP request", got)
	}
}

func TestSecurityHeaders_CallsNext(t *testing.T) {
	t.Parallel()

	called := false
	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})

	h := SecurityHeaders(inner)
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if !called {
		t.Error("SecurityHeaders did not call next handler")
	}
}

// ---------------------------------------------------------------------------
// MaxBodySize
// ---------------------------------------------------------------------------

func TestMaxBodySize_LimitsBody(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	h := MaxBodySize(10)(inner)

	body := strings.NewReader("this body is definitely longer than ten bytes")
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d for oversized body", w.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestMaxBodySize_AllowsSmallBody(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	h := MaxBodySize(1024)(inner)

	body := strings.NewReader(`{"key":"value"}`)
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d for small body", w.Code, http.StatusOK)
	}
}

func TestMaxBodySize_ExcludesMultipartFormData(t *testing.T) {
	t.Parallel()

	var bodyRead []byte
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		bodyRead, err = io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	h := MaxBodySize(10)(inner)

	largeBody := strings.NewReader(strings.Repeat("x", 100))
	r := httptest.NewRequest("POST", "/", largeBody)
	r.Header.Set("Content-Type", "multipart/form-data; boundary=----")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d for multipart body (should bypass limit)", w.Code, http.StatusOK)
	}
	if len(bodyRead) != 100 {
		t.Errorf("read %d bytes, want 100 (multipart should not be limited)", len(bodyRead))
	}
}

func TestMaxBodySize_NoContentType(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	h := MaxBodySize(10)(inner)

	body := strings.NewReader("this body is way too long for the limit")
	r := httptest.NewRequest("POST", "/", body)
	// No Content-Type header set
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d for oversized body with no Content-Type", w.Code, http.StatusRequestEntityTooLarge)
	}
}

// ---------------------------------------------------------------------------
// RequestLogger
// ---------------------------------------------------------------------------

func TestRequestLogger_CallsNext(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	called := false
	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})

	h := RequestLogger(logger)(inner)

	r := httptest.NewRequest("GET", "/some/path", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if !called {
		t.Error("RequestLogger did not call next handler")
	}
}

func TestRequestLogger_SuppressesHealthAndMetricsPaths(t *testing.T) {
	t.Parallel()

	paths := []string{"/health", "/health/ready", "/metrics"}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			logger := slog.New(slog.NewTextHandler(&buf, nil))

			inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})

			h := RequestLogger(logger)(inner)

			r := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			if strings.Contains(buf.String(), "request completed") {
				t.Errorf("path %q should be suppressed in logs, but found log output", path)
			}
		})
	}
}

func TestRequestLogger_LogsNonHealthPaths(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})

	h := RequestLogger(logger)(inner)

	r := httptest.NewRequest("GET", "/api/documents", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if !strings.Contains(buf.String(), "request completed") {
		t.Errorf("expected log output for /api/documents, got: %s", buf.String())
	}
}

func TestRequestLogger_LogsMethodAndPath(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	h := RequestLogger(logger)(inner)

	r := httptest.NewRequest("POST", "/api/things", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	output := buf.String()
	if !strings.Contains(output, "POST") {
		t.Errorf("expected log to contain method POST, got: %s", output)
	}
	if !strings.Contains(output, "/api/things") {
		t.Errorf("expected log to contain path /api/things, got: %s", output)
	}
}

// ---------------------------------------------------------------------------
// extractIP edge cases
// ---------------------------------------------------------------------------

func TestExtractIP_RemoteAddrWithoutPort(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1" // no port -- triggers SplitHostPort error branch

	got := extractIP(r, nil)
	if got != "192.168.1.1" {
		t.Errorf("extractIP() = %q, want %q", got, "192.168.1.1")
	}
}

func TestExtractIP_RemoteAddrUnparseable(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "not-an-ip-at-all" // no port, unparseable IP

	got := extractIP(r, nil)
	if got != "not-an-ip-at-all" {
		t.Errorf("extractIP() = %q, want %q for unparseable RemoteAddr", got, "not-an-ip-at-all")
	}
}

func TestExtractIP_TrustedProxyUnparseableRemoteAddr(t *testing.T) {
	t.Parallel()

	trusted := []*net.IPNet{mustParseCIDR("10.0.0.0/8")}

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "not-an-ip" // unparseable, so trusted proxy check fails
	r.Header.Set("X-Real-Ip", "203.0.113.1")

	got := extractIP(r, trusted)
	// remoteIP will be nil, so headers should be ignored
	if got != "not-an-ip" {
		t.Errorf("extractIP() = %q, want %q", got, "not-an-ip")
	}
}

// ---------------------------------------------------------------------------
// internalTokenAuth additional cases
// ---------------------------------------------------------------------------

func TestInternalTokenAuth_EmptyBearerValue(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	})

	h := internalTokenAuth("token")(inner)

	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d for empty bearer value", w.Code, http.StatusUnauthorized)
	}
}

func TestInternalTokenAuth_VariousWrongTokens(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	})

	h := internalTokenAuth("correct-secret-token-value")(inner)

	tokens := []struct {
		name  string
		token string
	}{
		{"too short", "x"},
		{"one char short", "correct-secret-token-valu"},
		{"one char extra", "correct-secret-token-value-extra"},
	}

	for _, tt := range tokens {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest("GET", "/metrics", nil)
			r.Header.Set("Authorization", "Bearer "+tt.token)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("token %q: status = %d, want %d", tt.token, w.Code, http.StatusUnauthorized)
			}
		})
	}
}
