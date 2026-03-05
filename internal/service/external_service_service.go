package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/security"
)

// ExternalServiceRepo defines repository methods needed by ExternalServiceService.
type ExternalServiceRepo interface {
	FindByUUID(ctx context.Context, uuid string) (*model.ExternalService, error)
	FindEnabledByType(ctx context.Context, serviceType string) ([]model.ExternalService, error)
	List(ctx context.Context, serviceType, status string, limit, offset int) ([]model.ExternalService, int, error)
	Create(ctx context.Context, svc *model.ExternalService) error
	Update(ctx context.Context, svc *model.ExternalService) error
	Delete(ctx context.Context, id int64) error
	UpdateHealthStatus(ctx context.Context, id int64, status string, latencyMs int, lastError string) error
}

// HealthChecker performs health checks on an external service.
type HealthChecker interface {
	Health(ctx context.Context) error
}

// CreateExternalServiceParams holds the input for creating an external service.
type CreateExternalServiceParams struct {
	Name     string
	Type     string // kiwix, confluence
	BaseURL  string
	APIKey   string
	Config   string // JSON string
	Priority int
}

// UpdateExternalServiceParams holds the input for updating an external service.
type UpdateExternalServiceParams struct {
	Name      string
	BaseURL   string
	APIKey    string
	Config    string
	Priority  *int
	IsEnabled *bool
}

// ExternalServiceService handles CRUD and health check orchestration for external services.
type ExternalServiceService struct {
	repo   ExternalServiceRepo
	logger *slog.Logger
}

// NewExternalServiceService creates a new ExternalServiceService.
func NewExternalServiceService(repo ExternalServiceRepo, logger *slog.Logger) *ExternalServiceService {
	return &ExternalServiceService{repo: repo, logger: logger}
}

// List returns external services filtered by type and status with pagination.
func (s *ExternalServiceService) List(ctx context.Context, serviceType, status string, limit, offset int) ([]model.ExternalService, int, error) {
	services, total, err := s.repo.List(ctx, serviceType, status, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing external services: %w", err)
	}
	return services, total, nil
}

// FindByUUID retrieves an external service by its UUID.
// Returns (nil, nil) when the service does not exist.
func (s *ExternalServiceService) FindByUUID(ctx context.Context, svcUUID string) (*model.ExternalService, error) {
	svc, err := s.repo.FindByUUID(ctx, svcUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("finding external service by uuid: %w", err)
	}
	return svc, nil
}

// Create creates a new external service with a generated UUID and slug.
func (s *ExternalServiceService) Create(ctx context.Context, params CreateExternalServiceParams) (*model.ExternalService, error) {
	if err := security.ValidateExternalURL(params.BaseURL); err != nil {
		return nil, fmt.Errorf("base URL validation: %w", err)
	}

	svc := &model.ExternalService{
		UUID:     uuid.New().String(),
		Name:     params.Name,
		Slug:     slugify(params.Name),
		Type:     params.Type,
		BaseURL:  params.BaseURL,
		Priority: params.Priority,
		Status:   "unknown",
		IsEnabled: true,
	}

	if params.APIKey != "" {
		svc.APIKey = sql.NullString{String: params.APIKey, Valid: true}
	}
	if params.Config != "" {
		svc.Config = sql.NullString{String: params.Config, Valid: true}
	}

	if err := s.repo.Create(ctx, svc); err != nil {
		return nil, fmt.Errorf("creating external service: %w", err)
	}

	created, err := s.repo.FindByUUID(ctx, svc.UUID)
	if err != nil {
		return nil, fmt.Errorf("re-fetching created external service: %w", err)
	}

	return created, nil
}

// Update applies partial updates to an existing external service identified by UUID.
func (s *ExternalServiceService) Update(ctx context.Context, svcUUID string, params UpdateExternalServiceParams) (*model.ExternalService, error) {
	svc, err := s.FindByUUID(ctx, svcUUID)
	if err != nil {
		return nil, fmt.Errorf("finding external service for update: %w", err)
	}
	if svc == nil {
		return nil, fmt.Errorf("external service %s: %w", svcUUID, ErrNotFound)
	}

	if params.Name != "" {
		svc.Name = params.Name
		svc.Slug = slugify(params.Name)
	}
	if params.BaseURL != "" {
		if err := security.ValidateExternalURL(params.BaseURL); err != nil {
			return nil, fmt.Errorf("base URL validation: %w", err)
		}
		svc.BaseURL = params.BaseURL
	}
	if params.APIKey != "" {
		svc.APIKey = sql.NullString{String: params.APIKey, Valid: true}
	}
	if params.Config != "" {
		svc.Config = sql.NullString{String: params.Config, Valid: true}
	}
	if params.Priority != nil {
		svc.Priority = *params.Priority
	}
	if params.IsEnabled != nil {
		svc.IsEnabled = *params.IsEnabled
	}

	if err := s.repo.Update(ctx, svc); err != nil {
		return nil, fmt.Errorf("updating external service: %w", err)
	}

	updated, err := s.repo.FindByUUID(ctx, svc.UUID)
	if err != nil {
		return nil, fmt.Errorf("re-fetching updated external service: %w", err)
	}

	return updated, nil
}

// Delete removes a non-env-managed external service by UUID.
func (s *ExternalServiceService) Delete(ctx context.Context, svcUUID string) error {
	svc, err := s.FindByUUID(ctx, svcUUID)
	if err != nil {
		return fmt.Errorf("finding external service for deletion: %w", err)
	}
	if svc == nil {
		return fmt.Errorf("external service %s: %w", svcUUID, ErrNotFound)
	}

	if svc.IsEnvManaged {
		return fmt.Errorf("external service %s: %w", svcUUID, ErrEnvManaged)
	}

	if err := s.repo.Delete(ctx, svc.ID); err != nil {
		return fmt.Errorf("deleting external service: %w", err)
	}

	return nil
}

// CheckHealth performs a health check on the external service identified by UUID,
// measures latency, and persists the result.
func (s *ExternalServiceService) CheckHealth(ctx context.Context, svcUUID string, checker HealthChecker) (*model.ExternalService, error) {
	svc, err := s.FindByUUID(ctx, svcUUID)
	if err != nil {
		return nil, fmt.Errorf("finding external service for health check: %w", err)
	}
	if svc == nil {
		return nil, fmt.Errorf("external service %s: %w", svcUUID, ErrNotFound)
	}

	start := time.Now()
	healthErr := checker.Health(ctx)
	latencyMs := int(time.Since(start).Milliseconds())

	var status, lastError string
	if healthErr != nil {
		status = "unhealthy"
		lastError = healthErr.Error()
		s.logger.Warn("health check failed",
			"uuid", svcUUID,
			"latency_ms", latencyMs,
			"error", lastError,
		)
	} else {
		status = "healthy"
		s.logger.Info("health check passed",
			"uuid", svcUUID,
			"latency_ms", latencyMs,
		)
	}

	if err := s.repo.UpdateHealthStatus(ctx, svc.ID, status, latencyMs, lastError); err != nil {
		return nil, fmt.Errorf("updating health status: %w", err)
	}

	updated, err := s.repo.FindByUUID(ctx, svc.UUID)
	if err != nil {
		return nil, fmt.Errorf("re-fetching external service after health check: %w", err)
	}

	return updated, nil
}

// slugify converts a name to a URL-friendly slug.
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, s)

	// Collapse consecutive hyphens.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")

	return s
}
