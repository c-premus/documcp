package repository

import (
	"context"
	"database/sql"
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

// FindAllEnabled returns all enabled external services regardless of type.
func (r *ExternalServiceRepository) FindAllEnabled(ctx context.Context) ([]model.ExternalService, error) {
	var services []model.ExternalService
	err := r.db.SelectContext(ctx, &services,
		`SELECT * FROM external_services WHERE is_enabled = true ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("finding all enabled external services: %w", err)
	}
	return services, nil
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

// FindBySlug returns an external service by its slug.
func (r *ExternalServiceRepository) FindBySlug(ctx context.Context, slug string) (*model.ExternalService, error) {
	var svc model.ExternalService
	err := r.db.GetContext(ctx, &svc,
		`SELECT * FROM external_services WHERE slug = $1`, slug)
	if err != nil {
		return nil, fmt.Errorf("finding external service by slug %s: %w", slug, err)
	}
	return &svc, nil
}

// List returns external services with optional type/status filters and pagination.
// Returns the matching services and the total count (before LIMIT/OFFSET).
func (r *ExternalServiceRepository) List(ctx context.Context, serviceType, status string, limit, offset int) ([]model.ExternalService, int, error) {
	where := ``
	args := []any{}
	argIdx := 1

	if serviceType != "" {
		where += fmt.Sprintf(` AND type = $%d`, argIdx)
		args = append(args, serviceType)
		argIdx++
	}

	if status != "" {
		where += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	// Count total matching rows.
	countQuery := `SELECT COUNT(*) FROM external_services WHERE 1=1` + where
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("counting external services: %w", err)
	}

	// Fetch paginated results.
	if limit <= 0 {
		limit = 50
	}

	q := `SELECT * FROM external_services WHERE 1=1` + where + ` ORDER BY priority, name`
	q += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var services []model.ExternalService
	if err := r.db.SelectContext(ctx, &services, q, args...); err != nil {
		return nil, 0, fmt.Errorf("listing external services: %w", err)
	}

	return services, total, nil
}

// Create inserts a new external service and sets the generated ID on svc.
func (r *ExternalServiceRepository) Create(ctx context.Context, svc *model.ExternalService) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO external_services (
			uuid, name, slug, type, base_url, api_key, config,
			priority, status, is_enabled, is_env_managed,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		svc.UUID, svc.Name, svc.Slug, svc.Type, svc.BaseURL, svc.APIKey, svc.Config,
		svc.Priority, svc.Status, svc.IsEnabled, svc.IsEnvManaged,
	).Scan(&svc.ID, &svc.CreatedAt, &svc.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating external service %q: %w", svc.Name, err)
	}
	return nil
}

// Update updates an existing external service by its ID.
func (r *ExternalServiceRepository) Update(ctx context.Context, svc *model.ExternalService) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE external_services SET
			name = $1, slug = $2, base_url = $3, api_key = $4, config = $5,
			priority = $6, is_enabled = $7, updated_at = NOW()
		WHERE id = $8`,
		svc.Name, svc.Slug, svc.BaseURL, svc.APIKey, svc.Config,
		svc.Priority, svc.IsEnabled, svc.ID,
	)
	if err != nil {
		return fmt.Errorf("updating external service %d: %w", svc.ID, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete removes an external service by its ID.
func (r *ExternalServiceRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM external_services WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting external service %d: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Count returns the total number of external services.
func (r *ExternalServiceRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM external_services`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting external services: %w", err)
	}
	return count, nil
}

// ReorderPriorities updates the priority column for each service based on
// its position in the provided ID slice (index 0 = priority 0, etc.).
func (r *ExternalServiceRepository) ReorderPriorities(ctx context.Context, serviceIDs []int64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning reorder transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for priority, id := range serviceIDs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE external_services SET priority = $1, updated_at = NOW() WHERE id = $2`,
			priority, id,
		); err != nil {
			return fmt.Errorf("updating priority for service %d: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing reorder transaction: %w", err)
	}
	return nil
}

// UpdateHealthStatus updates health-related fields for an external service.
// On a healthy status, consecutive_failures resets to 0.
// On an unhealthy status, consecutive_failures increments and error_count increments.
func (r *ExternalServiceRepository) UpdateHealthStatus(ctx context.Context, id int64, status string, latencyMs int, lastError string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE external_services SET
			status = $1,
			last_latency_ms = $2,
			last_check_at = NOW(),
			last_error = CASE WHEN $3 = 'unhealthy' THEN $4 ELSE last_error END,
			last_error_at = CASE WHEN $5 = 'unhealthy' THEN NOW() ELSE last_error_at END,
			consecutive_failures = CASE WHEN $6 = 'healthy' THEN 0 ELSE consecutive_failures + 1 END,
			error_count = CASE WHEN $7 = 'unhealthy' THEN error_count + 1 ELSE error_count END,
			updated_at = NOW()
		WHERE id = $8`,
		status, latencyMs, status, lastError, status, status, status, id,
	)
	if err != nil {
		return fmt.Errorf("updating health status for external service %d: %w", id, err)
	}
	return nil
}
