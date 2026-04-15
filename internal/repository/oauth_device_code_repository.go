package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/model"
)

//nolint:godot // ---------------------------------------------------------------------------
// Device Codes.
//nolint:godot // ---------------------------------------------------------------------------

// CreateDeviceCode inserts a new device code and sets the generated ID and timestamps.
func (r *OAuthRepository) CreateDeviceCode(ctx context.Context, dc *model.OAuthDeviceCode) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO oauth_device_codes (
			device_code, user_code, client_id, user_id, scope, resource,
			verification_uri, verification_uri_complete, interval,
			last_polled_at, status, expires_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9,
			$10, $11, $12,
			NOW(), NOW()
		) RETURNING id, created_at, updated_at`,
		dc.DeviceCode, dc.UserCode, dc.ClientID, dc.UserID, dc.Scope, dc.Resource,
		dc.VerificationURI, dc.VerificationURIComplete, dc.Interval,
		dc.LastPolledAt, dc.Status, dc.ExpiresAt,
	).Scan(&dc.ID, &dc.CreatedAt, &dc.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating device code: %w", err)
	}
	return nil
}

// FindDeviceCodeByDeviceCode returns a device code by its hash.
func (r *OAuthRepository) FindDeviceCodeByDeviceCode(ctx context.Context, deviceCodeHash string) (*model.OAuthDeviceCode, error) {
	dc, err := database.Get[model.OAuthDeviceCode](ctx, r.db,
		`SELECT * FROM oauth_device_codes WHERE device_code = $1 AND expires_at > NOW()`, deviceCodeHash)
	if err != nil {
		return nil, fmt.Errorf("finding device code: %w", err)
	}
	return &dc, nil
}

// FindDeviceCodeByUserCode returns a pending, non-expired device code by its user code.
// The comparison normalizes the user code by removing dashes and ignoring case.
func (r *OAuthRepository) FindDeviceCodeByUserCode(ctx context.Context, userCode string) (*model.OAuthDeviceCode, error) {
	dc, err := database.Get[model.OAuthDeviceCode](ctx, r.db,
		`SELECT * FROM oauth_device_codes
		WHERE UPPER(REPLACE(user_code, '-', '')) = UPPER(REPLACE($1, '-', ''))
			AND status = 'pending' AND expires_at > NOW()`, userCode)
	if err != nil {
		return nil, fmt.Errorf("finding device code by user code: %w", err)
	}
	return &dc, nil
}

// UpdateDeviceCodeStatus updates the status and optionally the user_id of a device code.
func (r *OAuthRepository) UpdateDeviceCodeStatus(ctx context.Context, id int64, status model.DeviceCodeStatus, userID *int64) error {
	var err error
	if userID != nil {
		_, err = r.db.Exec(ctx,
			`UPDATE oauth_device_codes SET status = $1, user_id = $2, updated_at = NOW() WHERE id = $3`,
			status, *userID, id)
	} else {
		_, err = r.db.Exec(ctx,
			`UPDATE oauth_device_codes SET status = $1, updated_at = NOW() WHERE id = $2`,
			status, id)
	}
	if err != nil {
		return fmt.Errorf("updating device code %d status to %s: %w", id, status, err)
	}
	return nil
}

// ExchangeDeviceCodeStatus atomically transitions a device code from authorized to exchanged.
// Returns sql.ErrNoRows if the code is not in authorized state (already consumed or wrong state).
func (r *OAuthRepository) ExchangeDeviceCodeStatus(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE oauth_device_codes SET status = $1, updated_at = NOW()
		WHERE id = $2 AND status = $3`,
		model.DeviceCodeStatusExchanged, id, model.DeviceCodeStatusAuthorized)
	if err != nil {
		return fmt.Errorf("exchanging device code %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("device code %d not in authorized state: %w", id, sql.ErrNoRows)
	}
	return nil
}

// UpdateDeviceCodeLastPolled updates the last_polled_at timestamp and polling interval.
func (r *OAuthRepository) UpdateDeviceCodeLastPolled(ctx context.Context, id int64, interval int) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE oauth_device_codes SET last_polled_at = NOW(), interval = $1, updated_at = NOW() WHERE id = $2`,
		interval, id)
	if err != nil {
		return fmt.Errorf("updating device code %d last polled: %w", id, err)
	}
	n := tag.RowsAffected()
	if n == 0 {
		return fmt.Errorf("device code %d not found: %w", id, sql.ErrNoRows)
	}
	return nil
}

// UpdateDeviceCodeStatusAndScope atomically updates status, user_id, and scope of a device code.
func (r *OAuthRepository) UpdateDeviceCodeStatusAndScope(ctx context.Context, id int64, status model.DeviceCodeStatus, userID *int64, scope string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE oauth_device_codes SET status = $1, user_id = $2, scope = $3, updated_at = NOW() WHERE id = $4`,
		status, userID, sql.NullString{String: scope, Valid: scope != ""}, id)
	if err != nil {
		return fmt.Errorf("updating device code %d status+scope: %w", id, err)
	}
	return nil
}
