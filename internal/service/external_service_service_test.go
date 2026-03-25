package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ---------------------------------------------------------------------------
// Mock
// ---------------------------------------------------------------------------

type mockExternalServiceRepo struct {
	findByUUIDFn        func(ctx context.Context, uuid string) (*model.ExternalService, error)
	findEnabledByTypeFn func(ctx context.Context, serviceType string) ([]model.ExternalService, error)
	listFn              func(ctx context.Context, serviceType, status string, limit, offset int) ([]model.ExternalService, int, error)
	createFn            func(ctx context.Context, svc *model.ExternalService) error
	updateFn            func(ctx context.Context, svc *model.ExternalService) error
	deleteFn            func(ctx context.Context, id int64) error
	updateHealthFn      func(ctx context.Context, id int64, status string, latencyMs int, lastError string) error
}

func (m *mockExternalServiceRepo) FindByUUID(ctx context.Context, uuid string) (*model.ExternalService, error) {
	if m.findByUUIDFn != nil {
		return m.findByUUIDFn(ctx, uuid)
	}
	return nil, nil
}

func (m *mockExternalServiceRepo) FindEnabledByType(ctx context.Context, serviceType string) ([]model.ExternalService, error) {
	if m.findEnabledByTypeFn != nil {
		return m.findEnabledByTypeFn(ctx, serviceType)
	}
	return nil, nil
}

func (m *mockExternalServiceRepo) List(ctx context.Context, serviceType, status string, limit, offset int) ([]model.ExternalService, int, error) {
	if m.listFn != nil {
		return m.listFn(ctx, serviceType, status, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockExternalServiceRepo) Create(ctx context.Context, svc *model.ExternalService) error {
	if m.createFn != nil {
		return m.createFn(ctx, svc)
	}
	return nil
}

func (m *mockExternalServiceRepo) Update(ctx context.Context, svc *model.ExternalService) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, svc)
	}
	return nil
}

func (m *mockExternalServiceRepo) Delete(ctx context.Context, id int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockExternalServiceRepo) UpdateHealthStatus(ctx context.Context, id int64, status string, latencyMs int, lastError string) error {
	if m.updateHealthFn != nil {
		return m.updateHealthFn(ctx, id, status, latencyMs, lastError)
	}
	return nil
}

// mockHealthChecker implements HealthChecker for tests.
type mockHealthChecker struct {
	err error
}

