package git

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/c-premus/documcp/internal/model"
)

// --- test helpers ---

// makeTestGitRepo initializes a local git repo with one commit so it can be
// used as a clone source in tests.
func makeTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "git", "init", "-b", "main")
	runGit(t, dir, "git", "config", "user.email", "test@example.com")
	runGit(t, dir, "git", "config", "user.name", "Test")
	runGit(t, dir, "git", "config", "commit.gpgsign", "false")
	writeTestFile(t, filepath.Join(dir, "README.md"), "# Test Repo\n")
	writeTestFile(t, filepath.Join(dir, "hello.txt"), "Hello {{name}}!\n")
	runGit(t, dir, "git", "add", ".")
	runGit(t, dir, "git", "commit", "-m", "initial commit")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec // test helper with controlled inputs
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v failed: %v\n%s", args, err, out)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeTestFile(%s): %v", path, err)
	}
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	return NewClient(t.TempDir(), DefaultMaxFileSize, DefaultMaxTotalSize, slog.Default())
}

// --- Clone ---

func TestClone_EmptyBranch(t *testing.T) {
	c := newTestClient(t)
	_, err := c.Clone(context.Background(), CloneParams{
		URL:    "https://example.com/repo",
		Branch: "",
	})
	if err == nil {
		t.Fatal("expected error for empty branch")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClone_DashBranch(t *testing.T) {
	c := newTestClient(t)
	_, err := c.Clone(context.Background(), CloneParams{
		URL:    "https://example.com/repo",
		Branch: "--delete",
	})
	if err == nil {
		t.Fatal("expected error for dash-prefixed branch")
	}
	if !strings.Contains(err.Error(), "dash") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClone_Success(t *testing.T) {
	src := makeTestGitRepo(t)
	c := newTestClient(t)
	dest := filepath.Join(c.tempDir, "clone")

	dir, err := c.Clone(context.Background(), CloneParams{
		URL:    src,
		Branch: "main",
		Dest:   dest,
	})
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	if dir != dest {
		t.Errorf("expected dest %q, got %q", dest, dir)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".git")); statErr != nil {
		t.Errorf(".git not found in clone: %v", statErr)
	}
}

func TestClone_DefaultDest(t *testing.T) {
	src := makeTestGitRepo(t)
	c := newTestClient(t)

	dir, err := c.Clone(context.Background(), CloneParams{
		URL:    src,
		Branch: "main",
		// No Dest — should use filepath.Base(URL)
	})
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	want := filepath.Join(c.tempDir, filepath.Base(src))
	if dir != want {
		t.Errorf("expected default dest %q, got %q", want, dir)
	}
}

func TestClone_GitFails(t *testing.T) {
	c := newTestClient(t)
	_, err := c.Clone(context.Background(), CloneParams{
		URL:    "/nonexistent/path/to/repo",
		Branch: "main",
		Dest:   filepath.Join(c.tempDir, "dest"),
	})
	if err == nil {
		t.Fatal("expected error for nonexistent repo")
	}
	if !strings.Contains(err.Error(), "git clone failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Pull ---

func TestPull_Success(t *testing.T) {
	src := makeTestGitRepo(t)
	c := newTestClient(t)
	dest := filepath.Join(c.tempDir, "clone")

	if _, err := c.Clone(context.Background(), CloneParams{URL: src, Branch: "main", Dest: dest}); err != nil {
		t.Fatalf("setup Clone failed: %v", err)
	}

	if err := c.Pull(context.Background(), dest, ""); err != nil {
		t.Fatalf("Pull failed: %v", err)
	}
}

func TestPull_GitFails(t *testing.T) {
	c := newTestClient(t)
	err := c.Pull(context.Background(), "/nonexistent/path", "")
	if err == nil {
		t.Fatal("expected error for nonexistent dir")
	}
	if !strings.Contains(err.Error(), "git pull failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- LatestCommitSHA ---

func TestLatestCommitSHA_Success(t *testing.T) {
	src := makeTestGitRepo(t)
	c := newTestClient(t)
	dest := filepath.Join(c.tempDir, "clone")

	if _, err := c.Clone(context.Background(), CloneParams{URL: src, Branch: "main", Dest: dest}); err != nil {
		t.Fatalf("setup Clone failed: %v", err)
	}

	sha, err := c.LatestCommitSHA(context.Background(), dest)
	if err != nil {
		t.Fatalf("LatestCommitSHA failed: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("expected 40-char SHA, got %q (len=%d)", sha, len(sha))
	}
}

func TestLatestCommitSHA_NotARepo(t *testing.T) {
	c := newTestClient(t)
	_, err := c.LatestCommitSHA(context.Background(), "/nonexistent/path")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "git rev-parse HEAD failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- ExtractFiles ---

func TestExtractFiles_BasicFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "README.md"), "# Hello {{name}}\n")
	writeTestFile(t, filepath.Join(dir, "main.go"), "package main\n")

	c := newTestClient(t)
	files, err := c.ExtractFiles(dir, 0, 0)
	if err != nil {
		t.Fatalf("ExtractFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestExtractFiles_SkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(gitDir, "config"), "[core]\n")
	writeTestFile(t, filepath.Join(dir, "README.md"), "# Test\n")

	c := newTestClient(t)
	files, err := c.ExtractFiles(dir, 0, 0)
	if err != nil {
		t.Fatalf("ExtractFiles failed: %v", err)
	}
	if len(files) != 1 || files[0].Filename != "README.md" {
		t.Errorf("expected only README.md, got %v", files)
	}
}

func TestExtractFiles_SkipsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "big.txt"), strings.Repeat("x", 10))
	writeTestFile(t, filepath.Join(dir, "small.txt"), "ok")

	c := newTestClient(t)
	files, err := c.ExtractFiles(dir, 5, 0) // maxFileSize = 5 bytes
	if err != nil {
		t.Fatalf("ExtractFiles failed: %v", err)
	}
	if len(files) != 1 || files[0].Filename != "small.txt" {
		t.Errorf("expected only small.txt, got %v", files)
	}
}

func TestExtractFiles_StopsAtTotalSize(t *testing.T) {
	dir := t.TempDir()
	// 3 files × 5 bytes each; total limit = 8 → first file fits, second pushes over limit
	writeTestFile(t, filepath.Join(dir, "a.txt"), strings.Repeat("a", 5))
	writeTestFile(t, filepath.Join(dir, "b.txt"), strings.Repeat("b", 5))
	writeTestFile(t, filepath.Join(dir, "c.txt"), strings.Repeat("c", 5))

	c := newTestClient(t)
	files, err := c.ExtractFiles(dir, 0, 8)
	if err != nil {
		t.Fatalf("ExtractFiles failed: %v", err)
	}
	if len(files) >= 3 {
		t.Errorf("expected fewer than 3 files (total size limit), got %d", len(files))
	}
}

func TestExtractFiles_SkipsBinaryFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "binary.bin"), "hello\x00world")
	writeTestFile(t, filepath.Join(dir, "text.txt"), "plain text")

	c := newTestClient(t)
	files, err := c.ExtractFiles(dir, 0, 0)
	if err != nil {
		t.Fatalf("ExtractFiles failed: %v", err)
	}
	if len(files) != 1 || files[0].Filename != "text.txt" {
		t.Errorf("expected only text.txt, got %v", files)
	}
}

func TestExtractFiles_ExtractsVariables(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "tmpl.txt"), "Hello {{name}}, your {{role}} is ready")

	c := newTestClient(t)
	files, err := c.ExtractFiles(dir, 0, 0)
	if err != nil {
		t.Fatalf("ExtractFiles failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if len(files[0].Variables) != 2 {
		t.Errorf("expected 2 variables, got %v", files[0].Variables)
	}
}

