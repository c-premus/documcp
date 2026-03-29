package git

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
	UpdateSyncStatus(ctx context.Context, templateID int64, status, commitSHA string, fileCount int, totalSize int64, errMsg string) error
	ReplaceFiles(ctx context.Context, templateID int64, files []TemplateFile) error
}

// Sync clones or pulls a template repository, extracts files, and persists
// them to the database. Search indexing is handled automatically by PostgreSQL
// triggers on the git_templates table.
func Sync(ctx context.Context, params SyncParams) error {
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
	if err := ValidateRepositoryURL(tmpl.RepositoryURL); err != nil {
		syncErr := fmt.Sprintf("invalid repository URL: %v", err)
		if statusErr := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, "failed", "", 0, 0, syncErr); statusErr != nil {
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
			if statusErr := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, "failed", "", 0, 0, syncErr); statusErr != nil {
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
			if statusErr := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, "failed", "", 0, 0, syncErr); statusErr != nil {
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
		if statusErr := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, "failed", "", 0, 0, syncErr); statusErr != nil {
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
		if statusErr := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, "failed", commitSHA, 0, 0, syncErr); statusErr != nil {
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
	if err := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, "synced", commitSHA, len(files), totalSize, ""); err != nil {
		return fmt.Errorf("updating sync status: %w", err)
	}

	// 7. Replace files in DB.
	if err := params.Repo.ReplaceFiles(ctx, tmpl.ID, files); err != nil {
		syncErr := fmt.Sprintf("replacing files failed: %v", err)
		if statusErr := params.Repo.UpdateSyncStatus(ctx, tmpl.ID, "failed", commitSHA, len(files), totalSize, syncErr); statusErr != nil {
			logger.Warn("failed to update sync status", "template_id", tmpl.ID, "target_status", "failed", "error", statusErr)
		}
		return fmt.Errorf("replacing template files: %w", err)
	}

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
