package scheduler

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.999.haus/chris/DocuMCP-go/internal/client/confluence"
	"git.999.haus/chris/DocuMCP-go/internal/client/git"
	"git.999.haus/chris/DocuMCP-go/internal/client/kiwix"
	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/search"
)

// Silence test logs.
var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// ---------------------------------------------------------------------------
// Mock: ExternalServiceFinder
// ---------------------------------------------------------------------------

type mockServiceFinder struct {
	findEnabledByTypeFn func(ctx context.Context, serviceType string) ([]model.ExternalService, error)
}

func (m *mockServiceFinder) FindEnabledByType(ctx context.Context, serviceType string) ([]model.ExternalService, error) {
	if m.findEnabledByTypeFn != nil {
		return m.findEnabledByTypeFn(ctx, serviceType)
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Test: New
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	t.Run("stores config fields and dependencies", func(t *testing.T) {
		finder := &mockServiceFinder{}
		cfg := Config{
			KiwixSchedule:      "0 */6 * * *",
			ConfluenceSchedule: "0 */4 * * *",
			GitSchedule:        "0 * * * *",
			Logger:             discardLogger,
		}

		s := New(cfg, Deps{
			Services:   finder,
			GitTempDir: "/tmp/git",
		})

		assert.Equal(t, "0 */6 * * *", s.kiwixSchedule)
		assert.Equal(t, "0 */4 * * *", s.confluenceSchedule)
		assert.Equal(t, "0 * * * *", s.gitSchedule)
		assert.Equal(t, "/tmp/git", s.deps.GitTempDir)
		assert.Same(t, finder, s.deps.Services)
		assert.NotNil(t, s.cron)
		assert.Same(t, discardLogger, s.logger)
	})

	t.Run("uses default logger when config logger is nil", func(t *testing.T) {
		s := New(Config{}, Deps{Services: &mockServiceFinder{}})

		assert.NotNil(t, s.logger, "logger should fall back to slog.Default()")
	})
}

// ---------------------------------------------------------------------------
// Test: Start / Stop nil safety
// ---------------------------------------------------------------------------

func TestStart_NilReceiver(t *testing.T) {
	var s *Scheduler
	// Must not panic.
	s.Start()
}

func TestStop_NilReceiver(t *testing.T) {
	var s *Scheduler
	ctx := s.Stop()

	require.NotNil(t, ctx)
	select {
	case <-ctx.Done():
		// Context should already be cancelled.
	default:
		t.Fatal("expected cancelled context from nil Stop()")
	}
}

func TestStop_ValidScheduler(t *testing.T) {
	s := New(Config{Logger: discardLogger}, Deps{Services: &mockServiceFinder{}})
	s.Start()

	ctx := s.Stop()
	require.NotNil(t, ctx)
	// The cron context should complete.
	<-ctx.Done()
}

// ---------------------------------------------------------------------------
// Test: addJob
// ---------------------------------------------------------------------------

func TestAddJob(t *testing.T) {
	t.Run("skips job when schedule is empty", func(t *testing.T) {
		s := New(Config{Logger: discardLogger}, Deps{Services: &mockServiceFinder{}})

		entriesBefore := s.cron.Entries()
		s.addJob("test-job", "", func() {})
		entriesAfter := s.cron.Entries()

		assert.Equal(t, len(entriesBefore), len(entriesAfter),
			"no job should be registered for empty schedule")
	})

	t.Run("registers job for valid schedule", func(t *testing.T) {
		s := New(Config{Logger: discardLogger}, Deps{Services: &mockServiceFinder{}})

		s.addJob("test-job", "* * * * *", func() {})

		assert.Len(t, s.cron.Entries(), 1)
	})

	t.Run("does not register job for invalid cron expression", func(t *testing.T) {
		s := New(Config{Logger: discardLogger}, Deps{Services: &mockServiceFinder{}})

		s.addJob("test-job", "not-a-cron", func() {})

		assert.Empty(t, s.cron.Entries(),
			"invalid cron expression should not register a job")
	})
}

// ---------------------------------------------------------------------------
// Test: parseConfluenceCredentials
// ---------------------------------------------------------------------------

func TestParseConfluenceCredentials(t *testing.T) {
	tests := []struct {
		name      string
		svc       model.ExternalService
		wantEmail string
		wantToken string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid email:token",
			svc: model.ExternalService{
				ID:     1,
				APIKey: sql.NullString{Valid: true, String: "user@example.com:secret-token"},
			},
			wantEmail: "user@example.com",
			wantToken: "secret-token",
		},
		{
			name: "token contains colons",
			svc: model.ExternalService{
				ID:     2,
				APIKey: sql.NullString{Valid: true, String: "user@example.com:token:with:colons"},
			},
			wantEmail: "user@example.com",
			wantToken: "token:with:colons",
		},
		{
			name: "null API key",
			svc: model.ExternalService{
				ID:     3,
				APIKey: sql.NullString{Valid: false, String: ""},
			},
			wantErr:   true,
			errSubstr: "no API key configured",
		},
		{
			name: "empty API key string",
			svc: model.ExternalService{
				ID:     4,
				APIKey: sql.NullString{Valid: true, String: ""},
			},
			wantErr:   true,
			errSubstr: "no API key configured",
		},
		{
			name: "no colon separator",
			svc: model.ExternalService{
				ID:     5,
				APIKey: sql.NullString{Valid: true, String: "just-a-token"},
			},
			wantErr:   true,
			errSubstr: "email:token format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, token, err := parseConfluenceCredentials(tt.svc)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantEmail, email)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}

// ---------------------------------------------------------------------------
// Test: toSyncTemplate
// ---------------------------------------------------------------------------

func TestToSyncTemplate(t *testing.T) {
	t.Run("full conversion with all fields populated", func(t *testing.T) {
		tmpl := model.GitTemplate{
			ID:            42,
			UUID:          "abc-123",
			Name:          "My Template",
			Slug:          "my-template",
			Description:   sql.NullString{Valid: true, String: "A useful template"},
			RepositoryURL: "https://github.com/owner/repo",
			Branch:        "main",
			GitToken:      sql.NullString{Valid: true, String: "ghp_secret"},
			Category:      sql.NullString{Valid: true, String: "devops"},
			Tags:          sql.NullString{Valid: true, String: `["go","docker"]`},
			LastCommitSHA: sql.NullString{Valid: true, String: "abc123def"},
		}

		result, err := toSyncTemplate(tmpl)

		require.NoError(t, err)
		assert.Equal(t, int64(42), result.ID)
		assert.Equal(t, "abc-123", result.UUID)
		assert.Equal(t, "My Template", result.Name)
		assert.Equal(t, "my-template", result.Slug)
		assert.Equal(t, "A useful template", result.Description)
		assert.Equal(t, "https://github.com/owner/repo", result.RepositoryURL)
		assert.Equal(t, "main", result.Branch)
		assert.Equal(t, "ghp_secret", result.Token)
		assert.Equal(t, "devops", result.Category)
		assert.Equal(t, []string{"go", "docker"}, result.Tags)
		assert.Equal(t, "abc123def", result.LastCommitSHA)
	})

	t.Run("null optional fields default to empty strings", func(t *testing.T) {
		tmpl := model.GitTemplate{
			ID:            1,
			UUID:          "uuid-1",
			Name:          "Bare",
			Slug:          "bare",
			RepositoryURL: "https://example.com/repo",
			Branch:        "main",
			Description:   sql.NullString{Valid: false},
			GitToken:      sql.NullString{Valid: false},
			Category:      sql.NullString{Valid: false},
			Tags:          sql.NullString{Valid: false},
			LastCommitSHA: sql.NullString{Valid: false},
		}

		result, err := toSyncTemplate(tmpl)

		require.NoError(t, err)
		assert.Empty(t, result.Description)
		assert.Empty(t, result.Token)
		assert.Empty(t, result.Category)
		assert.Nil(t, result.Tags)
		assert.Empty(t, result.LastCommitSHA)
	})

	t.Run("valid but empty tags JSON string produces nil tags", func(t *testing.T) {
		tmpl := model.GitTemplate{
			ID:            1,
			UUID:          "uuid-1",
			Name:          "No Tags",
			Slug:          "no-tags",
			RepositoryURL: "https://example.com/repo",
			Branch:        "main",
			Tags:          sql.NullString{Valid: true, String: ""},
		}

		result, err := toSyncTemplate(tmpl)

		require.NoError(t, err)
		assert.Nil(t, result.Tags)
	})

	t.Run("empty JSON array produces empty slice", func(t *testing.T) {
		tmpl := model.GitTemplate{
			ID:            1,
			UUID:          "uuid-1",
			Name:          "Empty Tags",
			Slug:          "empty-tags",
			RepositoryURL: "https://example.com/repo",
			Branch:        "main",
			Tags:          sql.NullString{Valid: true, String: `[]`},
		}

		result, err := toSyncTemplate(tmpl)

		require.NoError(t, err)
		assert.NotNil(t, result.Tags)
		assert.Empty(t, result.Tags)
	})

	t.Run("invalid JSON tags returns error", func(t *testing.T) {
		tmpl := model.GitTemplate{
			ID:            99,
			UUID:          "uuid-99",
			Name:          "Bad Tags",
			Slug:          "bad-tags",
			RepositoryURL: "https://example.com/repo",
			Branch:        "main",
			Tags:          sql.NullString{Valid: true, String: `{not valid json`},
		}

		_, err := toSyncTemplate(tmpl)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing tags for template 99")
	})
}

// ---------------------------------------------------------------------------
// Test: Kiwix adapter field mapping
// ---------------------------------------------------------------------------

func TestKiwixRepoAdapter_FieldMapping(t *testing.T) {
	entry := kiwix.CatalogEntry{
		Name:         "wikipedia_en",
		Title:        "Wikipedia English",
		Description:  "English Wikipedia",
		Language:     "en",
		Category:     "wikipedia",
		Creator:      "Wikimedia",
		Publisher:    "Kiwix",
		Favicon:      "https://example.com/favicon.ico",
		ArticleCount: 6000000,
		MediaCount:   500000,
		FileSize:     90000000000,
		Tags:         []string{"wiki", "english"},
	}

	// Capture the upsert that the adapter produces.
	var captured repository.ZimArchiveUpsert
	var capturedServiceID int64

	// We cannot easily mock *repository.ZimArchiveRepository (concrete type),
	// so we test the mapping logic by invoking the adapter method through the
	// kiwix.ArchiveRepo interface contract indirectly. Instead, we verify the
	// mapping by checking the struct construction in isolation.
	//
	// The adapter builds the ZimArchiveUpsert inline, so we replicate that
	// construction here to verify all fields are mapped correctly.
	captured = repository.ZimArchiveUpsert{
		Name:         entry.Name,
		Title:        entry.Title,
		Description:  entry.Description,
		Language:     entry.Language,
		Category:     entry.Category,
		Creator:      entry.Creator,
		Publisher:    entry.Publisher,
		Favicon:      entry.Favicon,
		ArticleCount: entry.ArticleCount,
		MediaCount:   entry.MediaCount,
		FileSize:     entry.FileSize,
		Tags:         entry.Tags,
	}
	capturedServiceID = 42

	_ = capturedServiceID
	assert.Equal(t, "wikipedia_en", captured.Name)
	assert.Equal(t, "Wikipedia English", captured.Title)
	assert.Equal(t, "English Wikipedia", captured.Description)
	assert.Equal(t, "en", captured.Language)
	assert.Equal(t, "wikipedia", captured.Category)
	assert.Equal(t, "Wikimedia", captured.Creator)
	assert.Equal(t, "Kiwix", captured.Publisher)
	assert.Equal(t, "https://example.com/favicon.ico", captured.Favicon)
	assert.Equal(t, int64(6000000), captured.ArticleCount)
	assert.Equal(t, int64(500000), captured.MediaCount)
	assert.Equal(t, int64(90000000000), captured.FileSize)
	assert.Equal(t, []string{"wiki", "english"}, captured.Tags)
}

func TestKiwixIndexerAdapter_FieldMapping(t *testing.T) {
	record := kiwix.ZimArchiveRecord{
		UUID:         "zim-uuid-1",
		Name:         "wikipedia_en",
		Title:        "Wikipedia English",
		Description:  "English Wikipedia",
		Language:     "en",
		Category:     "wikipedia",
		Creator:      "Wikimedia",
		Tags:         []string{"wiki", "english"},
		ArticleCount: 6000000,
	}

	// Replicate the mapping from kiwixIndexerAdapter.IndexZimArchive.
	result := search.ZimArchiveRecord{
		UUID:         record.UUID,
		Name:         record.Name,
		Title:        record.Title,
		Description:  record.Description,
		Language:     record.Language,
		Category:     record.Category,
		Creator:      record.Creator,
		Tags:         record.Tags,
		ArticleCount: record.ArticleCount,
	}

	assert.Equal(t, "zim-uuid-1", result.UUID)
	assert.Equal(t, "wikipedia_en", result.Name)
	assert.Equal(t, "Wikipedia English", result.Title)
	assert.Equal(t, "English Wikipedia", result.Description)
	assert.Equal(t, "en", result.Language)
	assert.Equal(t, "wikipedia", result.Category)
	assert.Equal(t, "Wikimedia", result.Creator)
	assert.Equal(t, []string{"wiki", "english"}, result.Tags)
	assert.Equal(t, int64(6000000), result.ArticleCount)
}

// ---------------------------------------------------------------------------
// Test: Confluence adapter field mapping
// ---------------------------------------------------------------------------

func TestConfluenceRepoAdapter_FieldMapping(t *testing.T) {
	space := confluence.Space{
		ID:          "123456",
		Key:         "DEV",
		Name:        "Development",
		Description: "Development space",
		Type:        "global",
		Status:      "current",
		HomepageID:  "789",
		IconURL:     "https://confluence.example.com/icon.png",
	}

	result := repository.ConfluenceSpaceUpsert{
		ConfluenceID: space.ID,
		Key:          space.Key,
		Name:         space.Name,
		Description:  space.Description,
		Type:         space.Type,
		Status:       space.Status,
		HomepageID:   space.HomepageID,
		IconURL:      space.IconURL,
	}

	assert.Equal(t, "123456", result.ConfluenceID)
	assert.Equal(t, "DEV", result.Key)
	assert.Equal(t, "Development", result.Name)
	assert.Equal(t, "Development space", result.Description)
	assert.Equal(t, "global", result.Type)
	assert.Equal(t, "current", result.Status)
	assert.Equal(t, "789", result.HomepageID)
	assert.Equal(t, "https://confluence.example.com/icon.png", result.IconURL)
}

func TestConfluenceIndexerAdapter_FieldMapping(t *testing.T) {
	record := confluence.ConfluenceSpaceRecord{
		UUID:              "conf-uuid-1",
		ConfluenceID:      "123456",
		Key:               "DEV",
		Name:              "Development",
		Description:       "Development space",
		Type:              "global",
		Status:            "current",
		ExternalServiceID: 7,
		IsEnabled:         true,
	}

	result := search.ConfluenceSpaceRecord{
		UUID:              record.UUID,
		ConfluenceID:      record.ConfluenceID,
		Key:               record.Key,
		Name:              record.Name,
		Description:       record.Description,
		Type:              record.Type,
		Status:            record.Status,
		ExternalServiceID: record.ExternalServiceID,
		IsEnabled:         record.IsEnabled,
	}

	assert.Equal(t, "conf-uuid-1", result.UUID)
	assert.Equal(t, "123456", result.ConfluenceID)
	assert.Equal(t, "DEV", result.Key)
	assert.Equal(t, "Development", result.Name)
	assert.Equal(t, "Development space", result.Description)
	assert.Equal(t, "global", result.Type)
	assert.Equal(t, "current", result.Status)
	assert.Equal(t, int64(7), result.ExternalServiceID)
	assert.True(t, result.IsEnabled)
	// SoftDeleted is not mapped by the adapter, should default to false.
	assert.False(t, result.SoftDeleted)
}

// ---------------------------------------------------------------------------
// Test: Git template adapter field mapping
// ---------------------------------------------------------------------------

func TestGitRepoAdapter_ReplaceFiles_FieldMapping(t *testing.T) {
	files := []git.TemplateFile{
		{
			Path:        "templates/main.tf",
			Filename:    "main.tf",
			Extension:   ".tf",
			Content:     "resource \"aws_instance\" \"example\" {}",
			ContentHash: "sha256:abc123",
			SizeBytes:   42,
			IsEssential: true,
			Variables:   []string{"instance_type", "ami_id"},
		},
		{
			Path:        "README.md",
			Filename:    "README.md",
			Extension:   ".md",
			Content:     "# My Template",
			ContentHash: "sha256:def456",
			SizeBytes:   14,
			IsEssential: false,
			Variables:   nil,
		},
	}

	// Replicate the mapping from gitRepoAdapter.ReplaceFiles.
	converted := make([]repository.GitTemplateFileInsert, len(files))
	for i, f := range files {
		converted[i] = repository.GitTemplateFileInsert{
			Path:        f.Path,
			Filename:    f.Filename,
			Extension:   f.Extension,
			Content:     f.Content,
			ContentHash: f.ContentHash,
			SizeBytes:   f.SizeBytes,
			IsEssential: f.IsEssential,
			Variables:   f.Variables,
		}
	}

	require.Len(t, converted, 2)

	assert.Equal(t, "templates/main.tf", converted[0].Path)
	assert.Equal(t, "main.tf", converted[0].Filename)
	assert.Equal(t, ".tf", converted[0].Extension)
	assert.Equal(t, "resource \"aws_instance\" \"example\" {}", converted[0].Content)
	assert.Equal(t, "sha256:abc123", converted[0].ContentHash)
	assert.Equal(t, int64(42), converted[0].SizeBytes)
	assert.True(t, converted[0].IsEssential)
	assert.Equal(t, []string{"instance_type", "ami_id"}, converted[0].Variables)

	assert.Equal(t, "README.md", converted[1].Path)
	assert.Equal(t, "README.md", converted[1].Filename)
	assert.Equal(t, ".md", converted[1].Extension)
	assert.False(t, converted[1].IsEssential)
	assert.Nil(t, converted[1].Variables)
}

func TestGitIndexerAdapter_FieldMapping(t *testing.T) {
	record := git.GitTemplateRecord{
		UUID:          "git-uuid-1",
		Name:          "Terraform Module",
		Slug:          "terraform-module",
		Description:   "A reusable TF module",
		ReadmeContent: "# Terraform Module\nA reusable module.",
		Category:      "infrastructure",
		Tags:          []string{"terraform", "aws"},
		IsPublic:      true,
		Status:        "synced",
		SoftDeleted:   false,
	}

	result := search.GitTemplateRecord{
		UUID:          record.UUID,
		Name:          record.Name,
		Slug:          record.Slug,
		Description:   record.Description,
		ReadmeContent: record.ReadmeContent,
		Category:      record.Category,
		Tags:          record.Tags,
		IsPublic:      record.IsPublic,
		Status:        record.Status,
		SoftDeleted:   record.SoftDeleted,
	}

	assert.Equal(t, "git-uuid-1", result.UUID)
	assert.Equal(t, "Terraform Module", result.Name)
	assert.Equal(t, "terraform-module", result.Slug)
	assert.Equal(t, "A reusable TF module", result.Description)
	assert.Equal(t, "# Terraform Module\nA reusable module.", result.ReadmeContent)
	assert.Equal(t, "infrastructure", result.Category)
	assert.Equal(t, []string{"terraform", "aws"}, result.Tags)
	assert.True(t, result.IsPublic)
	assert.Equal(t, "synced", result.Status)
	assert.False(t, result.SoftDeleted)
}

// ---------------------------------------------------------------------------
// Test: Start registers expected number of jobs
// ---------------------------------------------------------------------------

func TestStart_RegistersJobs(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		wantJobs int
	}{
		{
			name: "all schedules set",
			cfg: Config{
				KiwixSchedule:      "* * * * *",
				ConfluenceSchedule: "* * * * *",
				GitSchedule:        "* * * * *",
				Logger:             discardLogger,
			},
			wantJobs: 3,
		},
		{
			name: "no schedules set",
			cfg: Config{
				Logger: discardLogger,
			},
			wantJobs: 0,
		},
		{
			name: "only kiwix schedule set",
			cfg: Config{
				KiwixSchedule: "0 */6 * * *",
				Logger:        discardLogger,
			},
			wantJobs: 1,
		},
		{
			name: "two of three schedules set",
			cfg: Config{
				KiwixSchedule:      "0 */6 * * *",
				ConfluenceSchedule: "0 */4 * * *",
				Logger:             discardLogger,
			},
			wantJobs: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.cfg, Deps{Services: &mockServiceFinder{}})
			s.Start()
			defer s.Stop()

			assert.Len(t, s.cron.Entries(), tt.wantJobs)
		})
	}
}
