// Package git provides a client for cloning and extracting files from git
// template repositories. It uses go-git for pure-Go git operations and includes
// SSRF protection, credential handling, and binary file detection.
package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Default size limits for file extraction. Only text files are extracted from
// template repositories — binary files (PDFs, images, compiled artifacts) are
// silently skipped. Binary detection checks for null bytes in the first 8 KB.
const (
	DefaultMaxFileSize  int64 = 1 * 1024 * 1024  // 1 MB per file
	DefaultMaxTotalSize int64 = 10 * 1024 * 1024 // 10 MB total
	binaryProbeSize           = 8 * 1024         // 8 KB for binary detection
)

// VariablePattern matches {{variable}} placeholders in template files.
var VariablePattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

// Client handles git operations for template repositories.
type Client struct {
	tempDir      string
	maxFileSize  int64
	maxTotalSize int64
	logger       *slog.Logger
}

// NewClient creates a Client that uses tempDir as the base directory for cloning repos.
// The maxFileSize and maxTotalSize parameters set default limits for ExtractFiles;
// pass 0 to use DefaultMaxFileSize and DefaultMaxTotalSize respectively.
//
// The first NewClient call per process installs an SSRF-safe HTTP(S) transport
// into go-git's plumbing/transport/client registry (see installSafeHTTPTransport
// in safehttp.go). Subsequent calls are no-ops via sync.Once.
func NewClient(tempDir string, maxFileSize, maxTotalSize int64, logger *slog.Logger) *Client {
	installSafeHTTPTransport()

	if maxFileSize <= 0 {
		maxFileSize = DefaultMaxFileSize
	}
	if maxTotalSize <= 0 {
		maxTotalSize = DefaultMaxTotalSize
	}
	return &Client{
		tempDir:      tempDir,
		maxFileSize:  maxFileSize,
		maxTotalSize: maxTotalSize,
		logger:       logger,
	}
}

// sanitizeErr wraps an error's message through SanitizeGitError to redact
// URLs and tokens before the error is returned or logged.
func sanitizeErr(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(SanitizeGitError(err.Error()))
}

// tokenAuth returns an http.BasicAuth suitable for go-git when a PAT is
// provided. Returns nil when token is empty.
func tokenAuth(token string) *http.BasicAuth {
	if token == "" {
		return nil
	}
	return &http.BasicAuth{
		Username: "x-token-auth",
		Password: token,
	}
}

