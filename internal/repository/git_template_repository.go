package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// GitTemplateRepository handles git template persistence.
type GitTemplateRepository struct {
	db     *sqlx.DB
	logger *slog.Logger
}

// NewGitTemplateRepository creates a new GitTemplateRepository.
func NewGitTemplateRepository(db *sqlx.DB, logger *slog.Logger) *GitTemplateRepository {
	return &GitTemplateRepository{db: db, logger: logger}
}

// List returns enabled, non-deleted git templates with an optional category filter.
func (r *GitTemplateRepository) List(ctx context.Context, category string, limit int) ([]model.GitTemplate, error) {
	q := `SELECT * FROM git_templates WHERE is_enabled = true AND deleted_at IS NULL`
	args := []any{}
	argIdx := 1

	if category != "" {
		q += fmt.Sprintf(` AND category = $%d`, argIdx)
		args = append(args, category)
		argIdx++
	}

	q += ` ORDER BY name`

	if limit > 0 {
		q += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, limit)
	}

	var templates []model.GitTemplate
	err := r.db.SelectContext(ctx, &templates, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing git templates: %w", err)
	}
	return templates, nil
}

// FindByUUID returns a git template by its UUID, if enabled and not soft-deleted.
func (r *GitTemplateRepository) FindByUUID(ctx context.Context, uuid string) (*model.GitTemplate, error) {
	var tmpl model.GitTemplate
	err := r.db.GetContext(ctx, &tmpl,
		`SELECT * FROM git_templates WHERE uuid = $1 AND deleted_at IS NULL AND is_enabled = true`, uuid)
	if err != nil {
		return nil, fmt.Errorf("finding git template by uuid %s: %w", uuid, err)
	}
	return &tmpl, nil
}

// FilesForTemplate returns all files belonging to a git template.
func (r *GitTemplateRepository) FilesForTemplate(ctx context.Context, templateID int64) ([]model.GitTemplateFile, error) {
	var files []model.GitTemplateFile
	err := r.db.SelectContext(ctx, &files,
		`SELECT * FROM git_template_files WHERE git_template_id = $1 ORDER BY path`, templateID)
	if err != nil {
		return nil, fmt.Errorf("finding files for git template %d: %w", templateID, err)
	}
	return files, nil
}

// FindFileByPath returns a single template file by template ID and path.
func (r *GitTemplateRepository) FindFileByPath(ctx context.Context, templateID int64, path string) (*model.GitTemplateFile, error) {
	var file model.GitTemplateFile
	err := r.db.GetContext(ctx, &file,
		`SELECT * FROM git_template_files WHERE git_template_id = $1 AND path = $2`, templateID, path)
	if err != nil {
		return nil, fmt.Errorf("finding file by path %q in git template %d: %w", path, templateID, err)
	}
	return &file, nil
}
