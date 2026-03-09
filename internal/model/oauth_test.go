package model

import (
	"testing"
)

func TestOAuthClient_ParseRedirectURIs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "single URI",
			input: `["https://example.com/callback"]`,
			want:  []string{"https://example.com/callback"},
		},
		{
			name:  "multiple URIs",
			input: `["https://a.com/cb","https://b.com/cb"]`,
			want:  []string{"https://a.com/cb", "https://b.com/cb"},
		},
		{
			name:  "empty array",
			input: `[]`,
			want:  []string{},
		},
		{
			name:    "invalid JSON",
			input:   `not-json`,
			wantErr: true,
		},
		{
			name:    "wrong type",
			input:   `{"key": "value"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &OAuthClient{RedirectURIs: tt.input}
			got, err := c.ParseRedirectURIs()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d URIs, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("URI[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestOAuthClient_ParseGrantTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "authorization_code",
			input: `["authorization_code"]`,
			want:  []string{"authorization_code"},
		},
		{
			name:  "multiple",
			input: `["authorization_code","refresh_token","client_credentials"]`,
			want:  []string{"authorization_code", "refresh_token", "client_credentials"},
		},
		{
			name:    "invalid JSON",
			input:   `{bad`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &OAuthClient{GrantTypes: tt.input}
			got, err := c.ParseGrantTypes()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d types, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("type[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestOAuthClient_ParseResponseTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "code",
			input: `["code"]`,
			want:  []string{"code"},
		},
		{
			name:  "multiple",
			input: `["code","token"]`,
			want:  []string{"code", "token"},
		},
		{
			name:    "invalid JSON",
			input:   `not-json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &OAuthClient{ResponseTypes: tt.input}
			got, err := c.ParseResponseTypes()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d types, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("type[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
