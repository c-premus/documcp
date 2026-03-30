package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/security"
	"github.com/c-premus/documcp/internal/stringutil"
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
	Type     string // kiwix
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

// zimArchiveFinder finds ZIM archive UUIDs for an external service.
type zimArchiveFinder interface {
	FindUUIDsByExternalServiceID(ctx context.Context, serviceID int64) ([]string, error)
}

// ExternalServiceIndexCleaner removes indexed entries on service deletion.
type ExternalServiceIndexCleaner interface {
	DeleteZimArchive(ctx context.Context, uuid string) error
}

// ExternalServiceService handles CRUD and health check orchestration for external services.
type ExternalServiceService struct {
	repo         ExternalServiceRepo
	zimRepo      zimArchiveFinder
	indexCleaner ExternalServiceIndexCleaner
	logger       *slog.Logger
}

// NewExternalServiceService creates a new ExternalServiceService.
func NewExternalServiceService(
	repo ExternalServiceRepo,
	zimRepo zimArchiveFinder,
	indexCleaner ExternalServiceIndexCleaner,
	logger *slog.Logger,
) *ExternalServiceService {
	return &ExternalServiceService{
		repo:         repo,
		zimRepo:      zimRepo,
		indexCleaner: indexCleaner,
		logger:       logger,
	}
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
// Returns ErrNotFound when the service does not exist.
func (s *ExternalServiceService) FindByUUID(ctx context.Context, svcUUID string) (*model.ExternalService, error) {
	svc, err := s.repo.FindByUUID(ctx, svcUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("finding external service by uuid: %w", err)
	}
	return svc, nil
}

// Create creates a new external service with a generated UUID and slug.
func (s *ExternalServiceService) Create(ctx context.Context, params CreateExternalServiceParams) (*model.ExternalService, error) {
	if err := security.ValidateExternalURL(params.BaseURL, true); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidURL, err)
	}

	svc := &model.ExternalService{
		UUID:      uuid.New().String(),
		Name:      params.Name,
		Slug:      stringutil.Slugify(params.Name),
		Type:      params.Type,
		BaseURL:   params.BaseURL,
		Priority:  params.Priority,
		Status:    "unknown",
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

	if params.Name != "" {
		svc.Name = params.Name
		svc.Slug = stringutil.Slugify(params.Name)
	}
	if params.BaseURL != "" {
		if err = security.ValidateExternalURL(params.BaseURL, true); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidURL, err)
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

	if err = s.repo.Update(ctx, svc); err != nil {
		return nil, fmt.Errorf("updating external service: %w", err)
	}

	updated, err := s.repo.FindByUUID(ctx, svc.UUID)
	if err != nil {
		return nil, fmt.Errorf("re-fetching updated external service: %w", err)
	}

	return updated, nil
}

// Delete removes a non-env-managed external service by UUID.
// If a search index is configured, indexed entries associated with this service
// are removed first. Index cleanup failures are non-fatal and only logged.
func (s *ExternalServiceService) Delete(ctx context.Context, svcUUID string) error {
	svc, err := s.FindByUUID(ctx, svcUUID)
	if err != nil {
		return fmt.Errorf("finding external service for deletion: %w", err)
	}

	if svc.IsEnvManaged {
		return fmt.Errorf("external service %s: %w", svcUUID, ErrEnvManaged)
	}

	if s.indexCleaner != nil {
		s.cleanupServiceIndex(ctx, svc.ID, svc.Type)
	}

	if err := s.repo.Delete(ctx, svc.ID); err != nil {
		return fmt.Errorf("deleting external service: %w", err)
	}

	return nil
}

// cleanupServiceIndex removes indexed entries for the given service from PostgreSQL full-text search indexes.
// Errors are logged but do not block deletion.
func (s *ExternalServiceService) cleanupServiceIndex(ctx context.Context, serviceID int64, serviceType string) {
	if serviceType != "kiwix" {
		return
	}
	if s.zimRepo == nil {
		return
	}
	uuids, err := s.zimRepo.FindUUIDsByExternalServiceID(ctx, serviceID)
	if err != nil {
		s.logger.Warn("failed to find ZIM archive UUIDs for index cleanup",
			"service_id", serviceID, "error", err)
		return
	}
	for _, uuid := range uuids {
		if err := s.indexCleaner.DeleteZimArchive(ctx, uuid); err != nil {
			s.logger.Warn("failed to delete ZIM archive from search index",
				"uuid", uuid, "error", err)
		}
	}
	s.logger.Info("cleaned up ZIM archives from search index",
		"service_id", serviceID, "count", len(uuids))
}

// CheckHealth performs a health check on the external service identified by UUID,
// measures latency, and persists the result.
func (s *ExternalServiceService) CheckHealth(ctx context.Context, svcUUID string, checker HealthChecker) (*model.ExternalService, error) {
	svc, err := s.FindByUUID(ctx, svcUUID)
	if err != nil {
		return nil, fmt.Errorf("finding external service for health check: %w", err)
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

	if err = s.repo.UpdateHealthStatus(ctx, svc.ID, status, latencyMs, lastError); err != nil {
		return nil, fmt.Errorf("updating health status: %w", err)
	}

	updated, err := s.repo.FindByUUID(ctx, svc.UUID)
	if err != nil {
		return nil, fmt.Errorf("re-fetching external service after health check: %w", err)
	}

	return updated, nil
}

