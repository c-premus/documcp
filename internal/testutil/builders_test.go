package testutil

import (
	"testing"

	"github.com/c-premus/documcp/internal/model"
)

// ---------------------------------------------------------------------------
// Document
// ---------------------------------------------------------------------------

func TestNewDocument(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		d := NewDocument()
		if d == nil {
			t.Fatal("expected non-nil Document")
		}
		if d.ID != 1 {
			t.Errorf("ID = %d, want 1", d.ID)
		}
		if d.UUID != "test-doc-uuid" {
			t.Errorf("UUID = %q, want %q", d.UUID, "test-doc-uuid")
		}
		if d.Title != "Test Document" {
			t.Errorf("Title = %q, want %q", d.Title, "Test Document")
		}
		if d.FileType != "pdf" {
			t.Errorf("FileType = %q, want %q", d.FileType, "pdf")
		}
		if d.FilePath != "/tmp/test-document.pdf" {
			t.Errorf("FilePath = %q, want %q", d.FilePath, "/tmp/test-document.pdf")
		}
		if d.FileSize != 1024 {
			t.Errorf("FileSize = %d, want 1024", d.FileSize)
		}
		if d.MIMEType != "application/pdf" {
			t.Errorf("MIMEType = %q, want %q", d.MIMEType, "application/pdf")
		}
		if d.IsPublic {
			t.Error("IsPublic = true, want false")
		}
		if d.Status != model.DocumentStatusIndexed {
			t.Errorf("Status = %q, want %q", d.Status, model.DocumentStatusIndexed)
		}
		if !d.CreatedAt.Valid {
			t.Error("CreatedAt should be valid")
		}
		if !d.UpdatedAt.Valid {
			t.Error("UpdatedAt should be valid")
		}
	})

	t.Run("with all options", func(t *testing.T) {
		t.Parallel()
		d := NewDocument(
			WithDocumentID(42),
			WithDocumentUUID("custom-uuid"),
			WithDocumentTitle("Custom Title"),
			WithDocumentDescription("A description"),
			WithDocumentFileType("markdown"),
			WithDocumentFilePath("/data/file.md"),
			WithDocumentFileSize(2048),
			WithDocumentMIMEType("text/markdown"),
			WithDocumentContent("Hello world"),
			WithDocumentUserID(7),
			WithDocumentIsPublic(true),
			WithDocumentStatus(model.DocumentStatusPending),
		)
		if d.ID != 42 {
			t.Errorf("ID = %d, want 42", d.ID)
		}
		if d.UUID != "custom-uuid" {
			t.Errorf("UUID = %q, want %q", d.UUID, "custom-uuid")
		}
		if d.Title != "Custom Title" {
			t.Errorf("Title = %q, want %q", d.Title, "Custom Title")
		}
		if !d.Description.Valid || d.Description.String != "A description" {
			t.Errorf("Description = %v, want valid %q", d.Description, "A description")
		}
		if d.FileType != "markdown" {
			t.Errorf("FileType = %q, want %q", d.FileType, "markdown")
		}
		if d.FilePath != "/data/file.md" {
			t.Errorf("FilePath = %q, want %q", d.FilePath, "/data/file.md")
		}
		if d.FileSize != 2048 {
			t.Errorf("FileSize = %d, want 2048", d.FileSize)
		}
		if d.MIMEType != "text/markdown" {
			t.Errorf("MIMEType = %q, want %q", d.MIMEType, "text/markdown")
		}
		if !d.Content.Valid || d.Content.String != "Hello world" {
			t.Errorf("Content = %v, want valid %q", d.Content, "Hello world")
		}
		if !d.UserID.Valid || d.UserID.Int64 != 7 {
			t.Errorf("UserID = %v, want valid 7", d.UserID)
		}
		if !d.IsPublic {
			t.Error("IsPublic = false, want true")
		}
		if d.Status != model.DocumentStatusPending {
			t.Errorf("Status = %q, want %q", d.Status, model.DocumentStatusPending)
		}
	})
}

// ---------------------------------------------------------------------------
// User
// ---------------------------------------------------------------------------

