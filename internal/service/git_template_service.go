package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/c-premus/documcp/internal/archive"
	gitclient "github.com/c-premus/documcp/internal/client/git"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/queue"
	"github.com/c-premus/documcp/internal/security"
	"github.com/c-premus/documcp/internal/stringutil"
)

// GitTemplateRepo defines the repository methods the git template service needs.
type GitTemplateRepo interface {
	List(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, error)
	CountFiltered(ctx context.Context, category string) (int, error)
	Search(ctx context.Context, query, category string, limit int) ([]model.GitTemplate, error)
	FindByUUID(ctx context.Context, uuid string) (*model.GitTemplate, error)
	FilesForTemplate(ctx context.Context, templateID int64) ([]model.GitTemplateFile, error)
	FindFileByPath(ctx context.Context, templateID int64, path string) (*model.GitTemplateFile, error)
	Create(ctx context.Context, tmpl *model.GitTemplate) error
	Update(ctx context.Context, tmpl *model.GitTemplate) error
	SoftDelete(ctx context.Context, id int64) error
}

// gitTemplateEncryptor encrypts sensitive values before storage.
type gitTemplateEncryptor interface {
	Encrypt(plaintext string) (string, error)
}

// Sentinel errors for git template operations.
var (
	// ErrGitTemplateNotFound indicates the requested git template does not exist.
	ErrGitTemplateNotFound = errors.New("git template not found")

	// ErrEncryptionDisabled indicates encryption is not configured but a git token was provided.
	ErrEncryptionDisabled = errors.New("encryption not available: cannot store git token")
)

// TemplateStructure holds the resolved structure of a git template.
type TemplateStructure struct {
	Template       *model.GitTemplate
	Files          []model.GitTemplateFile
	FileTree       []string
	EssentialFiles []string
	Variables      []string
	FileCount      int
	TotalSize      int64
}

// FileResult holds a file's content after optional variable substitution.
type FileResult struct {
	File       *model.GitTemplateFile
	Content    string
	Unresolved []string
}

// DeploymentGuide holds a deployment guide with essential files.
type DeploymentGuide struct {
	Template   *model.GitTemplate
	Steps      []string
	Files      []DeploymentFile
	Unresolved []string
}

// DeploymentFile holds a single file's path and substituted content.
type DeploymentFile struct {
	Path    string
	Content string
}

// ArchiveResult holds a built archive ready for delivery.
type ArchiveResult struct {
	Filename   string
	Format     string
	Data       []byte
	FileCount  int
	Unresolved []string
}

// CreateGitTemplateInput holds parameters for creating a git template.
type CreateGitTemplateInput struct {
	Name          string
	Description   string
	RepositoryURL string
	Branch        string
	GitToken      string
	Category      string
	Tags          []string
	IsPublic      bool
}

// UpdateGitTemplateInput holds parameters for updating a git template.
// Pointer fields are optional; nil means "do not change".
type UpdateGitTemplateInput struct {
	Name          *string
	Description   *string
	RepositoryURL *string
	Branch        *string
	GitToken      *string
	Category      *string
	Tags          *[]string
	IsPublic      *bool
}

// GitTemplateService orchestrates git template business logic.
type GitTemplateService struct {
	repo      GitTemplateRepo
	inserter  JobInserter
	encryptor gitTemplateEncryptor
	logger    *slog.Logger
}

// NewGitTemplateService creates a new GitTemplateService.
// Pass nil for encryptor to disable git token storage, and nil for inserter
// to disable background job enqueueing.
func NewGitTemplateService(
	repo GitTemplateRepo,
	inserter JobInserter,
	encryptor gitTemplateEncryptor,
	logger *slog.Logger,
) *GitTemplateService {
	return &GitTemplateService{
		repo:      repo,
		inserter:  inserter,
		encryptor: encryptor,
		logger:    logger,
	}
}

// FindByUUID retrieves a git template by its UUID.
// Returns ErrGitTemplateNotFound when the template does not exist.
func (s *GitTemplateService) FindByUUID(ctx context.Context, tmplUUID string) (*model.GitTemplate, error) {
	tmpl, err := s.repo.FindByUUID(ctx, tmplUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrGitTemplateNotFound
		}
		return nil, fmt.Errorf("finding git template by uuid: %w", err)
	}
	return tmpl, nil
}

