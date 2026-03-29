package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/c-premus/documcp/internal/crypto"
	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/model"
)

// GitTemplateFileInsert holds the fields needed to insert a git template file.
type GitTemplateFileInsert struct {
	Path        string
	Filename    string
	Extension   string
	Content     string
	ContentHash string
	SizeBytes   int64
	IsEssential bool
	Variables   []string
}

// GitTemplateRepository handles git template persistence.
type GitTemplateRepository struct {
	db        *pgxpool.Pool
	logger    *slog.Logger
	encryptor *crypto.Encryptor // nil disables encryption
}

// NewGitTemplateRepository creates a new GitTemplateRepository.
func NewGitTemplateRepository(db *pgxpool.Pool, logger *slog.Logger, enc *crypto.Encryptor) *GitTemplateRepository {
	return &GitTemplateRepository{db: db, logger: logger, encryptor: enc}
}

// List returns enabled, non-deleted git templates with an optional category filter.
func (r *GitTemplateRepository) List(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error) {
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
		argIdx++
	}

	if offset > 0 {
		q += fmt.Sprintf(` OFFSET $%d`, argIdx)
		args = append(args, offset)
	}

	templates, err := database.Select[model.GitTemplate](ctx, r.db, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing git templates: %w", err)
	}
	r.decryptTokens(templates)
	return templates, nil
}

// CountFiltered returns the total number of enabled, non-deleted git templates matching the given filters.
func (r *GitTemplateRepository) CountFiltered(ctx context.Context, category string) (int, error) {
	q := `SELECT COUNT(*) FROM git_templates WHERE is_enabled = true AND deleted_at IS NULL`
	args := []any{}
	argIdx := 1

	if category != "" {
		q += fmt.Sprintf(` AND category = $%d`, argIdx)
		args = append(args, category)
	}

	var count int
	err := r.db.QueryRow(ctx, q, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting git templates: %w", err)
	}
	return count, nil
}

// FindByUUID returns a git template by its UUID, if enabled and not soft-deleted.
func (r *GitTemplateRepository) FindByUUID(ctx context.Context, uuid string) (*model.GitTemplate, error) {
	tmpl, err := database.Get[model.GitTemplate](ctx, r.db,
		`SELECT * FROM git_templates WHERE uuid = $1 AND deleted_at IS NULL AND is_enabled = true`, uuid)
	if err != nil {
		return nil, fmt.Errorf("finding git template by uuid %s: %w", uuid, err)
	}
	r.decryptToken(&tmpl)
	return &tmpl, nil
}

// FilesForTemplate returns all files belonging to a git template.
func (r *GitTemplateRepository) FilesForTemplate(ctx context.Context, templateID int64) ([]model.GitTemplateFile, error) {
	files, err := database.Select[model.GitTemplateFile](ctx, r.db,
		`SELECT * FROM git_template_files WHERE git_template_id = $1 ORDER BY path`, templateID)
	if err != nil {
		return nil, fmt.Errorf("finding files for git template %d: %w", templateID, err)
	}
	return files, nil
}

// FindFileByPath returns a single template file by template ID and path.
func (r *GitTemplateRepository) FindFileByPath(ctx context.Context, templateID int64, path string) (*model.GitTemplateFile, error) {
	file, err := database.Get[model.GitTemplateFile](ctx, r.db,
		`SELECT * FROM git_template_files WHERE git_template_id = $1 AND path = $2`, templateID, path)
	if err != nil {
		return nil, fmt.Errorf("finding file by path %q in git template %d: %w", path, templateID, err)
	}
	return &file, nil
}

// ListAll returns all non-deleted git templates (including disabled) with optional search query.
func (r *GitTemplateRepository) ListAll(ctx context.Context, query string, limit int) ([]model.GitTemplate, error) {
	q := `SELECT * FROM git_templates WHERE deleted_at IS NULL`
	args := []any{}
	argIdx := 1

	if query != "" {
		q += fmt.Sprintf(` AND (name ILIKE $%d OR description ILIKE $%d)`, argIdx, argIdx+1)
		likeQuery := "%" + query + "%"
		args = append(args, likeQuery, likeQuery)
		argIdx += 2
	}

	q += ` ORDER BY name`
	if limit > 0 {
		q += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, limit)
	}

	templates, err := database.Select[model.GitTemplate](ctx, r.db, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing all git templates: %w", err)
	}
	r.decryptTokens(templates)
	return templates, nil
}

// ListAllUUIDs returns all non-deleted git template UUIDs.
func (r *GitTemplateRepository) ListAllUUIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT uuid FROM git_templates WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("listing all git template uuids: %w", err)
	}
	uuids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("listing all git template uuids: %w", err)
	}
	return uuids, nil
}

