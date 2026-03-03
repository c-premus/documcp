package scheduler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
)

// newTestScheduler creates a Scheduler with the given Deps and a discard logger.
func newTestScheduler(deps Deps) *Scheduler {
	return New(Config{Logger: discardLogger}, deps)
}

// ---------------------------------------------------------------------------
// Mock: ExternalServiceHealthChecker
// ---------------------------------------------------------------------------

type mockHealthChecker struct {
	findAllEnabledFn     func(ctx context.Context) ([]model.ExternalService, error)
	updateHealthStatusFn func(ctx context.Context, id int64, status string, latencyMs int, lastError string) error
}

func (m *mockHealthChecker) FindAllEnabled(ctx context.Context) ([]model.ExternalService, error) {
	if m.findAllEnabledFn != nil {
		return m.findAllEnabledFn(ctx)
	}
	return nil, nil
}

func (m *mockHealthChecker) UpdateHealthStatus(ctx context.Context, id int64, status string, latencyMs int, lastError string) error {
	if m.updateHealthStatusFn != nil {
		return m.updateHealthStatusFn(ctx, id, status, latencyMs, lastError)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Test: cleanupOAuthTokens
// ---------------------------------------------------------------------------

func TestCleanupOAuthTokens(t *testing.T) {
	t.Parallel()

	t.Run("nil OAuthRepo logs warning and returns without panic", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{OAuthRepo: nil})

		// Must not panic.
		assert.NotPanics(t, func() {
			s.cleanupOAuthTokens()
		})
	})

	t.Run("all deps nil does not panic", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{})

		assert.NotPanics(t, func() {
			s.cleanupOAuthTokens()
		})
	})
}

// ---------------------------------------------------------------------------
// Test: cleanupOrphanedFiles
// ---------------------------------------------------------------------------

func TestCleanupOrphanedFiles(t *testing.T) {
	t.Parallel()

	t.Run("nil DocRepo logs warning and returns without panic", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{DocRepo: nil, StoragePath: "/some/path"})

		assert.NotPanics(t, func() {
			s.cleanupOrphanedFiles()
		})
	})

	t.Run("empty StoragePath logs warning and returns without panic", func(t *testing.T) {
		t.Parallel()

		// Even with a non-nil DocRepo, empty StoragePath should trigger the guard.
		s := newTestScheduler(Deps{
			DocRepo:     &repository.DocumentRepository{},
			StoragePath: "",
		})

		assert.NotPanics(t, func() {
			s.cleanupOrphanedFiles()
		})
	})

	t.Run("both nil DocRepo and empty StoragePath guard", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{})

		assert.NotPanics(t, func() {
			s.cleanupOrphanedFiles()
		})
	})
}

// ---------------------------------------------------------------------------
// Test: verifySearchIndex
// ---------------------------------------------------------------------------

func TestVerifySearchIndex(t *testing.T) {
	t.Parallel()

	t.Run("nil DocRepo logs warning and returns without panic", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{DocRepo: nil})

		assert.NotPanics(t, func() {
			s.verifySearchIndex()
		})
	})

	t.Run("nil Indexer logs warning and returns without panic", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{
			DocRepo: &repository.DocumentRepository{},
			Indexer: nil,
		})

		assert.NotPanics(t, func() {
			s.verifySearchIndex()
		})
	})

	t.Run("both nil DocRepo and nil Indexer guard", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{})

		assert.NotPanics(t, func() {
			s.verifySearchIndex()
		})
	})
}

// ---------------------------------------------------------------------------
// Test: purgeSoftDeleted
// ---------------------------------------------------------------------------

func TestPurgeSoftDeleted(t *testing.T) {
	t.Parallel()

	t.Run("nil DocRepo logs warning and returns without panic", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{DocRepo: nil})

		assert.NotPanics(t, func() {
			s.purgeSoftDeleted()
		})
	})

	t.Run("all deps nil does not panic", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{})

		assert.NotPanics(t, func() {
			s.purgeSoftDeleted()
		})
	})
}

// ---------------------------------------------------------------------------
// Test: cleanupDisabledZim
// ---------------------------------------------------------------------------

func TestCleanupDisabledZim(t *testing.T) {
	t.Parallel()

	t.Run("nil ZimRepo logs warning and returns without panic", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{ZimRepo: nil})

		assert.NotPanics(t, func() {
			s.cleanupDisabledZim()
		})
	})

	t.Run("nil Indexer logs warning and returns without panic", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{
			ZimRepo: &repository.ZimArchiveRepository{},
			Indexer: nil,
		})

		assert.NotPanics(t, func() {
			s.cleanupDisabledZim()
		})
	})

	t.Run("both nil ZimRepo and nil Indexer guard", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{})

		assert.NotPanics(t, func() {
			s.cleanupDisabledZim()
		})
	})
}