// List returns a paginated list of git templates filtered by category.
// It returns the templates, total count, and any error.
func (s *GitTemplateService) List(ctx context.Context, category string, limit, offset int) ([]model.GitTemplate, int, error) {
	total, err := s.repo.CountFiltered(ctx, category)
	if err != nil {
		return nil, 0, fmt.Errorf("counting git templates: %w", err)
	}

	templates, err := s.repo.List(ctx, category, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing git templates: %w", err)
	}

	return templates, total, nil
}

// Structure resolves the template, loads its files, and extracts the file
// tree, essential files, and variable placeholders.
func (s *GitTemplateService) Structure(ctx context.Context, tmplUUID string) (*TemplateStructure, error) {
	tmpl, err := s.FindByUUID(ctx, tmplUUID)
	if err != nil {
		return nil, fmt.Errorf("finding git template for structure: %w", err)
	}

	files, err := s.repo.FilesForTemplate(ctx, tmpl.ID)
	if err != nil {
		return nil, fmt.Errorf("listing template files: %w", err)
	}

	fileTree := make([]string, 0, len(files))
	essentialFiles := make([]string, 0)
	variableSet := make(map[string]bool)

	for i := range files {
		f := &files[i]
		fileTree = append(fileTree, f.Path)

		if f.IsEssential {
			essentialFiles = append(essentialFiles, f.Path)
		}

		if f.Content.Valid {
			matches := gitclient.VariablePattern.FindAllStringSubmatch(f.Content.String, -1)
			for _, match := range matches {
				variableSet[match[1]] = true
			}
		}
	}

	variables := make([]string, 0, len(variableSet))
	for v := range variableSet {
		variables = append(variables, v)
	}

	return &TemplateStructure{
		Template:       tmpl,
		Files:          files,
		FileTree:       fileTree,
		EssentialFiles: essentialFiles,
		Variables:      variables,
		FileCount:      tmpl.FileCount,
		TotalSize:      tmpl.TotalSizeBytes,
	}, nil
}

// File retrieves a single template file by path, optionally applying
// variable substitution to its content.
func (s *GitTemplateService) File(ctx context.Context, tmplUUID, path string, variables map[string]string) (*FileResult, error) {
	tmpl, err := s.FindByUUID(ctx, tmplUUID)
	if err != nil {
		return nil, fmt.Errorf("finding git template for file read: %w", err)
	}

	file, err := s.repo.FindFileByPath(ctx, tmpl.ID, path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrGitTemplateNotFound
		}
		return nil, fmt.Errorf("finding template file: %w", err)
	}

	content := file.Content.String
	var unresolved []string

	if len(variables) > 0 {
		content, unresolved = gitclient.SubstituteVariables(content, variables)
	}

	return &FileResult{
		File:       file,
		Content:    content,
		Unresolved: unresolved,
	}, nil
}

// DeploymentGuide generates a deployment guide containing all essential
// files with optional variable substitution applied.
func (s *GitTemplateService) DeploymentGuide(ctx context.Context, tmplUUID string, variables map[string]string) (*DeploymentGuide, error) {
	tmpl, err := s.FindByUUID(ctx, tmplUUID)
	if err != nil {
		return nil, fmt.Errorf("finding git template for deployment guide: %w", err)
	}

	files, err := s.repo.FilesForTemplate(ctx, tmpl.ID)
	if err != nil {
		return nil, fmt.Errorf("listing template files for deployment guide: %w", err)
	}

	allUnresolved := make(map[string]bool)
	deploymentFiles := make([]DeploymentFile, 0)

	for i := range files {
		if !files[i].IsEssential {
			continue
		}
		content := files[i].Content.String
		if len(variables) > 0 {
			var unresolved []string
			content, unresolved = gitclient.SubstituteVariables(content, variables)
			for _, u := range unresolved {
				allUnresolved[u] = true
			}
		}
		deploymentFiles = append(deploymentFiles, DeploymentFile{
			Path:    files[i].Path,
			Content: content,
		})
	}

	unresolvedList := make([]string, 0, len(allUnresolved))
	for v := range allUnresolved {
		unresolvedList = append(unresolvedList, v)
	}

	return &DeploymentGuide{
		Template:   tmpl,
		Steps:      []string{"Create the following files in your project directory."},
		Files:      deploymentFiles,
		Unresolved: unresolvedList,
	}, nil
}