// Clone clones a repository into a subdirectory of tempDir.
// It uses a shallow single-branch clone for efficiency. When params.Token is
// set, HTTP basic auth is used to authenticate.
// Returns the path to the cloned directory.
func (c *Client) Clone(ctx context.Context, params CloneParams) (string, error) {
	ctx, span := otel.Tracer("documcp/git").Start(ctx, "git.clone",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("git.url", params.URL),
			attribute.String("git.branch", params.Branch),
		),
	)
	defer span.End()

	if err := validateBranch(params.Branch); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	dest := params.Dest
	if dest == "" {
		dest = filepath.Join(c.tempDir, filepath.Base(params.URL))
	}

	cloneOpts := &gogit.CloneOptions{
		URL:           params.URL,
		ReferenceName: plumbing.NewBranchReferenceName(params.Branch),
		SingleBranch:  true,
		Depth:         1,
	}

	if auth := tokenAuth(params.Token); auth != nil {
		cloneOpts.Auth = auth
	}

	c.logger.Info("cloning repository",
		"url", params.URL,
		"branch", params.Branch,
		"dest", dest,
	)

	_, err := gogit.PlainCloneContext(ctx, dest, false, cloneOpts)
	if err != nil {
		err = fmt.Errorf("git clone failed: %w", sanitizeErr(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	return dest, nil
}

// Pull fetches and fast-forward merges the latest changes in an existing clone.
func (c *Client) Pull(ctx context.Context, repoDir, token string) error {
	ctx, span := otel.Tracer("documcp/git").Start(ctx, "git.pull",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("git.dir", repoDir)),
	)
	defer span.End()

	repo, err := gogit.PlainOpen(repoDir)
	if err != nil {
		err = fmt.Errorf("git pull failed: %w", sanitizeErr(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		err = fmt.Errorf("git pull failed: %w", sanitizeErr(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	pullOpts := &gogit.PullOptions{}
	if auth := tokenAuth(token); auth != nil {
		pullOpts.Auth = auth
	}

	c.logger.Info("pulling repository", "dir", repoDir)

	if err := wt.PullContext(ctx, pullOpts); err != nil {
		if errors.Is(err, gogit.NoErrAlreadyUpToDate) {
			span.SetAttributes(attribute.Bool("git.already_up_to_date", true))
			return nil
		}
		err = fmt.Errorf("git pull failed: %w", sanitizeErr(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

// ExtractFiles walks a cloned repository directory and returns all non-binary,
// non-symlink text files. The .git directory is skipped. Binary files (detected
// by null bytes in the first 8 KB) are silently excluded — this means PDFs,
// images, and other non-text content will not appear in synced templates.
// Individual files exceeding maxFileSize (default 1 MB) are skipped, and
// extraction stops if cumulative size exceeds maxTotalSize (default 10 MB).
func (c *Client) ExtractFiles(repoDir string, maxFileSize, maxTotalSize int64) ([]TemplateFile, error) {
	if maxFileSize <= 0 {
		maxFileSize = c.maxFileSize
	}
	if maxTotalSize <= 0 {
		maxTotalSize = c.maxTotalSize
	}

	// Open a root-scoped handle to prevent symlink traversal outside the
	// cloned repository (eliminates gosec G122 / G304 TOCTOU risk).
	root, err := os.OpenRoot(repoDir)
	if err != nil {
		return nil, fmt.Errorf("opening repository root: %w", err)
	}
	defer func() { _ = root.Close() }()

	var files []TemplateFile
	var totalSize int64

	err = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the .git directory entirely.
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		// Skip symlinks.
		if d.Type()&fs.ModeSymlink != 0 {
			c.logger.Debug("skipping symlink", "path", path)
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("getting file info for %s: %w", path, err)
		}

		// Skip files exceeding per-file limit.
		if info.Size() > maxFileSize {
			c.logger.Debug("skipping oversized file",
				"path", path,
				"size", info.Size(),
				"max", maxFileSize,
			)
			return nil
		}

		// Stop if total size limit would be exceeded.
		if totalSize+info.Size() > maxTotalSize {
			c.logger.Warn("total size limit reached, stopping extraction",
				"total", totalSize,
				"max", maxTotalSize,
			)
			return fs.SkipAll
		}

		data, err := root.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", path, err)
		}

		// Skip binary files (null byte in first 8KB).
		if isBinary(data) {
			c.logger.Debug("skipping binary file", "path", path)
			return nil
		}

		// path is already relative to root (fs.WalkDir uses "." as start).
		relPath := filepath.ToSlash(path)

		hash := sha256.Sum256(data)
		content := string(data)

		tf := TemplateFile{
			Path:        relPath,
			Filename:    filepath.Base(relPath),
			Extension:   strings.TrimPrefix(filepath.Ext(relPath), "."),
			Content:     content,
			SizeBytes:   info.Size(),
			ContentHash: hex.EncodeToString(hash[:]),
			IsEssential: IsEssentialFile(relPath),
			Variables:   extractVariables(content),
		}

		files = append(files, tf)
		totalSize += info.Size()

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking repository directory: %w", err)
	}

	c.logger.Info("extracted files",
		"count", len(files),
		"total_bytes", totalSize,
	)

	return files, nil
}

// LatestCommitSHA returns the full SHA of the HEAD commit in the given repository.
func (c *Client) LatestCommitSHA(ctx context.Context, repoDir string) (string, error) {
	repo, err := gogit.PlainOpen(repoDir)
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD failed: %w", sanitizeErr(err))
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD failed: %w", sanitizeErr(err))
	}

	return head.Hash().String(), nil
}

// validateBranch rejects branch names that could be interpreted as git flags.
func validateBranch(branch string) error {
	if branch == "" {
		return errors.New("branch name must not be empty")
	}
	if strings.HasPrefix(branch, "-") {
		return errors.New("branch name must not start with a dash")
	}
	return nil
}

// isBinary checks whether data looks like a binary file by searching for null
// bytes in the first binaryProbeSize bytes.
func isBinary(data []byte) bool {
	probe := data
	if len(probe) > binaryProbeSize {
		probe = probe[:binaryProbeSize]
	}
	return bytes.Contains(probe, []byte{0})
}

// extractVariables returns unique {{variable}} placeholder names found in content.
func extractVariables(content string) []string {
	matches := VariablePattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	var vars []string
	for _, m := range matches {
		name := m[1]
		if _, exists := seen[name]; !exists {
			seen[name] = struct{}{}
			vars = append(vars, name)
		}
	}
	return vars
}
