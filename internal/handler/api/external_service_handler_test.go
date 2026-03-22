package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/service"
)

// ---------------------------------------------------------------------------
// Mock repository implementing service.ExternalServiceRepo
// ---------------------------------------------------------------------------

type mockExternalServiceRepo struct {
	findByUUIDFn        func(ctx context.Context, uuid string) (*model.ExternalService, error)
	findEnabledByTypeFn func(ctx context.Context, serviceType string) ([]model.ExternalService, error)
	listFn              func(ctx context.Context, serviceType, status string, limit, offset int) ([]model.ExternalService, int, error)
	createFn            func(ctx context.Context, svc *model.ExternalService) error
	updateFn            func(ctx context.Context, svc *model.ExternalService) error
	deleteFn            func(ctx context.Context, id int64) error
	updateHealthFn      func(ctx context.Context, id int64, status string, latencyMs int, lastError string) error
	reorderFn           func(ctx context.Context, serviceIDs []int64) error
}

func (m *mockExternalServiceRepo) FindByUUID(ctx context.Context, uuid string) (*model.ExternalService, error) {
	if m.findByUUIDFn != nil {
		return m.findByUUIDFn(ctx, uuid)
	}
	return nil, sql.ErrNoRows
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

func (m *mockExternalServiceRepo) ReorderPriorities(ctx context.Context, serviceIDs []int64) error {
	if m.reorderFn != nil {
		return m.reorderFn(ctx, serviceIDs)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newExternalServiceHandler(mock *mockExternalServiceRepo) *ExternalServiceHandler {
	svc := service.NewExternalServiceService(mock, testLogger())
	return NewExternalServiceHandler(svc, mock, testLogger())
}

func newTestExternalService(uuid string) *model.ExternalService {
	now := sql.NullTime{Time: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC), Valid: true}
	return &model.ExternalService{
		ID:        1,
		UUID:      uuid,
		Name:      "Test Service",
		Slug:      "test-service",
		Type:      "kiwix",
		BaseURL:   "https://example.com",
		Priority:  10,
		Status:    "healthy",
		IsEnabled: true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// ---------------------------------------------------------------------------
// List handler tests
// ---------------------------------------------------------------------------

func TestExternalServiceHandler_List(t *testing.T) {
	t.Parallel()

	t.Run("returns empty list when no services exist", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{
			listFn: func(_ context.Context, _, _ string, _, _ int) ([]model.ExternalService, int, error) {
				return []model.ExternalService{}, 0, nil
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/external-services", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		body := decodeJSONBody(t, rr.Body)
		data, ok := body["data"].([]any)
		if !ok {
			t.Fatal("data field is not an array")
		}
		if len(data) != 0 {
			t.Errorf("data length = %d, want 0", len(data))
		}

		meta, ok := body["meta"].(map[string]any)
		if !ok {
			t.Fatal("meta field is not an object")
		}
		if total := meta["total"].(float64); total != 0 {
			t.Errorf("meta.total = %v, want 0", total)
		}
		if limit := meta["limit"].(float64); limit != 50 {
			t.Errorf("meta.limit = %v, want 50 (default)", limit)
		}
		if offset := meta["offset"].(float64); offset != 0 {
			t.Errorf("meta.offset = %v, want 0", offset)
		}
	})

	t.Run("returns multiple services", func(t *testing.T) {
		t.Parallel()

		services := []model.ExternalService{
			{UUID: "uuid-1", Name: "Service A", Slug: "service-a", Type: "kiwix", BaseURL: "https://a.com", Status: "healthy", IsEnabled: true},
			{UUID: "uuid-2", Name: "Service B", Slug: "service-b", Type: "confluence", BaseURL: "https://b.com", Status: "unknown", IsEnabled: false},
		}
		mock := &mockExternalServiceRepo{
			listFn: func(_ context.Context, _, _ string, _, _ int) ([]model.ExternalService, int, error) {
				return services, 2, nil
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/external-services", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		body := decodeJSONBody(t, rr.Body)
		data, ok := body["data"].([]any)
		if !ok {
			t.Fatal("data field is not an array")
		}
		if len(data) != 2 {
			t.Errorf("data length = %d, want 2", len(data))
		}

		first := data[0].(map[string]any)
		if first["uuid"] != "uuid-1" {
			t.Errorf("first uuid = %v, want uuid-1", first["uuid"])
		}
		if first["name"] != "Service A" {
			t.Errorf("first name = %v, want Service A", first["name"])
		}

		second := data[1].(map[string]any)
		if second["uuid"] != "uuid-2" {
			t.Errorf("second uuid = %v, want uuid-2", second["uuid"])
		}

		meta := body["meta"].(map[string]any)
		if total := meta["total"].(float64); total != 2 {
			t.Errorf("meta.total = %v, want 2", total)
		}
	})

	t.Run("passes filter params to service", func(t *testing.T) {
		t.Parallel()

		var gotType, gotStatus string
		var gotLimit, gotOffset int

		mock := &mockExternalServiceRepo{
			listFn: func(_ context.Context, serviceType, status string, limit, offset int) ([]model.ExternalService, int, error) {
				gotType = serviceType
				gotStatus = status
				gotLimit = limit
				gotOffset = offset
				return []model.ExternalService{}, 0, nil
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/external-services?type=kiwix&status=healthy&limit=10&offset=20", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		if gotType != "kiwix" {
			t.Errorf("type = %q, want kiwix", gotType)
		}
		if gotStatus != "healthy" {
			t.Errorf("status = %q, want healthy", gotStatus)
		}
		if gotLimit != 10 {
			t.Errorf("limit = %d, want 10", gotLimit)
		}
		if gotOffset != 20 {
			t.Errorf("offset = %d, want 20", gotOffset)
		}
	})

	t.Run("defaults limit to 50 when not provided", func(t *testing.T) {
		t.Parallel()

		var gotLimit int
		mock := &mockExternalServiceRepo{
			listFn: func(_ context.Context, _, _ string, limit, _ int) ([]model.ExternalService, int, error) {
				gotLimit = limit
				return []model.ExternalService{}, 0, nil
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/external-services", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if gotLimit != 50 {
			t.Errorf("limit = %d, want 50 (default)", gotLimit)
		}
	})

	t.Run("defaults limit to 50 when invalid", func(t *testing.T) {
		t.Parallel()

		var gotLimit int
		mock := &mockExternalServiceRepo{
			listFn: func(_ context.Context, _, _ string, limit, _ int) ([]model.ExternalService, int, error) {
				gotLimit = limit
				return []model.ExternalService{}, 0, nil
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/external-services?limit=abc", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if gotLimit != 50 {
			t.Errorf("limit = %d, want 50 (default for invalid)", gotLimit)
		}
	})

	t.Run("defaults limit to 50 when zero", func(t *testing.T) {
		t.Parallel()

		var gotLimit int
		mock := &mockExternalServiceRepo{
			listFn: func(_ context.Context, _, _ string, limit, _ int) ([]model.ExternalService, int, error) {
				gotLimit = limit
				return []model.ExternalService{}, 0, nil
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/external-services?limit=0", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if gotLimit != 50 {
			t.Errorf("limit = %d, want 50 (default for zero)", gotLimit)
		}
	})

	t.Run("defaults limit to 50 when negative", func(t *testing.T) {
		t.Parallel()

		var gotLimit int
		mock := &mockExternalServiceRepo{
			listFn: func(_ context.Context, _, _ string, limit, _ int) ([]model.ExternalService, int, error) {
				gotLimit = limit
				return []model.ExternalService{}, 0, nil
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/external-services?limit=-5", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if gotLimit != 50 {
			t.Errorf("limit = %d, want 50 (default for negative)", gotLimit)
		}
	})

	t.Run("returns 500 when repo returns error", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{
			listFn: func(_ context.Context, _, _ string, _, _ int) ([]model.ExternalService, int, error) {
				return nil, 0, errors.New("connection refused")
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/external-services", http.NoBody)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "failed to list external services" {
			t.Errorf("message = %v, want 'failed to list external services'", msg)
		}
	})
}

// ---------------------------------------------------------------------------
// Show handler tests
// ---------------------------------------------------------------------------

func TestExternalServiceHandler_Show(t *testing.T) {
	t.Parallel()

	t.Run("returns service by UUID", func(t *testing.T) {
		t.Parallel()

		es := newTestExternalService("svc-uuid-1")
		mock := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if uuid == "svc-uuid-1" {
					return es, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/external-services/svc-uuid-1", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "svc-uuid-1"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		body := decodeJSONBody(t, rr.Body)
		data, ok := body["data"].(map[string]any)
		if !ok {
			t.Fatal("data field is not an object")
		}
		if data["uuid"] != "svc-uuid-1" {
			t.Errorf("uuid = %v, want svc-uuid-1", data["uuid"])
		}
		if data["name"] != "Test Service" {
			t.Errorf("name = %v, want Test Service", data["name"])
		}
		if data["slug"] != "test-service" {
			t.Errorf("slug = %v, want test-service", data["slug"])
		}
		if data["type"] != "kiwix" {
			t.Errorf("type = %v, want kiwix", data["type"])
		}
		if data["base_url"] != "https://example.com" {
			t.Errorf("base_url = %v, want https://example.com", data["base_url"])
		}
		if data["status"] != "healthy" {
			t.Errorf("status = %v, want healthy", data["status"])
		}
		if data["is_enabled"] != true {
			t.Errorf("is_enabled = %v, want true", data["is_enabled"])
		}
		if data["priority"].(float64) != 10 {
			t.Errorf("priority = %v, want 10", data["priority"])
		}
	})

	t.Run("returns 404 when service not found", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return nil, sql.ErrNoRows
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/external-services/nonexistent", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "nonexistent"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "external service not found" {
			t.Errorf("message = %v, want 'external service not found'", msg)
		}
	})

	t.Run("returns 500 when repo returns non-ErrNoRows error", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return nil, errors.New("database connection timeout")
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/external-services/abc", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "abc"})
		rr := httptest.NewRecorder()

		h.Show(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "failed to find external service" {
			t.Errorf("message = %v, want 'failed to find external service'", msg)
		}
	})
}

// ---------------------------------------------------------------------------
// Create handler tests
// ---------------------------------------------------------------------------

func TestExternalServiceHandler_Create(t *testing.T) {
	t.Parallel()

	t.Run("creates service with valid body", func(t *testing.T) {
		t.Parallel()

		var createdSvc *model.ExternalService
		mock := &mockExternalServiceRepo{
			createFn: func(_ context.Context, svc *model.ExternalService) error {
				createdSvc = svc
				return nil
			},
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if createdSvc != nil && createdSvc.UUID == uuid {
					now := sql.NullTime{Time: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC), Valid: true}
					createdSvc.ID = 1
					createdSvc.CreatedAt = now
					createdSvc.UpdatedAt = now
					return createdSvc, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		h := newExternalServiceHandler(mock)

		reqBody := `{"name":"My Kiwix","type":"kiwix","base_url":"https://93.184.216.34","priority":5}`
		req := httptest.NewRequest(http.MethodPost, "/api/external-services", strings.NewReader(reqBody))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusCreated)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "External service created successfully." {
			t.Errorf("message = %v, want 'External service created successfully.'", msg)
		}

		data, ok := body["data"].(map[string]any)
		if !ok {
			t.Fatal("data field is not an object")
		}
		if data["name"] != "My Kiwix" {
			t.Errorf("name = %v, want My Kiwix", data["name"])
		}
		if data["type"] != "kiwix" {
			t.Errorf("type = %v, want kiwix", data["type"])
		}
		if data["base_url"] != "https://93.184.216.34" {
			t.Errorf("base_url = %v, want https://93.184.216.34", data["base_url"])
		}
		if data["slug"] != "my-kiwix" {
			t.Errorf("slug = %v, want my-kiwix", data["slug"])
		}
		if data["is_enabled"] != true {
			t.Errorf("is_enabled = %v, want true (default)", data["is_enabled"])
		}
		if data["status"] != "unknown" {
			t.Errorf("status = %v, want unknown (default)", data["status"])
		}
		if data["uuid"] == nil || data["uuid"] == "" {
			t.Error("uuid should be generated and non-empty")
		}
	})

	t.Run("creates service with optional api_key and config", func(t *testing.T) {
		t.Parallel()

		var createdSvc *model.ExternalService
		mock := &mockExternalServiceRepo{
			createFn: func(_ context.Context, svc *model.ExternalService) error {
				createdSvc = svc
				if !svc.APIKey.Valid || svc.APIKey.String != "secret-key" {
					return errors.New("api_key not set correctly")
				}
				if !svc.Config.Valid || svc.Config.String != `{"foo":"bar"}` {
					return errors.New("config not set correctly")
				}
				return nil
			},
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if createdSvc != nil && createdSvc.UUID == uuid {
					return createdSvc, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		h := newExternalServiceHandler(mock)

		reqBody := `{"name":"Svc","type":"confluence","base_url":"https://93.184.216.34","api_key":"secret-key","config":"{\"foo\":\"bar\"}"}`
		req := httptest.NewRequest(http.MethodPost, "/api/external-services", strings.NewReader(reqBody))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusCreated)
		}
	})

	t.Run("rejects missing name", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{}
		h := newExternalServiceHandler(mock)

		reqBody := `{"type":"kiwix","base_url":"https://example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/api/external-services", strings.NewReader(reqBody))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "name is required" {
			t.Errorf("message = %v, want 'name is required'", msg)
		}
	})

	t.Run("rejects missing type", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{}
		h := newExternalServiceHandler(mock)

		reqBody := `{"name":"My Service","base_url":"https://example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/api/external-services", strings.NewReader(reqBody))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "type is required" {
			t.Errorf("message = %v, want 'type is required'", msg)
		}
	})

	t.Run("rejects missing base_url", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{}
		h := newExternalServiceHandler(mock)

		reqBody := `{"name":"My Service","type":"kiwix"}`
		req := httptest.NewRequest(http.MethodPost, "/api/external-services", strings.NewReader(reqBody))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "base_url is required" {
			t.Errorf("message = %v, want 'base_url is required'", msg)
		}
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodPost, "/api/external-services", strings.NewReader("not json"))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "invalid JSON body" {
			t.Errorf("message = %v, want 'invalid JSON body'", msg)
		}
	})

	t.Run("returns 500 when repo create fails", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{
			createFn: func(_ context.Context, _ *model.ExternalService) error {
				return errors.New("unique constraint violation")
			},
		}
		h := newExternalServiceHandler(mock)

		reqBody := `{"name":"Svc","type":"kiwix","base_url":"https://93.184.216.34"}`
		req := httptest.NewRequest(http.MethodPost, "/api/external-services", strings.NewReader(reqBody))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "failed to create external service" {
			t.Errorf("message = %v, want 'failed to create external service'", msg)
		}
	})

	t.Run("validates name before type and base_url", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{}
		h := newExternalServiceHandler(mock)

		// All three fields missing -- name should be validated first.
		reqBody := `{}`
		req := httptest.NewRequest(http.MethodPost, "/api/external-services", strings.NewReader(reqBody))
		rr := httptest.NewRecorder()

		h.Create(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "name is required" {
			t.Errorf("message = %v, want 'name is required' (first validation)", msg)
		}
	})
}

// ---------------------------------------------------------------------------
// Update handler tests
// ---------------------------------------------------------------------------

func TestExternalServiceHandler_Update(t *testing.T) {
	t.Parallel()

	t.Run("updates existing service", func(t *testing.T) {
		t.Parallel()

		es := newTestExternalService("svc-uuid-1")
		callCount := 0
		mock := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if uuid == "svc-uuid-1" {
					callCount++
					if callCount > 1 {
						// Return updated version on re-fetch.
						updated := *es
						updated.Name = "Updated Name"
						updated.Slug = "updated-name"
						return &updated, nil
					}
					return es, nil
				}
				return nil, sql.ErrNoRows
			},
			updateFn: func(_ context.Context, svc *model.ExternalService) error {
				if svc.Name != "Updated Name" {
					return fmt.Errorf("expected name to be Updated Name, got %s", svc.Name)
				}
				return nil
			},
		}
		h := newExternalServiceHandler(mock)

		reqBody := `{"name":"Updated Name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/external-services/svc-uuid-1", strings.NewReader(reqBody))
		req = chiContext(req, map[string]string{"uuid": "svc-uuid-1"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "External service updated successfully." {
			t.Errorf("message = %v, want 'External service updated successfully.'", msg)
		}

		data, ok := body["data"].(map[string]any)
		if !ok {
			t.Fatal("data field is not an object")
		}
		if data["name"] != "Updated Name" {
			t.Errorf("name = %v, want Updated Name", data["name"])
		}
		if data["slug"] != "updated-name" {
			t.Errorf("slug = %v, want updated-name", data["slug"])
		}
	})

	t.Run("updates is_enabled and priority", func(t *testing.T) {
		t.Parallel()

		es := newTestExternalService("svc-uuid-2")
		callCount := 0
		mock := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if uuid == "svc-uuid-2" {
					callCount++
					if callCount > 1 {
						updated := *es
						updated.IsEnabled = false
						updated.Priority = 99
						return &updated, nil
					}
					return es, nil
				}
				return nil, sql.ErrNoRows
			},
			updateFn: func(_ context.Context, _ *model.ExternalService) error {
				return nil
			},
		}
		h := newExternalServiceHandler(mock)

		reqBody := `{"is_enabled":false,"priority":99}`
		req := httptest.NewRequest(http.MethodPut, "/api/external-services/svc-uuid-2", strings.NewReader(reqBody))
		req = chiContext(req, map[string]string{"uuid": "svc-uuid-2"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		body := decodeJSONBody(t, rr.Body)
		data := body["data"].(map[string]any)
		if data["is_enabled"] != false {
			t.Errorf("is_enabled = %v, want false", data["is_enabled"])
		}
		if data["priority"].(float64) != 99 {
			t.Errorf("priority = %v, want 99", data["priority"])
		}
	})

	t.Run("returns 404 when service not found", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return nil, sql.ErrNoRows
			},
		}
		h := newExternalServiceHandler(mock)

		reqBody := `{"name":"New Name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/external-services/nonexistent", strings.NewReader(reqBody))
		req = chiContext(req, map[string]string{"uuid": "nonexistent"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "external service not found" {
			t.Errorf("message = %v, want 'external service not found'", msg)
		}
	})

	t.Run("returns 400 for invalid JSON body", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodPut, "/api/external-services/abc", strings.NewReader("not json"))
		req = chiContext(req, map[string]string{"uuid": "abc"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "invalid JSON body" {
			t.Errorf("message = %v, want 'invalid JSON body'", msg)
		}
	})

	t.Run("returns 500 when repo update fails with non-not-found error", func(t *testing.T) {
		t.Parallel()

		es := newTestExternalService("svc-uuid-3")
		mock := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if uuid == "svc-uuid-3" {
					return es, nil
				}
				return nil, sql.ErrNoRows
			},
			updateFn: func(_ context.Context, _ *model.ExternalService) error {
				return errors.New("database write error")
			},
		}
		h := newExternalServiceHandler(mock)

		reqBody := `{"name":"Updated"}`
		req := httptest.NewRequest(http.MethodPut, "/api/external-services/svc-uuid-3", strings.NewReader(reqBody))
		req = chiContext(req, map[string]string{"uuid": "svc-uuid-3"})
		rr := httptest.NewRecorder()

		h.Update(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "failed to update external service" {
			t.Errorf("message = %v, want 'failed to update external service'", msg)
		}
	})
}

// ---------------------------------------------------------------------------
// Delete handler tests
// ---------------------------------------------------------------------------

func TestExternalServiceHandler_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes service successfully", func(t *testing.T) {
		t.Parallel()

		es := newTestExternalService("svc-uuid-1")
		mock := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if uuid == "svc-uuid-1" {
					return es, nil
				}
				return nil, sql.ErrNoRows
			},
			deleteFn: func(_ context.Context, id int64) error {
				if id != 1 {
					return fmt.Errorf("unexpected id %d", id)
				}
				return nil
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/external-services/svc-uuid-1", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "svc-uuid-1"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "External service deleted successfully." {
			t.Errorf("message = %v, want 'External service deleted successfully.'", msg)
		}
	})

	t.Run("returns 404 when service not found", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, _ string) (*model.ExternalService, error) {
				return nil, sql.ErrNoRows
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/external-services/nonexistent", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "nonexistent"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "external service not found" {
			t.Errorf("message = %v, want 'external service not found'", msg)
		}
	})

	t.Run("returns 403 for env-managed service", func(t *testing.T) {
		t.Parallel()

		es := newTestExternalService("svc-uuid-env")
		es.IsEnvManaged = true
		mock := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if uuid == "svc-uuid-env" {
					return es, nil
				}
				return nil, sql.ErrNoRows
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/external-services/svc-uuid-env", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "svc-uuid-env"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "cannot delete environment-managed external service" {
			t.Errorf("message = %v, want 'cannot delete environment-managed external service'", msg)
		}
	})

	t.Run("returns 500 when repo delete fails", func(t *testing.T) {
		t.Parallel()

		es := newTestExternalService("svc-uuid-del")
		mock := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				if uuid == "svc-uuid-del" {
					return es, nil
				}
				return nil, sql.ErrNoRows
			},
			deleteFn: func(_ context.Context, _ int64) error {
				return errors.New("foreign key constraint")
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/external-services/svc-uuid-del", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "svc-uuid-del"})
		rr := httptest.NewRecorder()

		h.Delete(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}

		body := decodeJSONBody(t, rr.Body)
		if msg := body["message"]; msg != "failed to delete external service" {
			t.Errorf("message = %v, want 'failed to delete external service'", msg)
		}
	})
}

