// Package git provides a client for cloning and extracting files from git
// template repositories. It shells out to the git CLI and includes SSRF
// protection, credential handling, and binary file detection.
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
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Default size limits for file extraction.
const (
	DefaultMaxFileSize  int64 = 1 * 1024 * 1024  // 1 MB per file
	DefaultMaxTotalSize int64 = 10 * 1024 * 1024 // 10 MB total
	binaryProbeSize           = 8 * 1024         // 8 KB for binary detection
)

// variablePattern matches {{variable}} placeholders in template files.
var variablePattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

// Client handles git operations for template repositories.
type Client struct {
	tempDir string
	logger  *slog.Logger
}

// NewClient creates a Client that uses tempDir as the base directory for cloning repos.
func NewClient(tempDir string, logger *slog.Logger) *Client {
	return &Client{
		tempDir: tempDir,
		logger:  logger,
	}
}

// Clone clones a repository into a subdirectory of tempDir.
// It uses a shallow single-branch clone for efficiency. When params.Token is
// set, a temporary GIT_ASKPASS script provides credentials.
// Returns the path to the cloned directory.
func (c *Client) Clone(ctx context.Context, params CloneParams) (string, error) {
	if err := validateBranch(params.Branch); err != nil {
		return "", err
	}

	dest := params.Dest
	if dest == "" {
		dest = filepath.Join(c.tempDir, filepath.Base(params.URL))
	}

	args := []string{
		"clone",
		"--depth", "1",
		"--single-branch",
		"--branch", params.Branch,
		params.URL,
		dest,
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = os.Environ()

	if params.Token != "" {
		scriptPath, cleanup, err := writeCredentialScript(params.Token)
		if err != nil {
			return "", fmt.Errorf("setting up git credentials: %w", err)
		}
		defer cleanup()

		// Prevent git from prompting interactively.
		cmd.Env = append(cmd.Env, "GIT_ASKPASS="+scriptPath, "GIT_TERMINAL_PROMPT=0")
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	c.logger.Info("cloning repository",
		"url", params.URL,
		"branch", params.Branch,
		"dest", dest,
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone failed: %s", SanitizeGitError(stderr.String()))
	}

	return dest, nil
}

// Pull fetches and fast-forward merges the latest changes in an existing clone.
func (c *Client) Pull(ctx context.Context, repoDir, token string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "pull", "--ff-only")
	cmd.Env = os.Environ()

	if token != "" {
		scriptPath, cleanup, err := writeCredentialScript(token)
		if err != nil {
			return fmt.Errorf("setting up git credentials for pull: %w", err)
		}
		defer cleanup()

		cmd.Env = append(cmd.Env, "GIT_ASKPASS="+scriptPath, "GIT_TERMINAL_PROMPT=0")
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	c.logger.Info("pulling repository", "dir", repoDir)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %s", SanitizeGitError(stderr.String()))
	}

	return nil
}

// ExtractFiles walks a cloned repository directory and returns all non-binary,
// non-symlink files. The .git directory is skipped. Files exceeding maxFileSize
// are skipped, and extraction stops if maxTotalSize is exceeded.
func (c *Client) ExtractFiles(repoDir string, maxFileSize, maxTotalSize int64) ([]TemplateFile, error) {
	if maxFileSize <= 0 {
		maxFileSize = DefaultMaxFileSize
	}
	if maxTotalSize <= 0 {
		maxTotalSize = DefaultMaxTotalSize
	}

	var files []TemplateFile
	var totalSize int64

	err := filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the .git directory entirely.
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
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
			return filepath.SkipAll
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", path, err)
		}

		// Skip binary files (null byte in first 8KB).
		if isBinary(data) {
			c.logger.Debug("skipping binary file", "path", path)
			return nil
		}

		relPath, err := filepath.Rel(repoDir, path)
		if err != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, err)
		}
		relPath = filepath.ToSlash(relPath)

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
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "HEAD")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse HEAD failed: %s", SanitizeGitError(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
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
	matches := variablePattern.FindAllStringSubmatch(content, -1)
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