func TestNewUser(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		u := NewUser()
		if u == nil {
			t.Fatal("expected non-nil User")
		}
		if u.ID != 1 {
			t.Errorf("ID = %d, want 1", u.ID)
		}
		if u.Name != "Test User" {
			t.Errorf("Name = %q, want %q", u.Name, "Test User")
		}
		if u.Email != "test@example.com" {
			t.Errorf("Email = %q, want %q", u.Email, "test@example.com")
		}
		if u.IsAdmin {
			t.Error("IsAdmin = true, want false")
		}
		if !u.CreatedAt.Valid {
			t.Error("CreatedAt should be valid")
		}
		if !u.UpdatedAt.Valid {
			t.Error("UpdatedAt should be valid")
		}
	})

	t.Run("with all options", func(t *testing.T) {
		t.Parallel()
		u := NewUser(
			WithUserID(99),
			WithUserName("Alice"),
			WithUserEmail("alice@example.com"),
			WithUserIsAdmin(true),
			WithUserOIDCSub("oidc-sub-123"),
			WithUserOIDCProvider("google"),
			WithUserPassword("hashed-pw"),
		)
		if u.ID != 99 {
			t.Errorf("ID = %d, want 99", u.ID)
		}
		if u.Name != "Alice" {
			t.Errorf("Name = %q, want %q", u.Name, "Alice")
		}
		if u.Email != "alice@example.com" {
			t.Errorf("Email = %q, want %q", u.Email, "alice@example.com")
		}
		if !u.IsAdmin {
			t.Error("IsAdmin = false, want true")
		}
		if !u.OIDCSub.Valid || u.OIDCSub.String != "oidc-sub-123" {
			t.Errorf("OIDCSub = %v, want valid %q", u.OIDCSub, "oidc-sub-123")
		}
		if !u.OIDCProvider.Valid || u.OIDCProvider.String != "google" {
			t.Errorf("OIDCProvider = %v, want valid %q", u.OIDCProvider, "google")
		}
		if !u.Password.Valid || u.Password.String != "hashed-pw" {
			t.Errorf("Password = %v, want valid %q", u.Password, "hashed-pw")
		}
	})
}

// ---------------------------------------------------------------------------
// OAuthClient
// ---------------------------------------------------------------------------

func TestNewOAuthClient(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		c := NewOAuthClient()
		if c == nil {
			t.Fatal("expected non-nil OAuthClient")
		}
		if c.ID != 1 {
			t.Errorf("ID = %d, want 1", c.ID)
		}
		if c.ClientID != "test-client-id" {
			t.Errorf("ClientID = %q, want %q", c.ClientID, "test-client-id")
		}
		if c.ClientName != "Test Client" {
			t.Errorf("ClientName = %q, want %q", c.ClientName, "Test Client")
		}
		if c.RedirectURIs != `["http://localhost:8080/callback"]` {
			t.Errorf("RedirectURIs = %q, want %q", c.RedirectURIs, `["http://localhost:8080/callback"]`)
		}
		if c.GrantTypes != `["authorization_code"]` {
			t.Errorf("GrantTypes = %q, want %q", c.GrantTypes, `["authorization_code"]`)
		}
		if c.ResponseTypes != `["code"]` {
			t.Errorf("ResponseTypes = %q, want %q", c.ResponseTypes, `["code"]`)
		}
		if c.TokenEndpointAuthMethod != "client_secret_basic" {
			t.Errorf("TokenEndpointAuthMethod = %q, want %q", c.TokenEndpointAuthMethod, "client_secret_basic")
		}
		if !c.CreatedAt.Valid {
			t.Error("CreatedAt should be valid")
		}
		if !c.UpdatedAt.Valid {
			t.Error("UpdatedAt should be valid")
		}
	})

	t.Run("with all options", func(t *testing.T) {
		t.Parallel()
		c := NewOAuthClient(
			WithOAuthClientID(10),
			WithOAuthClientClientID("my-client"),
			WithOAuthClientName("My App"),
			WithOAuthClientSecret("s3cret"),
			WithOAuthClientRedirectURIs(`["https://app.example.com/cb"]`),
			WithOAuthClientGrantTypes(`["client_credentials"]`),
			WithOAuthClientScope("read write"),
			WithOAuthClientUserID(5),
		)
		if c.ID != 10 {
			t.Errorf("ID = %d, want 10", c.ID)
		}
		if c.ClientID != "my-client" {
			t.Errorf("ClientID = %q, want %q", c.ClientID, "my-client")
		}
		if c.ClientName != "My App" {
			t.Errorf("ClientName = %q, want %q", c.ClientName, "My App")
		}
		if !c.ClientSecret.Valid || c.ClientSecret.String != "s3cret" {
			t.Errorf("ClientSecret = %v, want valid %q", c.ClientSecret, "s3cret")
		}
		if c.RedirectURIs != `["https://app.example.com/cb"]` {
			t.Errorf("RedirectURIs = %q", c.RedirectURIs)
		}
		if c.GrantTypes != `["client_credentials"]` {
			t.Errorf("GrantTypes = %q", c.GrantTypes)
		}
		if !c.Scope.Valid || c.Scope.String != "read write" {
			t.Errorf("Scope = %v, want valid %q", c.Scope, "read write")
		}
		if !c.UserID.Valid || c.UserID.Int64 != 5 {
			t.Errorf("UserID = %v, want valid 5", c.UserID)
		}
	})
}

