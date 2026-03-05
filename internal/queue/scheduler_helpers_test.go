package queue

import (
	"database/sql"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

func TestParseConfluenceCredentials(t *testing.T) {
	tests := []struct {
		name      string
		svc       model.ExternalService
		wantEmail string
		wantToken string
		wantErr   bool
	}{
		{
			name:      "valid email:token format",
			svc:       model.ExternalService{ID: 1, APIKey: sql.NullString{String: "user@example.com:api-token-123", Valid: true}},
			wantEmail: "user@example.com",
			wantToken: "api-token-123",
		},
		{
			name:    "no API key",
			svc:     model.ExternalService{ID: 1, APIKey: sql.NullString{Valid: false}},
			wantErr: true,
		},
		{
			name:    "empty API key",
			svc:     model.ExternalService{ID: 1, APIKey: sql.NullString{String: "", Valid: true}},
			wantErr: true,
		},
		{
			name:    "token without colon",
			svc:     model.ExternalService{ID: 1, APIKey: sql.NullString{String: "just-a-token", Valid: true}},
			wantErr: true,
		},
		{
			name:      "token with multiple colons",
			svc:       model.ExternalService{ID: 1, APIKey: sql.NullString{String: "user@example.com:token:with:colons", Valid: true}},
			wantEmail: "user@example.com",
			wantToken: "token:with:colons",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, token, err := parseConfluenceCredentials(tt.svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseConfluenceCredentials() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if email != tt.wantEmail {
					t.Errorf("email = %q, want %q", email, tt.wantEmail)
				}
				if token != tt.wantToken {
					t.Errorf("token = %q, want %q", token, tt.wantToken)
				}
			}
		})
	}
}

func TestToSyncTemplate(t *testing.T) {
	t.Run("converts full template", func(t *testing.T) {
		tmpl := model.GitTemplate{
			ID:            1,
			UUID:          "abc-123",
			Name:          "My Template",
			Slug:          "my-template",
			Description:   sql.NullString{String: "A description", Valid: true},
			RepositoryURL: "https://github.com/example/repo",
			Branch:        "main",
			GitToken:      sql.NullString{String: "ghp_token", Valid: true},
			Category:      sql.NullString{String: "starter", Valid: true},
			Tags:          sql.NullString{String: `["go","api"]`, Valid: true},
			LastCommitSHA: sql.NullString{String: "abc123", Valid: true},
		}

		result, err := toSyncTemplate(tmpl)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ID != 1 {
			t.Errorf("ID = %d, want 1", result.ID)
		}
		if result.UUID != "abc-123" {
			t.Errorf("UUID = %q, want abc-123", result.UUID)
		}
		if result.Token != "ghp_token" {
			t.Errorf("Token = %q, want ghp_token", result.Token)
		}
		if result.Category != "starter" {
			t.Errorf("Category = %q, want starter", result.Category)
		}
		if len(result.Tags) != 2 || result.Tags[0] != "go" || result.Tags[1] != "api" {
			t.Errorf("Tags = %v, want [go api]", result.Tags)
		}
		if result.LastCommitSHA != "abc123" {
			t.Errorf("LastCommitSHA = %q, want abc123", result.LastCommitSHA)
		}
	})

	t.Run("handles null optional fields", func(t *testing.T) {
		tmpl := model.GitTemplate{
			ID:            2,
			UUID:          "def-456",
			Name:          "Minimal",
			Slug:          "minimal",
			RepositoryURL: "https://github.com/example/minimal",
			Branch:        "main",
		}

		result, err := toSyncTemplate(tmpl)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Description != "" {
			t.Errorf("Description = %q, want empty", result.Description)
		}
		if result.Token != "" {
			t.Errorf("Token = %q, want empty", result.Token)
		}
		if len(result.Tags) != 0 {
			t.Errorf("Tags = %v, want empty", result.Tags)
		}
	})

	t.Run("returns error for invalid tags JSON", func(t *testing.T) {
		tmpl := model.GitTemplate{
			ID:            3,
			UUID:          "ghi-789",
			Name:          "Bad Tags",
			Slug:          "bad-tags",
			RepositoryURL: "https://github.com/example/bad",
			Branch:        "main",
			Tags:          sql.NullString{String: "not-json", Valid: true},
		}

		_, err := toSyncTemplate(tmpl)
		if err == nil {
			t.Fatal("expected error for invalid tags JSON")
		}
	})
}