// BuildArchive creates a zip or tar.gz archive of all template files with
// optional variable substitution. The format must be "zip" or "tar.gz".
// Returns the archive as raw bytes (not base64-encoded).
func (s *GitTemplateService) BuildArchive(ctx context.Context, tmplUUID, format string, variables map[string]string) (*ArchiveResult, error) {
	tmpl, err := s.FindByUUID(ctx, tmplUUID)
	if err != nil {
		return nil, fmt.Errorf("finding git template for archive: %w", err)
	}

	files, err := s.repo.FilesForTemplate(ctx, tmpl.ID)
	if err != nil {
		return nil, fmt.Errorf("listing template files for archive: %w", err)
	}

	if format == "" {
		format = "zip"
	}
	if format != "zip" && format != "tar.gz" {
		return nil, fmt.Errorf("unsupported archive format %q: must be \"zip\" or \"tar.gz\"", format)
	}

	allUnresolved := make(map[string]bool)
	entries := make([]archive.Entry, 0, len(files))

	for i := range files {
		content := files[i].Content.String
		if len(variables) > 0 {
			var unresolved []string
			content, unresolved = gitclient.SubstituteVariables(content, variables)
			for _, u := range unresolved {
				allUnresolved[u] = true
			}
		}
		entries = append(entries, archive.Entry{Path: files[i].Path, Content: content})
	}

	var buf bytes.Buffer
	var filename string

	switch format {
	case "tar.gz":
		if err := archive.BuildTarGz(&buf, entries); err != nil {
			return nil, fmt.Errorf("creating tar.gz archive: %w", err)
		}
		filename = tmpl.Slug + ".tar.gz"
	default:
		if err := archive.BuildZip(&buf, entries); err != nil {
			return nil, fmt.Errorf("creating zip archive: %w", err)
		}
		filename = tmpl.Slug + ".zip"
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	unresolvedList := make([]string, 0, len(allUnresolved))
	for v := range allUnresolved {
		unresolvedList = append(unresolvedList, v)
	}

	return &ArchiveResult{
		Filename:   filename,
		Format:     format,
		Data:       []byte(encoded),
		FileCount:  len(entries),
		Unresolved: unresolvedList,
	}, nil
}

// Create creates a new git template, validates the repository URL, encrypts
// the git token if provided, and enqueues a sync job.
func (s *GitTemplateService) Create(ctx context.Context, input CreateGitTemplateInput) (*model.GitTemplate, error) {
	if input.Name == "" {
		return nil, errors.New("name is required")
	}
	if input.RepositoryURL == "" {
		return nil, errors.New("repository_url is required")
	}

	if err := security.ValidateExternalURL(input.RepositoryURL, true); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidURL, err.Error())
	}

	branch := input.Branch
	if branch == "" {
		branch = "main"
	}

	tmpl := &model.GitTemplate{
		UUID:          uuid.New().String(),
		Name:          input.Name,
		Slug:          stringutil.Slugify(input.Name),
		RepositoryURL: input.RepositoryURL,
		Branch:        branch,
		IsPublic:      input.IsPublic,
		IsEnabled:     true,
		Status:        model.GitTemplateStatusPending,
	}

	if input.Description != "" {
		tmpl.Description = sql.NullString{String: input.Description, Valid: true}
	}
	if input.GitToken != "" {
		if s.encryptor == nil {
			return nil, ErrEncryptionDisabled
		}
		tmpl.GitToken = sql.NullString{String: input.GitToken, Valid: true}
	}
	if input.Category != "" {
		tmpl.Category = sql.NullString{String: input.Category, Valid: true}
	}
	if len(input.Tags) > 0 {
		tagsJSON, err := json.Marshal(input.Tags)
		if err != nil {
			return nil, fmt.Errorf("marshaling tags: %w", err)
		}
		tmpl.Tags = sql.NullString{String: string(tagsJSON), Valid: true}
	}

	if err := s.repo.Create(ctx, tmpl); err != nil {
		return nil, fmt.Errorf("creating git template: %w", err)
	}

	if s.inserter != nil {
		if _, err := s.inserter.Insert(ctx, queue.SyncGitTemplatesArgs{}, nil); err != nil {
			s.logger.WarnContext(ctx, "failed to enqueue git template sync after create", "error", err)
		}
	}

	return tmpl, nil
}