// ---------------------------------------------------------------------------
// ExternalService
// ---------------------------------------------------------------------------

func TestNewExternalService(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		es := NewExternalService()
		if es == nil {
			t.Fatal("expected non-nil ExternalService")
		}
		if es.ID != 1 {
			t.Errorf("ID = %d, want 1", es.ID)
		}
		if es.UUID != "test-extservice-uuid" {
			t.Errorf("UUID = %q, want %q", es.UUID, "test-extservice-uuid")
		}
		if es.Name != "Test Service" {
			t.Errorf("Name = %q, want %q", es.Name, "Test Service")
		}
		if es.Slug != "test-service" {
			t.Errorf("Slug = %q, want %q", es.Slug, "test-service")
		}
		if es.Type != "kiwix" {
			t.Errorf("Type = %q, want %q", es.Type, "kiwix")
		}
		if es.BaseURL != "https://example.com" {
			t.Errorf("BaseURL = %q, want %q", es.BaseURL, "https://example.com")
		}
		if es.Priority != 100 {
			t.Errorf("Priority = %d, want 100", es.Priority)
		}
		if es.Status != model.ExternalServiceStatusUnknown {
			t.Errorf("Status = %q, want %q", es.Status, model.ExternalServiceStatusUnknown)
		}
		if !es.IsEnabled {
			t.Error("IsEnabled = false, want true")
		}
		if !es.CreatedAt.Valid {
			t.Error("CreatedAt should be valid")
		}
		if !es.UpdatedAt.Valid {
			t.Error("UpdatedAt should be valid")
		}
	})

	t.Run("with all options", func(t *testing.T) {
		t.Parallel()
		es := NewExternalService(
			WithExternalServiceID(50),
			WithExternalServiceUUID("custom-es-uuid"),
			WithExternalServiceName("Wikipedia"),
			WithExternalServiceSlug("wikipedia"),
			WithExternalServiceType("wiki"),
			WithExternalServiceBaseURL("https://wikipedia.org"),
			WithExternalServiceStatus(model.ExternalServiceStatusUnhealthy),
			WithExternalServiceIsEnabled(false),
			WithExternalServicePriority(200),
		)
		if es.ID != 50 {
			t.Errorf("ID = %d, want 50", es.ID)
		}
		if es.UUID != "custom-es-uuid" {
			t.Errorf("UUID = %q, want %q", es.UUID, "custom-es-uuid")
		}
		if es.Name != "Wikipedia" {
			t.Errorf("Name = %q, want %q", es.Name, "Wikipedia")
		}
		if es.Slug != "wikipedia" {
			t.Errorf("Slug = %q, want %q", es.Slug, "wikipedia")
		}
		if es.Type != "wiki" {
			t.Errorf("Type = %q, want %q", es.Type, "wiki")
		}
		if es.BaseURL != "https://wikipedia.org" {
			t.Errorf("BaseURL = %q, want %q", es.BaseURL, "https://wikipedia.org")
		}
		if es.Status != model.ExternalServiceStatusUnhealthy {
			t.Errorf("Status = %q, want %q", es.Status, model.ExternalServiceStatusUnhealthy)
		}
		if es.IsEnabled {
			t.Error("IsEnabled = true, want false")
		}
		if es.Priority != 200 {
			t.Errorf("Priority = %d, want 200", es.Priority)
		}
	})
}

// ---------------------------------------------------------------------------
// ZimArchive
// ---------------------------------------------------------------------------

