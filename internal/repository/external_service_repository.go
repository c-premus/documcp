package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// ExternalServiceRepository handles external service persistence.
type ExternalServiceRepository struct {
	db     *sqlx.DB
	logger *slog.Logger
}

// NewExternalServiceRepository creates a new ExternalServiceRepository.
func NewExternalServiceRepository(db *sqlx.DB, logger *slog.Logger) *ExternalServiceRepository {
	return &ExternalServiceRepository{db: db, logger: logger}
}

// FindEnabledByType returns all enabled external services of the given type.
func (r *ExternalServiceRepository) FindEnabledByType(ctx context.Context, serviceType string) ([]model.ExternalService, error) {
	var services []model.ExternalService
	err := r.db.SelectContext(ctx, &services,
		`SELECT * FROM external_services WHERE type = $1 AND is_enabled = true ORDER BY priority`, serviceType)
	if err != nil {
		return nil, fmt.Errorf("finding enabled external services by type %s: %w", serviceType, err)
	}
	return services, nil
}

// FindByUUID returns an external service by its UUID.
func (r *ExternalServiceRepository) FindByUUID(ctx context.Context, uuid string) (*model.ExternalService, error) {
	var svc model.ExternalService
	err := r.db.GetContext(ctx, &svc,
		`SELECT * FROM external_services WHERE uuid = $1`, uuid)
	if err != nil {
		return nil, fmt.Errorf("finding external service by uuid %s: %w", uuid, err)
	}
	return &svc, nil
}