// Update applies partial updates to an existing git template identified by UUID.
func (s *GitTemplateService) Update(ctx context.Context, tmplUUID string, input UpdateGitTemplateInput) (*model.GitTemplate, error) {
	tmpl, err := s.FindByUUID(ctx, tmplUUID)
	if err != nil {
		return nil, fmt.Errorf("finding git template for update: %w", err)
	}

	if input.Name != nil && *input.Name != "" {
		tmpl.Name = *input.Name
		tmpl.Slug = stringutil.Slugify(*input.Name)
	}
	if input.RepositoryURL != nil && *input.RepositoryURL != "" {
		if err := security.ValidateExternalURL(*input.RepositoryURL, true); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidURL, err.Error())
		}
		tmpl.RepositoryURL = *input.RepositoryURL
	}
	if input.Description != nil && *input.Description != "" {
		tmpl.Description = sql.NullString{String: *input.Description, Valid: true}
	}
	if input.Branch != nil && *input.Branch != "" {
		tmpl.Branch = *input.Branch
	}
	if input.GitToken != nil && *input.GitToken != "" {
		if s.encryptor == nil {
			return nil, ErrEncryptionDisabled
		}
		tmpl.GitToken = sql.NullString{String: *input.GitToken, Valid: true}
	}
	if input.Category != nil && *input.Category != "" {
		tmpl.Category = sql.NullString{String: *input.Category, Valid: true}
	}
	if input.Tags != nil {
		tagsJSON, jsonErr := json.Marshal(*input.Tags)
		if jsonErr != nil {
			return nil, fmt.Errorf("marshaling tags for update: %w", jsonErr)
		}
		tmpl.Tags = sql.NullString{String: string(tagsJSON), Valid: true}
	}
	if input.IsPublic != nil {
		tmpl.IsPublic = *input.IsPublic
	}

	if err := s.repo.Update(ctx, tmpl); err != nil {
		return nil, fmt.Errorf("updating git template: %w", err)
	}

	return tmpl, nil
}

// Delete soft-deletes a git template identified by UUID.
func (s *GitTemplateService) Delete(ctx context.Context, tmplUUID string) error {
	tmpl, err := s.FindByUUID(ctx, tmplUUID)
	if err != nil {
		return fmt.Errorf("finding git template for deletion: %w", err)
	}

	if err := s.repo.SoftDelete(ctx, tmpl.ID); err != nil {
		return fmt.Errorf("soft deleting git template: %w", err)
	}

	return nil
}

// EnqueueSync verifies the template exists and enqueues a sync job.
func (s *GitTemplateService) EnqueueSync(ctx context.Context, tmplUUID string) error {
	if _, err := s.FindByUUID(ctx, tmplUUID); err != nil {
		return fmt.Errorf("finding git template for sync: %w", err)
	}

	if s.inserter == nil {
		return errors.New("job queue not available")
	}

	if _, err := s.inserter.Insert(ctx, queue.SyncGitTemplatesArgs{}, nil); err != nil {
		return fmt.Errorf("enqueuing git template sync: %w", err)
	}

	return nil
}

// ValidateRepositoryURL checks that a repository URL is safe to access.
func (s *GitTemplateService) ValidateRepositoryURL(rawURL string) error {
	if err := security.ValidateExternalURL(rawURL, true); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidURL, err.Error())
	}
	return nil
}