func TestExtractFiles_ContentHash(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "file.txt"), "content")

	c := newTestClient(t)
	files, err := c.ExtractFiles(dir, 0, 0)
	if err != nil {
		t.Fatalf("ExtractFiles failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file")
	}
	if len(files[0].ContentHash) != 64 {
		t.Errorf("expected 64-char hex SHA-256, got %q", files[0].ContentHash)
	}
}

func TestExtractFiles_IsEssentialFlagSet(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "README.md"), "# Docs\n")
	writeTestFile(t, filepath.Join(dir, "main.go"), "package main\n")

	c := newTestClient(t)
	files, err := c.ExtractFiles(dir, 0, 0)
	if err != nil {
		t.Fatalf("ExtractFiles failed: %v", err)
	}

	for _, f := range files {
		switch f.Filename {
		case "README.md":
			if !f.IsEssential {
				t.Errorf("README.md should be essential")
			}
		case "main.go":
			if f.IsEssential {
				t.Errorf("main.go should not be essential")
			}
		}
	}
}

// --- Sync mock types ---

type mockTemplateRepo struct {
	updateSyncStatusFn    func(ctx context.Context, templateID int64, status model.GitTemplateStatus, commitSHA string, fileCount int, totalSize int64, errMsg string) error
	replaceFilesFn        func(ctx context.Context, templateID int64, files []TemplateFile) error
	updateSearchContentFn func(ctx context.Context, templateID int64, readmeContent, filePaths string) error
}