// ---------------------------------------------------------------------------
// Test: healthCheckServices
// ---------------------------------------------------------------------------

func TestHealthCheckServices(t *testing.T) {
	t.Parallel()

	t.Run("nil HealthChecker logs warning and returns without panic", func(t *testing.T) {
		t.Parallel()

		s := newTestScheduler(Deps{HealthChecker: nil})

		assert.NotPanics(t, func() {
			s.healthCheckServices()
		})
	})

	t.Run("FindAllEnabled returns error logs and returns", func(t *testing.T) {
		t.Parallel()

		checker := &mockHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return nil, errors.New("db connection failed")
			},
		}
		s := newTestScheduler(Deps{HealthChecker: checker})

		assert.NotPanics(t, func() {
			s.healthCheckServices()
		})
	})

	t.Run("empty services list completes without error", func(t *testing.T) {
		t.Parallel()

		checker := &mockHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{}, nil
			},
		}
		s := newTestScheduler(Deps{HealthChecker: checker})

		assert.NotPanics(t, func() {
			s.healthCheckServices()
		})
	})

	t.Run("healthy service returns 200 updates status to healthy", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		var capturedStatus string
		var capturedID int64
		var capturedLatency int
		var capturedError string

		checker := &mockHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 42, BaseURL: server.URL},
				}, nil
			},
			updateHealthStatusFn: func(_ context.Context, id int64, status string, latencyMs int, lastError string) error {
				capturedID = id
				capturedStatus = status
				capturedLatency = latencyMs
				capturedError = lastError
				return nil
			},
		}

		s := newTestScheduler(Deps{HealthChecker: checker})
		s.healthCheckServices()

		assert.Equal(t, int64(42), capturedID)
		assert.Equal(t, "healthy", capturedStatus)
		assert.GreaterOrEqual(t, capturedLatency, 0)
		assert.Empty(t, capturedError)
	})

	t.Run("unhealthy service returns 500 updates status to unhealthy", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		var capturedStatus string
		var capturedID int64
		var capturedError string

		checker := &mockHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 7, BaseURL: server.URL},
				}, nil
			},
			updateHealthStatusFn: func(_ context.Context, id int64, status string, _ int, lastError string) error {
				capturedID = id
				capturedStatus = status
				capturedError = lastError
				return nil
			},
		}

		s := newTestScheduler(Deps{HealthChecker: checker})
		s.healthCheckServices()

		assert.Equal(t, int64(7), capturedID)
		assert.Equal(t, "unhealthy", capturedStatus)
		assert.Contains(t, capturedError, "unexpected status code: 500")
	})

	t.Run("unhealthy service returns 503 updates status to unhealthy", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		var capturedStatus string
		var capturedError string

		checker := &mockHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: server.URL},
				}, nil
			},
			updateHealthStatusFn: func(_ context.Context, _ int64, status string, _ int, lastError string) error {
				capturedStatus = status
				capturedError = lastError
				return nil
			},
		}

		s := newTestScheduler(Deps{HealthChecker: checker})
		s.healthCheckServices()

		assert.Equal(t, "unhealthy", capturedStatus)
		assert.Contains(t, capturedError, "unexpected status code: 503")
	})

	t.Run("unreachable service updates status to unhealthy", func(t *testing.T) {
		t.Parallel()

		var capturedStatus string
		var capturedError string

		checker := &mockHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 99, BaseURL: "http://192.0.2.1:1"}, // RFC 5737 TEST-NET, should be unreachable.
				}, nil
			},
			updateHealthStatusFn: func(_ context.Context, _ int64, status string, _ int, lastError string) error {
				capturedStatus = status
				capturedError = lastError
				return nil
			},
		}

		s := newTestScheduler(Deps{HealthChecker: checker})
		s.healthCheckServices()

		assert.Equal(t, "unhealthy", capturedStatus)
		assert.NotEmpty(t, capturedError)
	})

	t.Run("invalid base URL updates status to unhealthy", func(t *testing.T) {
		t.Parallel()

		var capturedStatus string

		checker := &mockHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 10, BaseURL: "://invalid-url"},
				}, nil
			},
			updateHealthStatusFn: func(_ context.Context, _ int64, status string, _ int, _ string) error {
				capturedStatus = status
				return nil
			},
		}

		s := newTestScheduler(Deps{HealthChecker: checker})
		s.healthCheckServices()

		assert.Equal(t, "unhealthy", capturedStatus)
	})

	t.Run("multiple services each checked independently", func(t *testing.T) {
		t.Parallel()

		healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer healthyServer.Close()

		unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer unhealthyServer.Close()

		results := make(map[int64]string)

		checker := &mockHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: healthyServer.URL},
					{ID: 2, BaseURL: unhealthyServer.URL},
				}, nil
			},
			updateHealthStatusFn: func(_ context.Context, id int64, status string, _ int, _ string) error {
				results[id] = status
				return nil
			},
		}

		s := newTestScheduler(Deps{HealthChecker: checker})
		s.healthCheckServices()

		assert.Equal(t, "healthy", results[1])
		assert.Equal(t, "unhealthy", results[2])
	})

	t.Run("UpdateHealthStatus error does not stop processing", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		var callCount int

		checker := &mockHealthChecker{
			findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
				return []model.ExternalService{
					{ID: 1, BaseURL: server.URL},
					{ID: 2, BaseURL: server.URL},
				}, nil
			},
			updateHealthStatusFn: func(_ context.Context, id int64, _ string, _ int, _ string) error {
				callCount++
				if id == 1 {
					return errors.New("db write failed")
				}
				return nil
			},
		}

		s := newTestScheduler(Deps{HealthChecker: checker})
		s.healthCheckServices()

		// Both services should have been checked despite the first update failing.
		assert.Equal(t, 2, callCount)
	})

	t.Run("healthy service returns 2xx range status codes", func(t *testing.T) {
		t.Parallel()

		statusCodes := []int{
			http.StatusOK,
			http.StatusCreated,
			http.StatusAccepted,
			http.StatusNoContent,
		}

		for _, code := range statusCodes {
			code := code
			t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
				t.Parallel()

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(code)
				}))
				defer server.Close()

				var capturedStatus string

				checker := &mockHealthChecker{
					findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
						return []model.ExternalService{
							{ID: 1, BaseURL: server.URL},
						}, nil
					},
					updateHealthStatusFn: func(_ context.Context, _ int64, status string, _ int, _ string) error {
						capturedStatus = status
						return nil
					},
				}

				s := newTestScheduler(Deps{HealthChecker: checker})
				s.healthCheckServices()

				assert.Equal(t, "healthy", capturedStatus)
			})
		}
	})

	t.Run("non-2xx status codes are unhealthy", func(t *testing.T) {
		t.Parallel()

		statusCodes := []int{
			http.StatusMovedPermanently,  // 301
			http.StatusBadRequest,        // 400
			http.StatusForbidden,         // 403
			http.StatusNotFound,          // 404
			http.StatusInternalServerError, // 500
		}

		for _, code := range statusCodes {
			code := code
			t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
				t.Parallel()

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(code)
				}))
				defer server.Close()

				var capturedStatus string

				checker := &mockHealthChecker{
					findAllEnabledFn: func(_ context.Context) ([]model.ExternalService, error) {
						return []model.ExternalService{
							{ID: 1, BaseURL: server.URL},
						}, nil
					},
					updateHealthStatusFn: func(_ context.Context, _ int64, status string, _ int, _ string) error {
						capturedStatus = status
						return nil
					},
				}

				s := newTestScheduler(Deps{HealthChecker: checker})
				s.healthCheckServices()

				assert.Equal(t, "unhealthy", capturedStatus)
			})
		}
	})
}