// ---------------------------------------------------------------------------
// HealthCheck handler tests
// ---------------------------------------------------------------------------

func TestExternalServiceHandler_HealthCheck(t *testing.T) {
	t.Parallel()

	t.Run("returns current service state", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{
			findByUUIDFn: func(_ context.Context, uuid string) (*model.ExternalService, error) {
				return &model.ExternalService{
					UUID:   uuid,
					Name:   "Test Service",
					Status: "healthy",
				}, nil
			},
		}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodPost, "/api/external-services/svc-uuid-1/health", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "svc-uuid-1"})
		rr := httptest.NewRecorder()

		h.HealthCheck(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("returns 404 for unknown service", func(t *testing.T) {
		t.Parallel()

		mock := &mockExternalServiceRepo{}
		h := newExternalServiceHandler(mock)

		req := httptest.NewRequest(http.MethodPost, "/api/external-services/unknown/health", http.NoBody)
		req = chiContext(req, map[string]string{"uuid": "unknown"})
		rr := httptest.NewRecorder()

		h.HealthCheck(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})
}

// ---------------------------------------------------------------------------
// toExternalServiceResponse tests
// ---------------------------------------------------------------------------

func TestToExternalServiceResponse(t *testing.T) {
	t.Parallel()

	t.Run("maps all fields correctly", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		es := &model.ExternalService{
			UUID:                "resp-uuid",
			Name:                "Response Test",
			Slug:                "response-test",
			Type:                "kiwix",
			BaseURL:             "https://kiwix.example.com",
			Priority:            5,
			Status:              "healthy",
			IsEnabled:           true,
			IsEnvManaged:        false,
			ErrorCount:          3,
			ConsecutiveFailures: 1,
			LastError:           sql.NullString{String: "timeout", Valid: true},
			LastErrorAt:         sql.NullTime{Time: now, Valid: true},
			LastCheckAt:         sql.NullTime{Time: now, Valid: true},
			LastLatencyMS:       sql.NullInt64{Int64: 150, Valid: true},
			CreatedAt:           sql.NullTime{Time: now, Valid: true},
			UpdatedAt:           sql.NullTime{Time: now, Valid: true},
		}

		resp := toExternalServiceResponse(es)

		if resp.UUID != "resp-uuid" {
			t.Errorf("UUID = %q, want resp-uuid", resp.UUID)
		}
		if resp.Name != "Response Test" {
			t.Errorf("Name = %q, want Response Test", resp.Name)
		}
		if resp.Slug != "response-test" {
			t.Errorf("Slug = %q, want response-test", resp.Slug)
		}
		if resp.Type != "kiwix" {
			t.Errorf("Type = %q, want kiwix", resp.Type)
		}
		if resp.BaseURL != "https://kiwix.example.com" {
			t.Errorf("BaseURL = %q, want https://kiwix.example.com", resp.BaseURL)
		}
		if resp.Priority != 5 {
			t.Errorf("Priority = %d, want 5", resp.Priority)
		}
		if resp.Status != "healthy" {
			t.Errorf("Status = %q, want healthy", resp.Status)
		}
		if !resp.IsEnabled {
			t.Error("IsEnabled = false, want true")
		}
		if resp.IsEnvManaged {
			t.Error("IsEnvManaged = true, want false")
		}
		if resp.ErrorCount != 3 {
			t.Errorf("ErrorCount = %d, want 3", resp.ErrorCount)
		}
		if resp.ConsecutiveFailures != 1 {
			t.Errorf("ConsecutiveFailures = %d, want 1", resp.ConsecutiveFailures)
		}
		if resp.LastError != "timeout" {
			t.Errorf("LastError = %q, want timeout", resp.LastError)
		}
		if resp.LastLatencyMS != 150 {
			t.Errorf("LastLatencyMS = %d, want 150", resp.LastLatencyMS)
		}

		wantTime := now.Format(time.RFC3339)
		if resp.LastErrorAt != wantTime {
			t.Errorf("LastErrorAt = %q, want %q", resp.LastErrorAt, wantTime)
		}
		if resp.LastCheckAt != wantTime {
			t.Errorf("LastCheckAt = %q, want %q", resp.LastCheckAt, wantTime)
		}
		if resp.CreatedAt != wantTime {
			t.Errorf("CreatedAt = %q, want %q", resp.CreatedAt, wantTime)
		}
		if resp.UpdatedAt != wantTime {
			t.Errorf("UpdatedAt = %q, want %q", resp.UpdatedAt, wantTime)
		}
	})

	t.Run("handles null optional fields", func(t *testing.T) {
		t.Parallel()

		es := &model.ExternalService{
			UUID:      "null-uuid",
			Name:      "Null Fields",
			Slug:      "null-fields",
			Type:      "kiwix",
			BaseURL:   "https://example.com",
			Status:    "unknown",
			IsEnabled: true,
		}

		resp := toExternalServiceResponse(es)

		if resp.LastError != "" {
			t.Errorf("LastError = %q, want empty for null", resp.LastError)
		}
		if resp.LastErrorAt != "" {
			t.Errorf("LastErrorAt = %q, want empty for null", resp.LastErrorAt)
		}
		if resp.LastCheckAt != "" {
			t.Errorf("LastCheckAt = %q, want empty for null", resp.LastCheckAt)
		}
		if resp.LastLatencyMS != 0 {
			t.Errorf("LastLatencyMS = %d, want 0 for null", resp.LastLatencyMS)
		}
		if resp.CreatedAt != "" {
			t.Errorf("CreatedAt = %q, want empty for null", resp.CreatedAt)
		}
		if resp.UpdatedAt != "" {
			t.Errorf("UpdatedAt = %q, want empty for null", resp.UpdatedAt)
		}
	})
}
