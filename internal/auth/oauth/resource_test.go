package oauth

import (
	"errors"
	"testing"
)

func TestValidateResource(t *testing.T) {
	allowed := []string{
		"https://documcp.example.com",
		"https://documcp.example.com/documcp",
		"http://127.0.0.1:8080/documcp",
		"http://localhost:8080",
	}

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "https exact match", raw: "https://documcp.example.com", want: "https://documcp.example.com"},
		{name: "https with path match", raw: "https://documcp.example.com/documcp", want: "https://documcp.example.com/documcp"},
		{name: "loopback 127.0.0.1", raw: "http://127.0.0.1:8080/documcp", want: "http://127.0.0.1:8080/documcp"},
		{name: "loopback localhost", raw: "http://localhost:8080", want: "http://localhost:8080"},

		{name: "empty", raw: "", wantErr: true},
		{name: "relative URI", raw: "/documcp", wantErr: true},
		{name: "missing scheme", raw: "documcp.example.com", wantErr: true},
		{name: "trailing slash mismatch", raw: "https://documcp.example.com/", wantErr: true},
		{name: "fragment", raw: "https://documcp.example.com#frag", wantErr: true},
		{name: "http on non-loopback", raw: "http://documcp.example.com", wantErr: true},
		{name: "ftp scheme", raw: "ftp://documcp.example.com", wantErr: true},
		{name: "not in allowlist", raw: "https://other.example.com", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateResource(tc.raw, allowed)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (result %q)", got)
				}
				if !errors.Is(err, ErrInvalidResource) {
					t.Fatalf("error is not ErrInvalidResource: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