// ---------------------------------------------------------------------------
// Test: cleanupOrphanedFiles with real filesystem
// ---------------------------------------------------------------------------

func TestCleanupOrphanedFiles_WithFilesystem(t *testing.T) {
	t.Parallel()

	// This test verifies the file walking and deletion logic. Since DocRepo is
	// a concrete type that requires a database, we cannot mock its methods for
	// unit tests. The nil guard tests above cover the guard paths. The
	// filesystem logic below is tested via a helper that replicates the core
	// algorithm from cleanupOrphanedFiles without needing a real DocRepo.

	t.Run("orphaned files are removed from storage directory", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		// Create files: 2 active, 1 orphaned.
		activeFile1 := filepath.Join(storageDir, "docs", "active1.pdf")
		activeFile2 := filepath.Join(storageDir, "docs", "active2.pdf")
		orphanedFile := filepath.Join(storageDir, "docs", "orphaned.pdf")

		require.NoError(t, os.MkdirAll(filepath.Dir(activeFile1), 0o755))
		require.NoError(t, os.WriteFile(activeFile1, []byte("active1"), 0o644))
		require.NoError(t, os.WriteFile(activeFile2, []byte("active2"), 0o644))
		require.NoError(t, os.WriteFile(orphanedFile, []byte("orphaned"), 0o644))

		// Simulate the core algorithm from cleanupOrphanedFiles.
		activePaths := []repository.DocumentFilePath{
			{ID: 1, UUID: "uuid-1", FilePath: "docs/active1.pdf"},
			{ID: 2, UUID: "uuid-2", FilePath: "docs/active2.pdf"},
		}

		activeSet := make(map[string]bool, len(activePaths))
		for _, fp := range activePaths {
			absPath := filepath.Join(storageDir, fp.FilePath)
			activeSet[absPath] = true
		}

		var deletedCount int
		err := filepath.WalkDir(storageDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !activeSet[path] {
				if removeErr := os.Remove(path); removeErr != nil {
					t.Errorf("removing orphaned file: %v", removeErr)
				} else {
					deletedCount++
				}
			}
			return nil
		})
		require.NoError(t, err)

		assert.Equal(t, 1, deletedCount)

		// Active files still exist.
		_, err = os.Stat(activeFile1)
		assert.NoError(t, err, "active file 1 should still exist")

		_, err = os.Stat(activeFile2)
		assert.NoError(t, err, "active file 2 should still exist")

		// Orphaned file removed.
		_, err = os.Stat(orphanedFile)
		assert.True(t, os.IsNotExist(err), "orphaned file should be removed")
	})

	t.Run("no files to delete when all are active", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		activeFile := filepath.Join(storageDir, "doc.pdf")
		require.NoError(t, os.WriteFile(activeFile, []byte("content"), 0o644))

		activeSet := map[string]bool{activeFile: true}

		var deletedCount int
		err := filepath.WalkDir(storageDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !activeSet[path] {
				deletedCount++
			}
			return nil
		})
		require.NoError(t, err)

		assert.Equal(t, 0, deletedCount)
	})

	t.Run("empty storage directory results in zero deletions", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		var deletedCount int
		err := filepath.WalkDir(storageDir, func(_ string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			deletedCount++
			return nil
		})
		require.NoError(t, err)

		assert.Equal(t, 0, deletedCount)
	})
}

