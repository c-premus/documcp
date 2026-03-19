package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// ExternalService represents a row in the "external_services" table.
type ExternalService struct {
	ID                  int64          `db:"id" json:"id"`
	UUID                string         `db:"uuid" json:"uuid"`
	Name                string         `db:"name" json:"name"`
	Slug                string         `db:"slug" json:"slug"`
	Type                string         `db:"type" json:"type"`
	BaseURL             string         `db:"base_url" json:"base_url"`
	APIKey              sql.NullString `db:"api_key" json:"-"`
	Config              sql.NullString `db:"config" json:"config"`
	Priority            int            `db:"priority" json:"priority"`
	Status              string         `db:"status" json:"status"`
	LastCheckAt         sql.NullTime   `db:"last_check_at" json:"last_check_at"`
	LastLatencyMS       sql.NullInt64  `db:"last_latency_ms" json:"last_latency_ms"`
	ErrorCount          int            `db:"error_count" json:"error_count"`
	ConsecutiveFailures int            `db:"consecutive_failures" json:"consecutive_failures"`
	LastError           sql.NullString `db:"last_error" json:"last_error"`
	LastErrorAt         sql.NullTime   `db:"last_error_at" json:"last_error_at"`
	IsEnabled           bool           `db:"is_enabled" json:"is_enabled"`
	IsEnvManaged        bool           `db:"is_env_managed" json:"is_env_managed"`
	CreatedAt           sql.NullTime   `db:"created_at" json:"created_at"`
	UpdatedAt           sql.NullTime   `db:"updated_at" json:"updated_at"`
}

// ParseConfig decodes the JSON config string into the provided destination.
func (es *ExternalService) ParseConfig(dest any) error {
	if !es.Config.Valid {
		return nil
	}
	if err := json.Unmarshal([]byte(es.Config.String), dest); err != nil {
		return fmt.Errorf("unmarshaling external service config: %w", err)
	}
	return nil
}