func (m *mockTemplateRepo) UpdateSyncStatus(ctx context.Context, templateID int64, status model.GitTemplateStatus, commitSHA string, fileCount int, totalSize int64, errMsg string) error {
	if m.updateSyncStatusFn != nil {
		return m.updateSyncStatusFn(ctx, templateID, status, commitSHA, fileCount, totalSize, errMsg)
	}
	return nil
}

func (m *mockTemplateRepo) ReplaceFiles(ctx context.Context, templateID int64, files []TemplateFile) error {
	if m.replaceFilesFn != nil {
		return m.replaceFilesFn(ctx, templateID, files)
	}
	return nil
}

func (m *mockTemplateRepo) UpdateSearchContent(ctx context.Context, templateID int64, readmeContent, filePaths string) error {
	if m.updateSearchContentFn != nil {
		return m.updateSearchContentFn(ctx, templateID, readmeContent, filePaths)
	}
	return nil
}

// syncURL is a real public IP that passes ValidateRepositoryURL without DNS
// resolution. Git operations in Sync tests use local origins from pre-cloned
// repos — this URL is only validated, never fetched.
const syncURL = "https://8.8.8.8/repo.git"

// preclone clones src to <client.tempDir>/<slug> so that Sync sees an existing
// .git dir and takes the pull path.
func preclone(t *testing.T, c *Client, src, slug string) string {
	t.Helper()
	dest := filepath.Join(c.tempDir, slug)
	if _, err := c.Clone(context.Background(), CloneParams{URL: src, Branch: "main", Dest: dest}); err != nil {
		t.Fatalf("preclone failed: %v", err)
	}
	return dest
}

// --- Sync ---