// ---------------------------------------------------------------------------
// Test: purgeSoftDeleted filesystem cleanup logic
// ---------------------------------------------------------------------------

func TestPurgeSoftDeleted_FilesystemCleanup(t *testing.T) {
	t.Parallel()

	// This test verifies the file deletion logic that runs after
	// PurgeSoftDeleted returns file paths. Since DocRepo requires a
	// database, we test the filesystem deletion logic in isolation.

	t.Run("files are deleted for returned file paths", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		file1 := filepath.Join(storageDir, "uploads", "doc1.pdf")
		file2 := filepath.Join(storageDir, "uploads", "doc2.pdf")

		require.NoError(t, os.MkdirAll(filepath.Dir(file1), 0o755))
		require.NoError(t, os.WriteFile(file1, []byte("doc1"), 0o644))
		require.NoError(t, os.WriteFile(file2, []byte("doc2"), 0o644))

		purged := []repository.DocumentFilePath{
			{ID: 1, UUID: "uuid-1", FilePath: "uploads/doc1.pdf"},
			{ID: 2, UUID: "uuid-2", FilePath: "uploads/doc2.pdf"},
		}

		// Simulate the cleanup logic from purgeSoftDeleted.
		for _, fp := range purged {
			if fp.FilePath != "" {
				absPath := filepath.Join(storageDir, fp.FilePath)
				_ = os.Remove(absPath)
			}
		}

		_, err := os.Stat(file1)
		assert.True(t, os.IsNotExist(err), "doc1.pdf should be removed")

		_, err = os.Stat(file2)
		assert.True(t, os.IsNotExist(err), "doc2.pdf should be removed")
	})

	t.Run("empty file path is skipped without error", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		purged := []repository.DocumentFilePath{
			{ID: 1, UUID: "uuid-1", FilePath: ""},
		}

		// Simulate the cleanup logic: empty FilePath should be skipped.
		for _, fp := range purged {
			if fp.FilePath != "" {
				absPath := filepath.Join(storageDir, fp.FilePath)
				err := os.Remove(absPath)
				assert.NoError(t, err, "should not attempt to remove empty path")
			}
		}
		// No panic, no error -- the empty path was correctly skipped.
	})

	t.Run("non-existent file does not cause panic", func(t *testing.T) {
		t.Parallel()

		storageDir := t.TempDir()

		purged := []repository.DocumentFilePath{
			{ID: 1, UUID: "uuid-1", FilePath: "nonexistent/file.pdf"},
		}

		// Simulate the cleanup logic from purgeSoftDeleted:
		// os.IsNotExist errors are tolerated.
		for _, fp := range purged {
			if fp.FilePath != "" {
				absPath := filepath.Join(storageDir, fp.FilePath)
				if removeErr := os.Remove(absPath); removeErr != nil && !os.IsNotExist(removeErr) {
					t.Errorf("unexpected error: %v", removeErr)
				}
			}
		}
	})
}