func TestNewZimArchive(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		za := NewZimArchive()
		if za == nil {
			t.Fatal("expected non-nil ZimArchive")
		}
		if za.ID != 1 {
			t.Errorf("ID = %d, want 1", za.ID)
		}
		if za.UUID != "test-zim-uuid" {
			t.Errorf("UUID = %q, want %q", za.UUID, "test-zim-uuid")
		}
		if za.Name != "Test ZIM Archive" {
			t.Errorf("Name = %q, want %q", za.Name, "Test ZIM Archive")
		}
		if za.Slug != "test-zim-archive" {
			t.Errorf("Slug = %q, want %q", za.Slug, "test-zim-archive")
		}
		if za.Title != "Test ZIM" {
			t.Errorf("Title = %q, want %q", za.Title, "Test ZIM")
		}
		if za.Language != "en" {
			t.Errorf("Language = %q, want %q", za.Language, "en")
		}
		if za.ArticleCount != 100 {
			t.Errorf("ArticleCount = %d, want 100", za.ArticleCount)
		}
		if za.MediaCount != 10 {
			t.Errorf("MediaCount = %d, want 10", za.MediaCount)
		}
		if za.FileSize != 1048576 {
			t.Errorf("FileSize = %d, want 1048576", za.FileSize)
		}
		if !za.IsEnabled {
			t.Error("IsEnabled = false, want true")
		}
		if !za.IsSearchable {
			t.Error("IsSearchable = false, want true")
		}
		if !za.CreatedAt.Valid {
			t.Error("CreatedAt should be valid")
		}
		if !za.UpdatedAt.Valid {
			t.Error("UpdatedAt should be valid")
		}
	})

	t.Run("with all options", func(t *testing.T) {
		t.Parallel()
		za := NewZimArchive(
			WithZimArchiveID(77),
			WithZimArchiveUUID("zim-uuid-77"),
			WithZimArchiveName("Wikipedia EN"),
			WithZimArchiveSlug("wikipedia-en"),
			WithZimArchiveTitle("Wikipedia English"),
			WithZimArchiveLanguage("fr"),
			WithZimArchiveExternalServiceID(3),
			WithZimArchiveIsEnabled(false),
			WithZimArchiveIsSearchable(false),
		)
		if za.ID != 77 {
			t.Errorf("ID = %d, want 77", za.ID)
		}
		if za.UUID != "zim-uuid-77" {
			t.Errorf("UUID = %q, want %q", za.UUID, "zim-uuid-77")
		}
		if za.Name != "Wikipedia EN" {
			t.Errorf("Name = %q, want %q", za.Name, "Wikipedia EN")
		}
		if za.Slug != "wikipedia-en" {
			t.Errorf("Slug = %q, want %q", za.Slug, "wikipedia-en")
		}
		if za.Title != "Wikipedia English" {
			t.Errorf("Title = %q, want %q", za.Title, "Wikipedia English")
		}
		if za.Language != "fr" {
			t.Errorf("Language = %q, want %q", za.Language, "fr")
		}
		if !za.ExternalServiceID.Valid || za.ExternalServiceID.Int64 != 3 {
			t.Errorf("ExternalServiceID = %v, want valid 3", za.ExternalServiceID)
		}
		if za.IsEnabled {
			t.Error("IsEnabled = true, want false")
		}
		if za.IsSearchable {
			t.Error("IsSearchable = true, want false")
		}
	})
}

// ---------------------------------------------------------------------------
// GitTemplate
// ---------------------------------------------------------------------------