func TestSync_InvalidURL(t *testing.T) {
	c := newTestClient(t)
	err := Sync(context.Background(), SyncParams{
		Template: SyncTemplate{
			ID:            1,
			UUID:          "u",
			RepositoryURL: "http://example.com/repo", // http not https
			Branch:        "main",
		},
		Client: c,
		Repo:   &mockTemplateRepo{},
		Logger: slog.Default(),
	})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "validating repository URL") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSync_CloneError_EmptyBranch(t *testing.T) {
	// Empty branch causes Clone to fail immediately (validateBranch) without
	// network I/O, even though the URL passes ValidateRepositoryURL.
	c := newTestClient(t)
	err := Sync(context.Background(), SyncParams{
		Template: SyncTemplate{
			ID:            1,
			UUID:          "u",
			Slug:          "no-slug",
			RepositoryURL: syncURL,
			Branch:        "", // triggers validateBranch error inside Clone
		},
		Client: c,
		Repo:   &mockTemplateRepo{},
		Logger: slog.Default(),
	})
	if err == nil {
		t.Fatal("expected error from Clone with empty branch")
	}
	if !strings.Contains(err.Error(), "cloning template repo") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSync_PullFailure(t *testing.T) {
	// Create a .git directory that is not a valid git repo so Pull fails.
	c := newTestClient(t)
	slug := "bad-repo"
	badGit := filepath.Join(c.tempDir, slug, ".git")
	if err := os.MkdirAll(badGit, 0o750); err != nil {
		t.Fatal(err)
	}

	err := Sync(context.Background(), SyncParams{
		Template: SyncTemplate{
			ID:            1,
			UUID:          "u",
			Slug:          slug,
			RepositoryURL: syncURL,
			Branch:        "main",
		},
		Client: c,
		Repo:   &mockTemplateRepo{},
		Logger: slog.Default(),
	})
	if err == nil {
		t.Fatal("expected error from Pull on invalid git repo")
	}
	if !strings.Contains(err.Error(), "pulling template repo") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSync_AlreadyUpToDate(t *testing.T) {
	src := makeTestGitRepo(t)
	c := newTestClient(t)
	slug := "upto-date"
	dest := preclone(t, c, src, slug)

	sha, err := c.LatestCommitSHA(context.Background(), dest)
	if err != nil {
		t.Fatalf("LatestCommitSHA: %v", err)
	}

	err = Sync(context.Background(), SyncParams{
		Template: SyncTemplate{
			ID:            1,
			UUID:          "u",
			Slug:          slug,
			RepositoryURL: syncURL,
			Branch:        "main",
			LastCommitSHA: sha, // already synced to this commit
		},
		Client: c,
		Repo:   &mockTemplateRepo{},
		Logger: slog.Default(),
	})
	if err != nil {
		t.Fatalf("expected nil (already up to date), got: %v", err)
	}
}

func TestSync_UpdateSyncStatusFailure(t *testing.T) {
	src := makeTestGitRepo(t)
	c := newTestClient(t)
	slug := "status-fail"
	preclone(t, c, src, slug)

	repo := &mockTemplateRepo{
		updateSyncStatusFn: func(_ context.Context, _ int64, status model.GitTemplateStatus, _ string, _ int, _ int64, _ string) error {
			if status == model.GitTemplateStatusSynced {
				return errors.New("db write failed")
			}
			return nil
		},
	}

	err := Sync(context.Background(), SyncParams{
		Template: SyncTemplate{
			ID:            1,
			UUID:          "u",
			Slug:          slug,
			RepositoryURL: syncURL,
			Branch:        "main",
		},
		Client: c,
		Repo:   repo,
		Logger: slog.Default(),
	})
	if err == nil {
		t.Fatal("expected error from UpdateSyncStatus")
	}
	if !strings.Contains(err.Error(), "updating sync status") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSync_ReplaceFilesError(t *testing.T) {
	src := makeTestGitRepo(t)
	c := newTestClient(t)
	slug := "replace-fail"
	preclone(t, c, src, slug)

	repo := &mockTemplateRepo{
		replaceFilesFn: func(_ context.Context, _ int64, _ []TemplateFile) error {
			return errors.New("disk full")
		},
	}

	err := Sync(context.Background(), SyncParams{
		Template: SyncTemplate{
			ID:            1,
			UUID:          "u",
			Slug:          slug,
			RepositoryURL: syncURL,
			Branch:        "main",
		},
		Client: c,
		Repo:   repo,
		Logger: slog.Default(),
	})
	if err == nil {
		t.Fatal("expected error from ReplaceFiles")
	}
	if !strings.Contains(err.Error(), "replacing template files") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSync_NilLogger(t *testing.T) {
	src := makeTestGitRepo(t)
	c := newTestClient(t)
	slug := "nil-logger"
	preclone(t, c, src, slug)

	err := Sync(context.Background(), SyncParams{
		Template: SyncTemplate{
			ID:            1,
			UUID:          "u",
			Slug:          slug,
			RepositoryURL: syncURL,
			Branch:        "main",
		},
		Client: c,
		Repo:   &mockTemplateRepo{},
		Logger: nil, // falls back to slog.Default() — must not panic
	})
	if err != nil {
		t.Fatalf("Sync with nil logger failed: %v", err)
	}
}