// FindByUUIDs returns non-deleted git templates matching the given UUIDs.
// Used by search index reconciliation to re-index missing entries.
func (r *GitTemplateRepository) FindByUUIDs(ctx context.Context, uuids []string) ([]model.GitTemplate, error) {
	if len(uuids) == 0 {
		return nil, nil
	}
	templates, err := database.Select[model.GitTemplate](ctx, r.db,
		`SELECT * FROM git_templates WHERE uuid = ANY($1) AND deleted_at IS NULL`, uuids)
	if err != nil {
		return nil, fmt.Errorf("finding git templates by uuids: %w", err)
	}
	r.decryptTokens(templates)
	return templates, nil
}

// Count returns the total number of non-deleted git templates.
func (r *GitTemplateRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM git_templates WHERE deleted_at IS NULL`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting git templates: %w", err)
	}
	return count, nil
}

// FindBySlug returns a git template by its slug, if enabled and not soft-deleted.
func (r *GitTemplateRepository) FindBySlug(ctx context.Context, slug string) (*model.GitTemplate, error) {
	tmpl, err := database.Get[model.GitTemplate](ctx, r.db,
		`SELECT * FROM git_templates WHERE slug = $1 AND deleted_at IS NULL AND is_enabled = true`, slug)
	if err != nil {
		return nil, fmt.Errorf("finding git template by slug %s: %w", slug, err)
	}
	r.decryptToken(&tmpl)
	return &tmpl, nil
}

// Create inserts a new git template and sets the generated ID, UUID, and timestamps.
func (r *GitTemplateRepository) Create(ctx context.Context, tmpl *model.GitTemplate) error {
	encToken, err := r.encryptToken(tmpl.GitToken)
	if err != nil {
		return err
	}
	err = r.db.QueryRow(ctx,
		`INSERT INTO git_templates (
			uuid, name, slug, description, repository_url, branch, git_token,
			readme_content, manifest, category, tags, user_id,
			is_public, is_enabled, status, error_message,
			file_count, total_size_bytes,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, $16,
			$17, $18,
			NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		tmpl.UUID, tmpl.Name, tmpl.Slug, tmpl.Description,
		tmpl.RepositoryURL, tmpl.Branch, encToken,
		tmpl.ReadmeContent, tmpl.Manifest, tmpl.Category, tmpl.Tags, tmpl.UserID,
		tmpl.IsPublic, tmpl.IsEnabled, tmpl.Status, tmpl.ErrorMessage,
		tmpl.FileCount, tmpl.TotalSizeBytes,
	).Scan(&tmpl.ID, &tmpl.CreatedAt, &tmpl.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating git template %q: %w", tmpl.Name, err)
	}
	return nil
}

// Update updates an existing git template by its ID.
func (r *GitTemplateRepository) Update(ctx context.Context, tmpl *model.GitTemplate) error {
	encToken, err := r.encryptToken(tmpl.GitToken)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`UPDATE git_templates SET
			name = $1, slug = $2, description = $3, repository_url = $4,
			branch = $5, git_token = $6, readme_content = $7, manifest = $8,
			category = $9, tags = $10, is_public = $11, is_enabled = $12,
			status = $13, error_message = $14, updated_at = NOW()
		WHERE id = $15 AND deleted_at IS NULL`,
		tmpl.Name, tmpl.Slug, tmpl.Description, tmpl.RepositoryURL,
		tmpl.Branch, encToken, tmpl.ReadmeContent, tmpl.Manifest,
		tmpl.Category, tmpl.Tags, tmpl.IsPublic, tmpl.IsEnabled,
		tmpl.Status, tmpl.ErrorMessage, tmpl.ID,
	)
	if err != nil {
		return fmt.Errorf("updating git template %d: %w", tmpl.ID, err)
	}
	return nil
}

// SoftDelete marks a git template as deleted by setting deleted_at.
func (r *GitTemplateRepository) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE git_templates SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft deleting git template %d: %w", id, err)
	}
	return nil
}

// UpdateSyncStatus updates the sync-related fields for a git template.
func (r *GitTemplateRepository) UpdateSyncStatus(ctx context.Context, templateID int64, status, commitSHA string, fileCount int, totalSize int64, errMsg string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE git_templates SET
			status = $1,
			last_commit_sha = $2,
			file_count = $3,
			total_size_bytes = $4,
			error_message = CASE WHEN $5 = '' THEN NULL ELSE $5 END,
			last_synced_at = NOW(),
			updated_at = NOW()
		WHERE id = $6`,
		status, commitSHA, fileCount, totalSize, errMsg, templateID,
	)
	if err != nil {
		return fmt.Errorf("updating sync status for git template %d: %w", templateID, err)
	}
	return nil
}

// UpdateSearchContent populates readme_content and file_paths for FTS indexing.
func (r *GitTemplateRepository) UpdateSearchContent(ctx context.Context, templateID int64, readmeContent, filePaths string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE git_templates SET
			readme_content = CASE WHEN $1 = '' THEN NULL ELSE $1 END,
			file_paths = CASE WHEN $2 = '' THEN NULL ELSE $2 END,
			updated_at = NOW()
		WHERE id = $3`,
		readmeContent, filePaths, templateID,
	)
	if err != nil {
		return fmt.Errorf("updating search content for git template %d: %w", templateID, err)
	}
	return nil
}