func TestNewGitTemplate(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		gt := NewGitTemplate()
		if gt == nil {
			t.Fatal("expected non-nil GitTemplate")
		}
		if gt.ID != 1 {
			t.Errorf("ID = %d, want 1", gt.ID)
		}
		if gt.UUID != "test-template-uuid" {
			t.Errorf("UUID = %q, want %q", gt.UUID, "test-template-uuid")
		}
		if gt.Name != "Test Template" {
			t.Errorf("Name = %q, want %q", gt.Name, "Test Template")
		}
		if gt.Slug != "test-template" {
			t.Errorf("Slug = %q, want %q", gt.Slug, "test-template")
		}
		if gt.RepositoryURL != "https://github.com/example/repo.git" {
			t.Errorf("RepositoryURL = %q, want %q", gt.RepositoryURL, "https://github.com/example/repo.git")
		}
		if gt.Branch != "main" {
			t.Errorf("Branch = %q, want %q", gt.Branch, "main")
		}
		if !gt.IsPublic {
			t.Error("IsPublic = false, want true")
		}
		if !gt.IsEnabled {
			t.Error("IsEnabled = false, want true")
		}
		if gt.Status != model.GitTemplateStatusSynced {
			t.Errorf("Status = %q, want %q", gt.Status, model.GitTemplateStatusSynced)
		}
		if gt.FileCount != 5 {
			t.Errorf("FileCount = %d, want 5", gt.FileCount)
		}
		if !gt.CreatedAt.Valid {
			t.Error("CreatedAt should be valid")
		}
		if !gt.UpdatedAt.Valid {
			t.Error("UpdatedAt should be valid")
		}
	})

	t.Run("with all options", func(t *testing.T) {
		t.Parallel()
		gt := NewGitTemplate(
			WithGitTemplateID(33),
			WithGitTemplateUUID("tmpl-uuid-33"),
			WithGitTemplateName("My Template"),
			WithGitTemplateSlug("my-template"),
			WithGitTemplateDescription("A template description"),
			WithGitTemplateRepositoryURL("https://gitlab.com/org/repo.git"),
			WithGitTemplateBranch("develop"),
			WithGitTemplateUserID(8),
			WithGitTemplateIsPublic(false),
			WithGitTemplateIsEnabled(false),
			WithGitTemplateStatus(model.GitTemplateStatus("error")),
		)
		if gt.ID != 33 {
			t.Errorf("ID = %d, want 33", gt.ID)
		}
		if gt.UUID != "tmpl-uuid-33" {
			t.Errorf("UUID = %q, want %q", gt.UUID, "tmpl-uuid-33")
		}
		if gt.Name != "My Template" {
			t.Errorf("Name = %q, want %q", gt.Name, "My Template")
		}
		if gt.Slug != "my-template" {
			t.Errorf("Slug = %q, want %q", gt.Slug, "my-template")
		}
		if !gt.Description.Valid || gt.Description.String != "A template description" {
			t.Errorf("Description = %v, want valid %q", gt.Description, "A template description")
		}
		if gt.RepositoryURL != "https://gitlab.com/org/repo.git" {
			t.Errorf("RepositoryURL = %q, want %q", gt.RepositoryURL, "https://gitlab.com/org/repo.git")
		}
		if gt.Branch != "develop" {
			t.Errorf("Branch = %q, want %q", gt.Branch, "develop")
		}
		if !gt.UserID.Valid || gt.UserID.Int64 != 8 {
			t.Errorf("UserID = %v, want valid 8", gt.UserID)
		}
		if gt.IsPublic {
			t.Error("IsPublic = true, want false")
		}
		if gt.IsEnabled {
			t.Error("IsEnabled = true, want false")
		}
		if gt.Status != "error" {
			t.Errorf("Status = %q, want %q", gt.Status, "error")
		}
	})
}

// ---------------------------------------------------------------------------
// SearchQuery
// ---------------------------------------------------------------------------

func TestNewSearchQuery(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		sq := NewSearchQuery()
		if sq == nil {
			t.Fatal("expected non-nil SearchQuery")
		}
		if sq.Query != "test search" {
			t.Errorf("Query = %q, want %q", sq.Query, "test search")
		}
		if sq.ResultsCount != 10 {
			t.Errorf("ResultsCount = %d, want 10", sq.ResultsCount)
		}
		// SearchQuery defaults do not set timestamps or UserID.
		if sq.UserID.Valid {
			t.Error("UserID should not be valid by default")
		}
		if sq.Filters.Valid {
			t.Error("Filters should not be valid by default")
		}
	})

	t.Run("with all options", func(t *testing.T) {
		t.Parallel()
		sq := NewSearchQuery(
			WithSearchQueryUserID(12),
			WithSearchQueryQuery("golang testing"),
			WithSearchQueryResultsCount(25),
			WithSearchQueryFilters(`{"type":"pdf"}`),
		)
		if sq.Query != "golang testing" {
			t.Errorf("Query = %q, want %q", sq.Query, "golang testing")
		}
		if sq.ResultsCount != 25 {
			t.Errorf("ResultsCount = %d, want 25", sq.ResultsCount)
		}
		if !sq.UserID.Valid || sq.UserID.Int64 != 12 {
			t.Errorf("UserID = %v, want valid 12", sq.UserID)
		}
		if !sq.Filters.Valid || sq.Filters.String != `{"type":"pdf"}` {
			t.Errorf("Filters = %v, want valid %q", sq.Filters, `{"type":"pdf"}`)
		}
	})
}

// ---------------------------------------------------------------------------
// Isolation: each call returns an independent instance.
// ---------------------------------------------------------------------------

func TestBuildersReturnIndependentInstances(t *testing.T) {
	t.Parallel()

	d1 := NewDocument(WithDocumentTitle("A"))
	d2 := NewDocument(WithDocumentTitle("B"))
	if d1.Title == d2.Title {
		t.Errorf("expected independent instances, both have Title = %q", d1.Title)
	}
	// Mutating one should not affect the other.
	d1.Status = model.DocumentStatus("deleted")
	if d2.Status == model.DocumentStatus("deleted") {
		t.Error("mutating d1 affected d2")
	}
}