func (m *mockHealthChecker) Health(_ context.Context) error {
	return m.err
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// TestExternalServiceService_FindByUUID
// ---------------------------------------------------------------------------

func TestExternalServiceService_FindByUUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repoFn    func(ctx context.Context, uuid string) (*model.ExternalService, error)
		wantSvc   bool
		wantErr   bool
		errSubstr string
	}{
		{
			name: "service found",
			repoFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return &model.ExternalService{ID: 1, UUID: "svc-123", Name: "Kiwix"}, nil
			},
			wantSvc: true,
		},
		{
			name: "service not found returns ErrNotFound",
			repoFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return nil, sql.ErrNoRows
			},
			wantSvc:   false,
			wantErr:   true,
			errSubstr: "not found",
		},
		{
			name: "repository error is wrapped",
			repoFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return nil, errors.New("connection refused")
			},
			wantErr:   true,
			errSubstr: "finding external service by uuid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &mockExternalServiceRepo{findByUUIDFn: tt.repoFn}
			svc := NewExternalServiceService(repo, nil, nil, discardLogger())

			result, err := svc.FindByUUID(context.Background(), "svc-123")

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantSvc && result == nil {
				t.Fatal("expected service, got nil")
			}
			if !tt.wantSvc && result != nil {
				t.Fatalf("expected nil service, got %+v", result)
			}
			if tt.wantSvc && result.UUID != "svc-123" {
				t.Errorf("UUID = %q, want %q", result.UUID, "svc-123")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestExternalServiceService_List
// ---------------------------------------------------------------------------

func TestExternalServiceService_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repoFn    func(ctx context.Context, serviceType, status string, limit, offset int) ([]model.ExternalService, int, error)
		wantCount int
		wantTotal int
		wantErr   bool
		errSubstr string
	}{
		{
			name: "returns services and total",
			repoFn: func(_ context.Context, _, _ string, _, _ int) ([]model.ExternalService, int, error) {
				return []model.ExternalService{
					{ID: 1, Name: "Kiwix"},
					{ID: 2, Name: "Confluence"},
				}, 5, nil
			},
			wantCount: 2,
			wantTotal: 5,
		},
		{
			name: "returns empty list",
			repoFn: func(_ context.Context, _, _ string, _, _ int) ([]model.ExternalService, int, error) {
				return []model.ExternalService{}, 0, nil
			},
			wantCount: 0,
			wantTotal: 0,
		},
		{
			name: "repository error is wrapped",
			repoFn: func(_ context.Context, _, _ string, _, _ int) ([]model.ExternalService, int, error) {
				return nil, 0, errors.New("timeout")
			},
			wantErr:   true,
			errSubstr: "listing external services",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &mockExternalServiceRepo{listFn: tt.repoFn}
			svc := NewExternalServiceService(repo, nil, nil, discardLogger())

			services, total, err := svc.List(context.Background(), "kiwix", "healthy", 10, 0)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(services) != tt.wantCount {
				t.Errorf("len(services) = %d, want %d", len(services), tt.wantCount)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestExternalServiceService_Create
// ---------------------------------------------------------------------------

func TestExternalServiceService_Create(t *testing.T) {
	t.Parallel()

	t.Run("success with defaults", func(t *testing.T) {
		t.Parallel()

		var createdSvc *model.ExternalService
		repo := &mockExternalServiceRepo{
			createFn: func(_ context.Context, svc *model.ExternalService) error {
				createdSvc = svc
				svc.ID = 10
				return nil
			},
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if createdSvc != nil && createdSvc.UUID == uuid {
					return createdSvc, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		result, err := svc.Create(context.Background(), CreateExternalServiceParams{
			Name:     "My Kiwix",
			Type:     "kiwix",
			BaseURL:  "http://93.184.216.34:8080",
			Priority: 5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.UUID == "" || len(result.UUID) != 36 {
			t.Errorf("UUID = %q, want a 36-char UUID", result.UUID)
		}
		if result.Name != "My Kiwix" {
			t.Errorf("Name = %q, want %q", result.Name, "My Kiwix")
		}
		if result.Slug != "my-kiwix" {
			t.Errorf("Slug = %q, want %q", result.Slug, "my-kiwix")
		}
		if result.Type != "kiwix" {
			t.Errorf("Type = %q, want %q", result.Type, "kiwix")
		}
		if result.BaseURL != "http://93.184.216.34:8080" {
			t.Errorf("BaseURL = %q, want %q", result.BaseURL, "http://93.184.216.34:8080")
		}
		if result.Priority != 5 {
			t.Errorf("Priority = %d, want 5", result.Priority)
		}
		if result.Status != "unknown" {
			t.Errorf("Status = %q, want %q", result.Status, "unknown")
		}
		if !result.IsEnabled {
			t.Error("expected IsEnabled to be true by default")
		}
	})

	t.Run("with API key and config", func(t *testing.T) {
		t.Parallel()

		var createdSvc *model.ExternalService
		repo := &mockExternalServiceRepo{
			createFn: func(_ context.Context, svc *model.ExternalService) error {
				createdSvc = svc
				svc.ID = 11
				return nil
			},
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if createdSvc != nil && createdSvc.UUID == uuid {
					return createdSvc, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		result, err := svc.Create(context.Background(), CreateExternalServiceParams{
			Name:    "Confluence",
			Type:    "confluence",
			BaseURL: "https://93.184.216.34",
			APIKey:  "secret-key",
			Config:  `{"timeout": 30}`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.APIKey.Valid || result.APIKey.String != "secret-key" {
			t.Errorf("APIKey = %q (valid=%v), want %q", result.APIKey.String, result.APIKey.Valid, "secret-key")
		}
		if !result.Config.Valid || result.Config.String != `{"timeout": 30}` {
			t.Errorf("Config = %q (valid=%v), want %q", result.Config.String, result.Config.Valid, `{"timeout": 30}`)
		}
	})

	t.Run("without API key and config leaves them null", func(t *testing.T) {
		t.Parallel()

		var createdSvc *model.ExternalService
		repo := &mockExternalServiceRepo{
			createFn: func(_ context.Context, svc *model.ExternalService) error {
				createdSvc = svc
				svc.ID = 12
				return nil
			},
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if createdSvc != nil && createdSvc.UUID == uuid {
					return createdSvc, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		result, err := svc.Create(context.Background(), CreateExternalServiceParams{
			Name:    "Plain",
			Type:    "kiwix",
			BaseURL: "http://93.184.216.34",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.APIKey.Valid {
			t.Errorf("expected APIKey to be invalid (null), got %q", result.APIKey.String)
		}
		if result.Config.Valid {
			t.Errorf("expected Config to be invalid (null), got %q", result.Config.String)
		}
	})

	t.Run("repository create error is wrapped", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{
			createFn: func(_ context.Context, _ *model.ExternalService) error {
				return errors.New("unique constraint")
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.Create(context.Background(), CreateExternalServiceParams{
			Name:    "Fail",
			Type:    "kiwix",
			BaseURL: "http://93.184.216.34",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "creating external service") {
			t.Errorf("error %q does not contain %q", err.Error(), "creating external service")
		}
	})

	t.Run("re-fetch error is wrapped", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{
			createFn: func(_ context.Context, svc *model.ExternalService) error {
				svc.ID = 13
				return nil
			},
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return nil, errors.New("gone")
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.Create(context.Background(), CreateExternalServiceParams{
			Name:    "Refetch Fail",
			Type:    "kiwix",
			BaseURL: "http://93.184.216.34",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "re-fetching created external service") {
			t.Errorf("error %q does not contain %q", err.Error(), "re-fetching created external service")
		}
	})
}

// ---------------------------------------------------------------------------
// TestExternalServiceService_Create_SSRF
// ---------------------------------------------------------------------------

func TestExternalServiceService_Create_SSRF(t *testing.T) {
	t.Parallel()

	t.Run("allows private IP URL for internal services", func(t *testing.T) {
		t.Parallel()

		var createdSvc *model.ExternalService
		repo := &mockExternalServiceRepo{
			createFn: func(_ context.Context, svc *model.ExternalService) error {
				createdSvc = svc
				svc.ID = 99
				return nil
			},
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if createdSvc != nil && createdSvc.UUID == uuid {
					return createdSvc, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.Create(context.Background(), CreateExternalServiceParams{
			Name:    "Internal Kiwix",
			Type:    "kiwix",
			BaseURL: "http://192.168.1.2/api",
		})
		if err != nil {
			t.Fatalf("expected no error for private IP, got: %v", err)
		}
	})

	t.Run("rejects loopback URL", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.Create(context.Background(), CreateExternalServiceParams{
			Name:    "Loopback",
			Type:    "kiwix",
			BaseURL: "http://127.0.0.1/api",
		})
		if err == nil {
			t.Fatal("expected error for loopback, got nil")
		}
		if !strings.Contains(err.Error(), "base URL validation") {
			t.Errorf("error %q does not contain %q", err.Error(), "base URL validation")
		}
	})

	t.Run("accepts valid public URL", func(t *testing.T) {
		t.Parallel()

		var createdSvc *model.ExternalService
		repo := &mockExternalServiceRepo{
			createFn: func(_ context.Context, svc *model.ExternalService) error {
				createdSvc = svc
				svc.ID = 100
				return nil
			},
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if createdSvc != nil && createdSvc.UUID == uuid {
					return createdSvc, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		result, err := svc.Create(context.Background(), CreateExternalServiceParams{
			Name:    "Public",
			Type:    "kiwix",
			BaseURL: "https://93.184.216.34/api",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.BaseURL != "https://93.184.216.34/api" {
			t.Errorf("BaseURL = %q, want %q", result.BaseURL, "https://93.184.216.34/api")
		}
	})
}

// ---------------------------------------------------------------------------
// TestExternalServiceService_Update_SSRF
// ---------------------------------------------------------------------------

func TestExternalServiceService_Update_SSRF(t *testing.T) {
	t.Parallel()

	existingSvc := func() *model.ExternalService {
		return &model.ExternalService{
			ID:        50,
			UUID:      "ssrf-upd-uuid",
			Name:      "Original",
			Slug:      "original",
			Type:      "kiwix",
			BaseURL:   "http://93.184.216.34",
			Priority:  1,
			IsEnabled: true,
		}
	}

	t.Run("allows private IP URL for internal services", func(t *testing.T) {
		t.Parallel()

		updated := existingSvc()
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return updated, nil
			},
			updateFn: func(_ context.Context, svc *model.ExternalService) error {
				*updated = *svc
				return nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.Update(context.Background(), "ssrf-upd-uuid", UpdateExternalServiceParams{
			BaseURL: "http://192.168.1.2/api",
		})
		if err != nil {
			t.Fatalf("expected no error for private IP, got: %v", err)
		}
	})

	t.Run("rejects loopback URL", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return existingSvc(), nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.Update(context.Background(), "ssrf-upd-uuid", UpdateExternalServiceParams{
			BaseURL: "http://127.0.0.1/api",
		})
		if err == nil {
			t.Fatal("expected error for loopback, got nil")
		}
		if !strings.Contains(err.Error(), "base URL validation") {
			t.Errorf("error %q does not contain %q", err.Error(), "base URL validation")
		}
	})

	t.Run("accepts valid public URL", func(t *testing.T) {
		t.Parallel()

		var updatedSvc *model.ExternalService
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				if updatedSvc != nil {
					return updatedSvc, nil
				}
				return existingSvc(), nil
			},
			updateFn: func(_ context.Context, svc *model.ExternalService) error {
				updatedSvc = svc
				return nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		result, err := svc.Update(context.Background(), "ssrf-upd-uuid", UpdateExternalServiceParams{
			BaseURL: "https://93.184.216.34/api",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.BaseURL != "https://93.184.216.34/api" {
			t.Errorf("BaseURL = %q, want %q", result.BaseURL, "https://93.184.216.34/api")
		}
	})

	t.Run("empty BaseURL skips validation", func(t *testing.T) {
		t.Parallel()

		var updatedSvc *model.ExternalService
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				if updatedSvc != nil {
					return updatedSvc, nil
				}
				return existingSvc(), nil
			},
			updateFn: func(_ context.Context, svc *model.ExternalService) error {
				updatedSvc = svc
				return nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		result, err := svc.Update(context.Background(), "ssrf-upd-uuid", UpdateExternalServiceParams{
			Name: "Renamed Only",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// BaseURL should remain unchanged from the existing service.
		if result.BaseURL != "http://93.184.216.34" {
			t.Errorf("BaseURL = %q, want %q (should remain unchanged)", result.BaseURL, "http://93.184.216.34")
		}
	})
}

// ---------------------------------------------------------------------------
// TestExternalServiceService_Update
// ---------------------------------------------------------------------------

func TestExternalServiceService_Update(t *testing.T) {
	t.Parallel()

	existingSvc := func() *model.ExternalService {
		return &model.ExternalService{
			ID:        20,
			UUID:      "upd-uuid",
			Name:      "Original",
			Slug:      "original",
			Type:      "kiwix",
			BaseURL:   "http://old.example.com",
			Priority:  1,
			IsEnabled: true,
		}
	}

	t.Run("success applies name and slug change", func(t *testing.T) {
		t.Parallel()

		var updatedSvc *model.ExternalService
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if updatedSvc != nil {
					return updatedSvc, nil
				}
				return existingSvc(), nil
			},
			updateFn: func(_ context.Context, svc *model.ExternalService) error {
				updatedSvc = svc
				return nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		result, err := svc.Update(context.Background(), "upd-uuid", UpdateExternalServiceParams{
			Name: "New Name",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Name != "New Name" {
			t.Errorf("Name = %q, want %q", result.Name, "New Name")
		}
		if result.Slug != "new-name" {
			t.Errorf("Slug = %q, want %q", result.Slug, "new-name")
		}
	})

	t.Run("applies base URL change", func(t *testing.T) {
		t.Parallel()

		var updatedSvc *model.ExternalService
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if updatedSvc != nil {
					return updatedSvc, nil
				}
				return existingSvc(), nil
			},
			updateFn: func(_ context.Context, svc *model.ExternalService) error {
				updatedSvc = svc
				return nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		result, err := svc.Update(context.Background(), "upd-uuid", UpdateExternalServiceParams{
			BaseURL: "http://93.184.216.34",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.BaseURL != "http://93.184.216.34" {
			t.Errorf("BaseURL = %q, want %q", result.BaseURL, "http://93.184.216.34")
		}
		// Name should remain unchanged.
		if result.Name != "Original" {
			t.Errorf("Name = %q, want %q (should not change)", result.Name, "Original")
		}
	})

	t.Run("applies priority and isEnabled change", func(t *testing.T) {
		t.Parallel()

		var updatedSvc *model.ExternalService
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if updatedSvc != nil {
					return updatedSvc, nil
				}
				return existingSvc(), nil
			},
			updateFn: func(_ context.Context, svc *model.ExternalService) error {
				updatedSvc = svc
				return nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		result, err := svc.Update(context.Background(), "upd-uuid", UpdateExternalServiceParams{
			Priority:  new(10),
			IsEnabled: new(false),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Priority != 10 {
			t.Errorf("Priority = %d, want 10", result.Priority)
		}
		if result.IsEnabled {
			t.Error("expected IsEnabled to be false")
		}
	})

	t.Run("applies API key and config change", func(t *testing.T) {
		t.Parallel()

		var updatedSvc *model.ExternalService
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if updatedSvc != nil {
					return updatedSvc, nil
				}
				return existingSvc(), nil
			},
			updateFn: func(_ context.Context, svc *model.ExternalService) error {
				updatedSvc = svc
				return nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		result, err := svc.Update(context.Background(), "upd-uuid", UpdateExternalServiceParams{
			APIKey: "new-secret",
			Config: `{"new": true}`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.APIKey.Valid || result.APIKey.String != "new-secret" {
			t.Errorf("APIKey = %q (valid=%v), want %q", result.APIKey.String, result.APIKey.Valid, "new-secret")
		}
		if !result.Config.Valid || result.Config.String != `{"new": true}` {
			t.Errorf("Config = %q (valid=%v), want %q", result.Config.String, result.Config.Valid, `{"new": true}`)
		}
	})

	t.Run("service not found returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.Update(context.Background(), "missing-uuid", UpdateExternalServiceParams{
			Name: "Nope",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("repository update error is wrapped", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return existingSvc(), nil
			},
			updateFn: func(_ context.Context, _ *model.ExternalService) error {
				return errors.New("db write failed")
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.Update(context.Background(), "upd-uuid", UpdateExternalServiceParams{
			Name: "Boom",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "updating external service") {
			t.Errorf("error %q does not contain %q", err.Error(), "updating external service")
		}
	})

	t.Run("re-fetch error is wrapped", func(t *testing.T) {
		t.Parallel()

		callCount := 0
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				callCount++
				if callCount == 1 {
					return existingSvc(), nil
				}
				return nil, errors.New("db gone")
			},
			updateFn: func(_ context.Context, _ *model.ExternalService) error {
				return nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.Update(context.Background(), "upd-uuid", UpdateExternalServiceParams{
			Name: "Whatever",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "re-fetching updated external service") {
			t.Errorf("error %q does not contain %q", err.Error(), "re-fetching updated external service")
		}
	})
}

// ---------------------------------------------------------------------------
// TestExternalServiceService_Delete
// ---------------------------------------------------------------------------

func TestExternalServiceService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("success deletes non-env-managed service", func(t *testing.T) {
		t.Parallel()

		deleteCalled := false
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return &model.ExternalService{
					ID:           30,
					UUID:         "del-uuid",
					IsEnvManaged: false,
				}, nil
			},
			deleteFn: func(_ context.Context, id int64) error {
				deleteCalled = true
				if id != 30 {
					t.Errorf("Delete id = %d, want 30", id)
				}
				return nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		err := svc.Delete(context.Background(), "del-uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !deleteCalled {
			t.Error("expected Delete to be called")
		}
	})

	t.Run("service not found returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		err := svc.Delete(context.Background(), "ghost-uuid")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("env-managed service cannot be deleted", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return &model.ExternalService{
					ID:           31,
					UUID:         "env-uuid",
					IsEnvManaged: true,
				}, nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		err := svc.Delete(context.Background(), "env-uuid")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrEnvManaged) {
			t.Errorf("expected ErrEnvManaged, got: %v", err)
		}
	})

	t.Run("repository delete error is wrapped", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return &model.ExternalService{
					ID:           32,
					UUID:         "err-del-uuid",
					IsEnvManaged: false,
				}, nil
			},
			deleteFn: func(_ context.Context, _ int64) error {
				return errors.New("foreign key")
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		err := svc.Delete(context.Background(), "err-del-uuid")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "deleting external service") {
			t.Errorf("error %q does not contain %q", err.Error(), "deleting external service")
		}
	})

	t.Run("FindByUUID repo error is wrapped", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return nil, errors.New("connection reset")
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		err := svc.Delete(context.Background(), "err-uuid")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "finding external service for deletion") {
			t.Errorf("error %q does not contain %q", err.Error(), "finding external service for deletion")
		}
	})
}

// ---------------------------------------------------------------------------
// Mock zimArchiveFinder and ExternalServiceIndexCleaner
// ---------------------------------------------------------------------------

type mockZimArchiveFinder struct {
	findUUIDsFn func(ctx context.Context, serviceID int64) ([]string, error)
}

func (m *mockZimArchiveFinder) FindUUIDsByExternalServiceID(ctx context.Context, serviceID int64) ([]string, error) {
	if m.findUUIDsFn != nil {
		return m.findUUIDsFn(ctx, serviceID)
	}
	return nil, nil
}

type mockIndexCleaner struct {
	deleteZimFn func(ctx context.Context, uuid string) error
}

func (m *mockIndexCleaner) DeleteZimArchive(ctx context.Context, uuid string) error {
	if m.deleteZimFn != nil {
		return m.deleteZimFn(ctx, uuid)
	}
	return nil
}

// ---------------------------------------------------------------------------
// TestExternalServiceService_Delete_WithCleanup
// ---------------------------------------------------------------------------

func TestExternalServiceService_Delete_WithCleanup(t *testing.T) {
	t.Parallel()

	t.Run("kiwix type cleans up ZIM archives from index", func(t *testing.T) {
		t.Parallel()

		var deletedUUIDs []string
		zimRepo := &mockZimArchiveFinder{
			findUUIDsFn: func(_ context.Context, serviceID int64) ([]string, error) {
				if serviceID != 40 {
					t.Errorf("FindUUIDsByExternalServiceID serviceID = %d, want 40", serviceID)
				}
				return []string{"zim-uuid-1", "zim-uuid-2"}, nil
			},
		}
		indexCleaner := &mockIndexCleaner{
			deleteZimFn: func(_ context.Context, uuid string) error {
				deletedUUIDs = append(deletedUUIDs, uuid)
				return nil
			},
		}
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return &model.ExternalService{
					ID:   40,
					UUID: "kiwix-svc-uuid",
					Type: "kiwix",
				}, nil
			},
			deleteFn: func(_ context.Context, _ int64) error {
				return nil
			},
		}

		svc := NewExternalServiceService(repo, zimRepo, indexCleaner, discardLogger())
		err := svc.Delete(context.Background(), "kiwix-svc-uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(deletedUUIDs) != 2 {
			t.Fatalf("expected 2 deleted UUIDs, got %d", len(deletedUUIDs))
		}
		if deletedUUIDs[0] != "zim-uuid-1" || deletedUUIDs[1] != "zim-uuid-2" {
			t.Errorf("deleted UUIDs = %v, want [zim-uuid-1 zim-uuid-2]", deletedUUIDs)
		}
	})

	t.Run("non-kiwix type skips cleanup", func(t *testing.T) {
		t.Parallel()

		zimRepoCalled := false
		zimRepo := &mockZimArchiveFinder{
			findUUIDsFn: func(_ context.Context, _ int64) ([]string, error) {
				zimRepoCalled = true
				return nil, nil
			},
		}
		indexCleaner := &mockIndexCleaner{}
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return &model.ExternalService{
					ID:   41,
					UUID: "meili-svc-uuid",
					Type: "meilisearch",
				}, nil
			},
			deleteFn: func(_ context.Context, _ int64) error {
				return nil
			},
		}

		svc := NewExternalServiceService(repo, zimRepo, indexCleaner, discardLogger())
		err := svc.Delete(context.Background(), "meili-svc-uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if zimRepoCalled {
			t.Error("expected zimRepo NOT to be called for non-kiwix type")
		}
	})

	t.Run("nil zimRepo skips cleanup", func(t *testing.T) {
		t.Parallel()

		indexCleaner := &mockIndexCleaner{}
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return &model.ExternalService{
					ID:   42,
					UUID: "nil-zim-uuid",
					Type: "kiwix",
				}, nil
			},
			deleteFn: func(_ context.Context, _ int64) error {
				return nil
			},
		}

		svc := NewExternalServiceService(repo, nil, indexCleaner, discardLogger())
		err := svc.Delete(context.Background(), "nil-zim-uuid")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("zimRepo error is non-fatal", func(t *testing.T) {
		t.Parallel()

		zimRepo := &mockZimArchiveFinder{
			findUUIDsFn: func(_ context.Context, _ int64) ([]string, error) {
				return nil, errors.New("zim lookup failed")
			},
		}
		indexCleaner := &mockIndexCleaner{}
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return &model.ExternalService{
					ID:   43,
					UUID: "zim-err-uuid",
					Type: "kiwix",
				}, nil
			},
			deleteFn: func(_ context.Context, _ int64) error {
				return nil
			},
		}

		svc := NewExternalServiceService(repo, zimRepo, indexCleaner, discardLogger())
		err := svc.Delete(context.Background(), "zim-err-uuid")
		if err != nil {
			t.Fatalf("expected no error (cleanup is best-effort), got: %v", err)
		}
	})

	t.Run("indexCleaner error is non-fatal and continues", func(t *testing.T) {
		t.Parallel()

		deleteCallCount := 0
		zimRepo := &mockZimArchiveFinder{
			findUUIDsFn: func(_ context.Context, _ int64) ([]string, error) {
				return []string{"fail-uuid", "ok-uuid"}, nil
			},
		}
		indexCleaner := &mockIndexCleaner{
			deleteZimFn: func(_ context.Context, uuid string) error {
				deleteCallCount++
				if uuid == "fail-uuid" {
					return errors.New("delete failed")
				}
				return nil
			},
		}
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return &model.ExternalService{
					ID:   44,
					UUID: "cleaner-err-uuid",
					Type: "kiwix",
				}, nil
			},
			deleteFn: func(_ context.Context, _ int64) error {
				return nil
			},
		}

		svc := NewExternalServiceService(repo, zimRepo, indexCleaner, discardLogger())
		err := svc.Delete(context.Background(), "cleaner-err-uuid")
		if err != nil {
			t.Fatalf("expected no error (cleanup is best-effort), got: %v", err)
		}
		if deleteCallCount != 2 {
			t.Errorf("expected DeleteZimArchive to be called 2 times, got %d", deleteCallCount)
		}
	})
}

// ---------------------------------------------------------------------------
// TestExternalServiceService_CheckHealth
// ---------------------------------------------------------------------------

func TestExternalServiceService_CheckHealth(t *testing.T) {
	t.Parallel()

	t.Run("healthy service", func(t *testing.T) {
		t.Parallel()

		var healthStatus string
		var healthLatency int
		var healthError string

		svcModel := &model.ExternalService{
			ID:   40,
			UUID: "health-uuid",
			Name: "Healthy",
		}

		callCount := 0
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				callCount++
				if callCount == 1 {
					return svcModel, nil
				}
				// After health update, return refreshed model.
				return &model.ExternalService{
					ID:     40,
					UUID:   "health-uuid",
					Name:   "Healthy",
					Status: "healthy",
				}, nil
			},
			updateHealthFn: func(_ context.Context, id int64, status string, latencyMs int, lastError string) error {
				if id != 40 {
					t.Errorf("UpdateHealthStatus id = %d, want 40", id)
				}
				healthStatus = status
				healthLatency = latencyMs
				healthError = lastError
				return nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		checker := &mockHealthChecker{err: nil}
		result, err := svc.CheckHealth(context.Background(), "health-uuid", checker)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if healthStatus != "healthy" {
			t.Errorf("status = %q, want %q", healthStatus, "healthy")
		}
		if healthLatency < 0 {
			t.Errorf("latency = %d, want >= 0", healthLatency)
		}
		if healthError != "" {
			t.Errorf("lastError = %q, want empty", healthError)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Status != "healthy" {
			t.Errorf("result.Status = %q, want %q", result.Status, "healthy")
		}
	})

	t.Run("unhealthy service", func(t *testing.T) {
		t.Parallel()

		var healthStatus string
		var healthError string

		svcModel := &model.ExternalService{
			ID:   41,
			UUID: "unhealthy-uuid",
			Name: "Unhealthy",
		}

		callCount := 0
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				callCount++
				if callCount == 1 {
					return svcModel, nil
				}
				return &model.ExternalService{
					ID:     41,
					UUID:   "unhealthy-uuid",
					Status: "unhealthy",
				}, nil
			},
			updateHealthFn: func(_ context.Context, _ int64, status string, _ int, lastError string) error {
				healthStatus = status
				healthError = lastError
				return nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		checker := &mockHealthChecker{err: errors.New("connection refused")}
		result, err := svc.CheckHealth(context.Background(), "unhealthy-uuid", checker)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if healthStatus != "unhealthy" {
			t.Errorf("status = %q, want %q", healthStatus, "unhealthy")
		}
		if healthError != "connection refused" {
			t.Errorf("lastError = %q, want %q", healthError, "connection refused")
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("service not found returns error", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return nil, sql.ErrNoRows
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.CheckHealth(context.Background(), "ghost-uuid", &mockHealthChecker{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("UpdateHealthStatus error is wrapped", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return &model.ExternalService{ID: 42, UUID: "upd-health-err"}, nil
			},
			updateHealthFn: func(_ context.Context, _ int64, _ string, _ int, _ string) error {
				return errors.New("write failed")
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.CheckHealth(context.Background(), "upd-health-err", &mockHealthChecker{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "updating health status") {
			t.Errorf("error %q does not contain %q", err.Error(), "updating health status")
		}
	})

	t.Run("re-fetch after health update error is wrapped", func(t *testing.T) {
		t.Parallel()

		callCount := 0
		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				callCount++
				if callCount == 1 {
					return &model.ExternalService{ID: 43, UUID: "refetch-health-err"}, nil
				}
				return nil, errors.New("db gone")
			},
			updateHealthFn: func(_ context.Context, _ int64, _ string, _ int, _ string) error {
				return nil
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.CheckHealth(context.Background(), "refetch-health-err", &mockHealthChecker{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "re-fetching external service after health check") {
			t.Errorf("error %q does not contain %q", err.Error(), "re-fetching external service after health check")
		}
	})

	t.Run("FindByUUID repo error is wrapped", func(t *testing.T) {
		t.Parallel()

		repo := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return nil, errors.New("timeout")
			},
		}
		svc := NewExternalServiceService(repo, nil, nil, discardLogger())

		_, err := svc.CheckHealth(context.Background(), "timeout-uuid", &mockHealthChecker{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "finding external service for health check") {
			t.Errorf("error %q does not contain %q", err.Error(), "finding external service for health check")
		}
	})
}