// ReplaceFiles deletes existing files for a template and inserts new ones in a transaction.
func (r *GitTemplateRepository) ReplaceFiles(ctx context.Context, templateID int64, files []GitTemplateFileInsert) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction for replacing files on template %d: %w", templateID, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op; error is irrelevant

	_, err = tx.Exec(ctx,
		`DELETE FROM git_template_files WHERE git_template_id = $1`, templateID)
	if err != nil {
		return fmt.Errorf("deleting files for git template %d: %w", templateID, err)
	}

	for _, f := range files {
		var variablesJSON *string
		if len(f.Variables) > 0 {
			b, jsonErr := json.Marshal(f.Variables)
			if jsonErr != nil {
				return fmt.Errorf("marshaling variables for file %q: %w", f.Path, jsonErr)
			}
			s := string(b)
			variablesJSON = &s
		}

		var ext *string
		if f.Extension != "" {
			ext = &f.Extension
		}
		var contentHash *string
		if f.ContentHash != "" {
			contentHash = &f.ContentHash
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO git_template_files (
				uuid, git_template_id, path, filename, extension,
				content, size_bytes, content_hash, is_essential, variables,
				created_at, updated_at
			) VALUES (
				gen_random_uuid(), $1, $2, $3, $4,
				$5, $6, $7, $8, $9,
				NOW(), NOW()
			)`,
			templateID, f.Path, f.Filename, ext,
			f.Content, f.SizeBytes, contentHash, f.IsEssential, variablesJSON,
		)
		if err != nil {
			return fmt.Errorf("inserting file %q for git template %d: %w", f.Path, templateID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing file replacement for git template %d: %w", templateID, err)
	}
	return nil
}

// Search returns git templates matching a text query and optional category filter.
// It searches across name, description, and readme_content using ILIKE.
func (r *GitTemplateRepository) Search(ctx context.Context, query, category string, limit int) ([]model.GitTemplate, error) {
	q := `SELECT * FROM git_templates WHERE is_enabled = true AND deleted_at IS NULL`
	args := []any{}
	argIdx := 1

	if query != "" {
		likeQuery := "%" + query + "%"
		q += fmt.Sprintf(` AND (name ILIKE $%d OR description ILIKE $%d OR readme_content ILIKE $%d)`,
			argIdx, argIdx+1, argIdx+2)
		args = append(args, likeQuery, likeQuery, likeQuery)
		argIdx += 3
	}

	if category != "" {
		q += fmt.Sprintf(` AND category = $%d`, argIdx)
		args = append(args, category)
		argIdx++
	}

	q += ` ORDER BY name`

	if limit <= 0 {
		limit = 50
	}
	q += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit)

	templates, err := database.Select[model.GitTemplate](ctx, r.db, q, args...)
	if err != nil {
		return nil, fmt.Errorf("searching git templates for %q: %w", query, err)
	}
	r.decryptTokens(templates)
	return templates, nil
}

// encryptToken encrypts a git token for storage.
func (r *GitTemplateRepository) encryptToken(token sql.NullString) (sql.NullString, error) {
	if !token.Valid || token.String == "" {
		return token, nil
	}
	enc, err := r.encryptor.Encrypt(token.String)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("encrypting git token: %w", err)
	}
	return sql.NullString{String: enc, Valid: true}, nil
}

// decryptToken decrypts a git token after loading from the database.
func (r *GitTemplateRepository) decryptToken(tmpl *model.GitTemplate) {
	if !tmpl.GitToken.Valid || tmpl.GitToken.String == "" {
		return
	}
	dec, err := r.encryptor.Decrypt(tmpl.GitToken.String)
	if err != nil {
		r.logger.Warn("decrypting git token (may be plaintext)", "template_id", tmpl.ID, "error", err)
		return
	}
	tmpl.GitToken.String = dec
}

// decryptTokens decrypts git tokens in a slice of templates.
func (r *GitTemplateRepository) decryptTokens(templates []model.GitTemplate) {
	for i := range templates {
		r.decryptToken(&templates[i])
	}
}
