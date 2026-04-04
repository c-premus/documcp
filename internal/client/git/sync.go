package git

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/c-premus/documcp/internal/model"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SyncParams holds dependencies for syncing a git template.
type SyncParams struct {
	Template SyncTemplate
	Client   *Client
	Repo     TemplateRepo
	Logger   *slog.Logger
}

// SyncTemplate represents a git template to be synced.
type SyncTemplate struct {
	ID            int64
	UUID          string
	Name          string
	Slug          string
	Description   string
	RepositoryURL string
	Branch        string
	Token         string // Decrypted PAT
	Category      string
	Tags          []string
	LastCommitSHA string
}

// TemplateRepo defines repository methods needed for sync.
type TemplateRepo interface {
	UpdateSyncStatus(ctx context.Context, templateID int64, status model.GitTemplateStatus, commitSHA string, fileCount int, totalSize int64, errMsg string) error
	ReplaceFiles(ctx context.Context, templateID int64, files []TemplateFile) error
	UpdateSearchContent(ctx context.Context, templateID int64, readmeContent, filePaths string) error
}

// Sync clones or pulls a template repository, extracts files, and persists
// them to the database. Search indexing is handled automatically by PostgreSQL
// triggers on the git_templates table.
func Sync(ctx context.Context, params SyncParams) (retErr error) {
	ctx, span := otel.Tracer("documcp/git").Start(ctx, "git.sync",
		trace.WithAttributes(
			attribute.String("git.template_id", params.Template.UUID),
			attribute.String("git.url", params.Template.RepositoryURL),
			attribute.String("git.branch", params.Template.Branch),
		),
	)
	defer func() {
		if retErr != nil {
			span.RecordError(retErr)
			span.SetStatus(codes.Error, retErr.Error())
		}
		span.End()
	}()

	tmpl := params.Template
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info("starting template sync",
		"template_id", tmpl.ID,
		"uuid", tmpl.UUID,
		"url", tmpl.RepositoryURL,
	)

	// 1. Validate repository URL.
	if err := ValidateRepositoryURL(tmpl.RepositoryURL, true); err != nil {
		syncErr := fmt.Sprintf("invalid repository URL: %v", err)
		if statusErr := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, model.GitTemplateStatusFailed, "", 0, 0, syncErr); statusErr != nil {
			logger.Warn("failed to update sync status", "template_id", tmpl.ID, "target_status", "failed", "error", statusErr)
		}
		return fmt.Errorf("validating repository URL: %w", err)
	}

	// 2. Clone or pull.
	dest := filepath.Join(params.Client.tempDir, tmpl.Slug)
	var repoDir string

	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		// Existing clone — pull latest.
		if err := params.Client.Pull(ctx, dest, tmpl.Token); err != nil {
			syncErr := fmt.Sprintf("pull failed: %v", err)
			if statusErr := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, model.GitTemplateStatusFailed, "", 0, 0, syncErr); statusErr != nil {
				logger.Warn("failed to update sync status", "template_id", tmpl.ID, "target_status", "failed", "error", statusErr)
			}
			return fmt.Errorf("pulling template repo: %w", err)
		}
		repoDir = dest
	} else {
		// Fresh clone.
		dir, err := params.Client.Clone(ctx, CloneParams{
			URL:    tmpl.RepositoryURL,
			Branch: tmpl.Branch,
			Token:  tmpl.Token,
			Dest:   dest,
		})
		if err != nil {
			syncErr := fmt.Sprintf("clone failed: %v", err)
			if statusErr := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, model.GitTemplateStatusFailed, "", 0, 0, syncErr); statusErr != nil {
				logger.Warn("failed to update sync status", "template_id", tmpl.ID, "target_status", "failed", "error", statusErr)
			}
			return fmt.Errorf("cloning template repo: %w", err)
		}
		repoDir = dir
	}

	// 3. Get latest commit SHA.
	commitSHA, err := params.Client.LatestCommitSHA(ctx, repoDir)
	if err != nil {
		syncErr := fmt.Sprintf("rev-parse failed: %v", err)
		if statusErr := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, model.GitTemplateStatusFailed, "", 0, 0, syncErr); statusErr != nil {
			logger.Warn("failed to update sync status", "template_id", tmpl.ID, "target_status", "failed", "error", statusErr)
		}
		return fmt.Errorf("getting latest commit SHA: %w", err)
	}

	// Skip if already synced to this commit.
	if commitSHA == tmpl.LastCommitSHA {
		logger.Info("template already up to date",
			"template_id", tmpl.ID,
			"commit", commitSHA,
		)
		return nil
	}

	// 4. Extract files.
	files, err := params.Client.ExtractFiles(repoDir, DefaultMaxFileSize, DefaultMaxTotalSize)
	if err != nil {
		syncErr := fmt.Sprintf("file extraction failed: %v", err)
		if statusErr := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, model.GitTemplateStatusFailed, commitSHA, 0, 0, syncErr); statusErr != nil {
			logger.Warn("failed to update sync status", "template_id", tmpl.ID, "target_status", "failed", "error", statusErr)
		}
		return fmt.Errorf("extracting template files: %w", err)
	}

	// 5. Compute total size.
	var totalSize int64
	for _, f := range files {
		totalSize += f.SizeBytes
	}

	// 6. Update sync status in DB.
	if err := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, model.GitTemplateStatusSynced, commitSHA, len(files), totalSize, ""); err != nil {
		return fmt.Errorf("updating sync status: %w", err)
	}

	// 7. Replace files in DB.
	if err := params.Repo.ReplaceFiles(ctx, tmpl.ID, files); err != nil {
		syncErr := fmt.Sprintf("replacing files failed: %v", err)
		if statusErr := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, model.GitTemplateStatusFailed, commitSHA, len(files), totalSize, syncErr); statusErr != nil {
			logger.Warn("failed to update sync status", "template_id", tmpl.ID, "target_status", "failed", "error", statusErr)
		}
		return fmt.Errorf("replacing template files: %w", err)
	}

	// 8. Populate search content (readme + file paths for FTS).
	readmeContent := findReadmeContent(files)
	filePaths := buildFilePaths(files)
	if err := params.Repo.UpdateSearchContent(ctx, tmpl.ID, readmeContent, filePaths); err != nil {
		logger.Warn("failed to update search content", "template_id", tmpl.ID, "error", err)
	}

	span.SetAttributes(
		attribute.String("git.commit_sha", commitSHA),
		attribute.Int("git.file_count", len(files)),
		attribute.Int64("git.total_bytes", totalSize),
	)

	logger.Info("template sync complete",
		"template_id", tmpl.ID,
		"uuid", tmpl.UUID,
		"commit", commitSHA,
		"file_count", len(files),
		"total_bytes", totalSize,
	)

	return nil
}

// findReadmeContent returns the content of the first README.md file found.
func findReadmeContent(files []TemplateFile) string {
	for _, f := range files {
		if strings.EqualFold(f.Filename, "README.md") {
			return f.Content
		}
	}
	return ""
}

// buildFilePaths builds a space-separated string of file paths and humanized
// filenames for FTS indexing. Each file contributes its path plus a version with
// hyphens/underscores/dots replaced by spaces (e.g., "spring-boot-engineer.md"
// becomes "spring-boot-engineer.md spring boot engineer").
func buildFilePaths(files []TemplateFile) string {
	var b strings.Builder
	for i, f := range files {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(f.Path)

		// Add humanized filename (strip extension, replace separators with spaces).
		name := f.Filename
		if ext := filepath.Ext(name); ext != "" {
			name = name[:len(name)-len(ext)]
		}
		humanized := strings.NewReplacer("-", " ", "_", " ", ".", " ").Replace(name)
		if humanized != name && humanized != "" {
			b.WriteByte(' ')
			b.WriteString(humanized)
		}
	}
	return b.String()
}
