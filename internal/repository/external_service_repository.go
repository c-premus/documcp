package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/c-premus/documcp/internal/crypto"
	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/model"
)

// ExternalServiceRepository handles external service persistence.
type ExternalServiceRepository struct {
	db        *pgxpool.Pool
	logger    *slog.Logger
	encryptor *crypto.Encryptor // nil disables encryption
}

// NewExternalServiceRepository creates a new ExternalServiceRepository.
func NewExternalServiceRepository(db *pgxpool.Pool, logger *slog.Logger, enc *crypto.Encryptor) *ExternalServiceRepository {
	return &ExternalServiceRepository{db: db, logger: logger, encryptor: enc}
}

// FindAllEnabled returns all enabled external services regardless of type.
func (r *ExternalServiceRepository) FindAllEnabled(ctx context.Context) ([]model.ExternalService, error) {
	services, err := database.Select[model.ExternalService](ctx, r.db,
		`SELECT * FROM external_services WHERE is_enabled = true ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("finding all enabled external services: %w", err)
	}
	r.decryptAPIKeys(services)
	return services, nil
}

// FindEnabledByType returns all enabled external services of the given type.
func (r *ExternalServiceRepository) FindEnabledByType(ctx context.Context, serviceType string) ([]model.ExternalService, error) {
	services, err := database.Select[model.ExternalService](ctx, r.db,
		`SELECT * FROM external_services WHERE type = $1 AND is_enabled = true ORDER BY priority`, serviceType)
	if err != nil {
		return nil, fmt.Errorf("finding enabled external services by type %s: %w", serviceType, err)
	}
	r.decryptAPIKeys(services)
	return services, nil
}

// FindByUUID returns an external service by its UUID.
func (r *ExternalServiceRepository) FindByUUID(ctx context.Context, uuid string) (*model.ExternalService, error) {
	svc, err := database.Get[model.ExternalService](ctx, r.db,
		`SELECT * FROM external_services WHERE uuid = $1`, uuid)
	if err != nil {
		return nil, fmt.Errorf("finding external service by uuid %s: %w", uuid, err)
	}
	r.decryptAPIKey(&svc)
	return &svc, nil
}

// List returns external services with optional type/status filters and pagination.
// Returns the matching services and the total count (before LIMIT/OFFSET).
//
// Query text is stable across all filter combinations so pgx's prepared-statement
// cache hits on repeated admin-list renders. Optional filters are bound as
// typed NULL when absent; the ::text cast is required so PostgreSQL can
// compare the column against the bound value.
func (r *ExternalServiceRepository) List(ctx context.Context, serviceType, status string, limit, offset int) ([]model.ExternalService, int, error) {
	if limit <= 0 {
		limit = 50
	}
	typeArg := nullStr(serviceType)
	statusArg := nullStr(status)

	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM external_services
		WHERE ($1::text IS NULL OR type = $1)
		  AND ($2::text IS NULL OR status = $2)`,
		typeArg, statusArg,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting external services: %w", err)
	}

	services, err := database.Select[model.ExternalService](ctx, r.db,
		`SELECT * FROM external_services
		WHERE ($1::text IS NULL OR type = $1)
		  AND ($2::text IS NULL OR status = $2)
		ORDER BY priority, name
		LIMIT $3 OFFSET $4`,
		typeArg, statusArg, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing external services: %w", err)
	}
	r.decryptAPIKeys(services)

	return services, total, nil
}

// Create inserts a new external service and sets the generated ID on svc.
func (r *ExternalServiceRepository) Create(ctx context.Context, svc *model.ExternalService) error {
	apiKey, err := r.encryptAPIKey(svc.APIKey)
	if err != nil {
		return err
	}
	err = r.db.QueryRow(ctx,
		`INSERT INTO external_services (
			uuid, name, slug, type, base_url, api_key, config,
			priority, status, is_enabled, is_env_managed,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		svc.UUID, svc.Name, svc.Slug, svc.Type, svc.BaseURL, apiKey, svc.Config,
		svc.Priority, svc.Status, svc.IsEnabled, svc.IsEnvManaged,
	).Scan(&svc.ID, &svc.CreatedAt, &svc.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating external service %q: %w", svc.Name, err)
	}
	return nil
}

// Update updates an existing external service by its ID.
func (r *ExternalServiceRepository) Update(ctx context.Context, svc *model.ExternalService) error {
	apiKey, err := r.encryptAPIKey(svc.APIKey)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE external_services SET
			name = $1, slug = $2, base_url = $3, api_key = $4, config = $5,
			priority = $6, is_enabled = $7, updated_at = NOW()
		WHERE id = $8`,
		svc.Name, svc.Slug, svc.BaseURL, apiKey, svc.Config,
		svc.Priority, svc.IsEnabled, svc.ID,
	)
	if err != nil {
		return fmt.Errorf("updating external service %d: %w", svc.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete removes an external service by its ID.
func (r *ExternalServiceRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM external_services WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting external service %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Count returns the total number of external services.
func (r *ExternalServiceRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM external_services`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting external services: %w", err)
	}
	return count, nil
}

// ReorderEntry pairs an external service UUID with its new priority value.
type ReorderEntry struct {
	UUID     string
	Priority int
}

// ErrReorderServiceNotFound is returned when a UUID in the reorder payload
// does not match any external service. Wrapped, so callers should use
// errors.Is to detect it.
var ErrReorderServiceNotFound = errors.New("external service not found")

// ReorderPriorities sets the priority of each external service named in the
// entries slice. Runs in a single transaction; fails fast on any UUID that
// doesn't resolve (wraps ErrReorderServiceNotFound).
func (r *ExternalServiceRepository) ReorderPriorities(ctx context.Context, entries []ReorderEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning reorder transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op; error is irrelevant

	for _, entry := range entries {
		ct, err := tx.Exec(ctx,
			`UPDATE external_services SET priority = $1, updated_at = NOW() WHERE uuid = $2`,
			entry.Priority, entry.UUID,
		)
		if err != nil {
			return fmt.Errorf("updating priority for service %s: %w", entry.UUID, err)
		}
		if ct.RowsAffected() == 0 {
			return fmt.Errorf("%w: %s", ErrReorderServiceNotFound, entry.UUID)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing reorder transaction: %w", err)
	}
	return nil
}

// encryptAPIKey encrypts an API key for storage.
func (r *ExternalServiceRepository) encryptAPIKey(apiKey sql.NullString) (sql.NullString, error) {
	return crypto.EncryptNullString(r.encryptor, apiKey, "api key")
}

// decryptAPIKey decrypts an API key after loading from the database.
func (r *ExternalServiceRepository) decryptAPIKey(svc *model.ExternalService) {
	if !svc.APIKey.Valid || svc.APIKey.String == "" {
		return
	}
	dec, err := r.encryptor.Decrypt(svc.APIKey.String)
	if err != nil {
		r.logger.Warn("decrypting api key (may be plaintext)", "service_id", svc.ID, "error", err)
		return
	}
	svc.APIKey.String = dec
}

// decryptAPIKeys decrypts API keys in a slice of services.
func (r *ExternalServiceRepository) decryptAPIKeys(services []model.ExternalService) {
	for i := range services {
		r.decryptAPIKey(&services[i])
	}
}

// UpdateHealthStatus updates health-related fields for an external service.
// On a healthy status, consecutive_failures resets to 0.
// On an unhealthy status, consecutive_failures increments and error_count increments.
func (r *ExternalServiceRepository) UpdateHealthStatus(ctx context.Context, id int64, status model.ExternalServiceStatus, latencyMs int, lastError string) error {
	_, err := r.db.Exec(ctx,
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
