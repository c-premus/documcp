package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"

	"github.com/c-premus/documcp/internal/config"
	"github.com/c-premus/documcp/internal/crypto"
	"github.com/c-premus/documcp/internal/database"
)

// RunRekey re-encrypts every stored at-rest ciphertext under the current
// ENCRYPTION_KEY. Rows whose ciphertext is already carrying the primary key's
// version byte are skipped. Legacy (pre-versioning) rows and rows written
// under ENCRYPTION_KEY_PREVIOUS are decrypted in-place and written back.
//
// Safe to run repeatedly — idempotent when every row already matches the
// primary. When encryption is disabled (ENCRYPTION_KEY empty) the command
// prints a hint and exits with an error rather than silently no-op'ing.
func RunRekey(cfg *config.Config) error {
	logger := newLogger(cfg.App.Env, cfg.App.Debug, os.Stdout)

	if len(cfg.App.EncryptionKeyBytes) == 0 {
		return errors.New("ENCRYPTION_KEY not set — rekey is a no-op and refuses to run to make misconfiguration visible")
	}

	enc, err := newEncryptor(cfg, logger)
	if err != nil {
		return fmt.Errorf("building encryptor: %w", err)
	}
	if enc == nil {
		// Defensive: newEncryptor returned nil despite a non-empty key.
		return errors.New("encryptor unexpectedly nil")
	}

	pool, err := database.NewPgxPool(
		context.Background(),
		cfg.DatabaseDSN(),
		cfg.Database.MaxOpenConns,
		cfg.Database.PgxMinConns,
		cfg.Database.PgxMaxConnLifetime,
		cfg.Database.PgxMaxConnIdleTime,
	)
	if err != nil {
		return fmt.Errorf("initializing database pool: %w", err)
	}
	defer pool.Close()

	ctx := context.Background()
	var total rekeyReport

	for _, target := range []struct {
		label  string
		table  string
		column string
	}{
		{"external service api_key", "external_services", "api_key"},
		{"git template git_token", "git_templates", "git_token"},
	} {
		report, rekeyErr := rekeyColumn(ctx, pool, enc, target.table, target.column)
		if rekeyErr != nil {
			return fmt.Errorf("rekeying %s: %w", target.label, rekeyErr)
		}
		logger.Info("rekey pass complete",
			"target", target.label,
			"scanned", report.Scanned,
			"rekeyed", report.Rekeyed,
			"skipped_already_primary", report.Skipped,
			"errors", report.Errors,
		)
		total.Scanned += report.Scanned
		total.Rekeyed += report.Rekeyed
		total.Skipped += report.Skipped
		total.Errors += report.Errors
	}

	logger.Info("rekey complete",
		"scanned", total.Scanned,
		"rekeyed", total.Rekeyed,
		"skipped_already_primary", total.Skipped,
		"errors", total.Errors,
	)
	if total.Errors > 0 {
		return fmt.Errorf("rekey finished with %d row errors — inspect logs", total.Errors)
	}
	return nil
}

// RekeyReport counts what a single rekey pass did.
type RekeyReport struct {
	Scanned int
	Rekeyed int
	Skipped int
	Errors  int
}

type rekeyReport = RekeyReport

// RekeyColumn re-encrypts every non-empty ciphertext in table.column under
// the primary key of enc. Rows already under the primary are skipped.
// Exported so integration tests can drive it directly against a real pool.
func RekeyColumn(ctx context.Context, pool database.Querier, enc *crypto.Encryptor, table, column string) (RekeyReport, error) {
	return rekeyColumn(ctx, pool, enc, table, column)
}

// rekeyColumn streams every non-empty ciphertext in table.column and rewrites
// rows where the stored ciphertext isn't already under the primary key. Uses
// one transaction per row so a mid-run failure leaves earlier rows upgraded
// without rolling back progress.
func rekeyColumn(ctx context.Context, pool database.Querier, enc *crypto.Encryptor, table, column string) (rekeyReport, error) {
	// #nosec G201 -- table and column are code-local constants, not user input.
	selectSQL := fmt.Sprintf("SELECT id, %s FROM %s WHERE %s IS NOT NULL AND %s != ''", column, table, column, column)
	rows, err := pool.Query(ctx, selectSQL)
	if err != nil {
		return rekeyReport{}, fmt.Errorf("querying %s: %w", table, err)
	}

	type row struct {
		ID         int64
		Ciphertext sql.NullString
	}
	collected, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var out row
		if scanErr := r.Scan(&out.ID, &out.Ciphertext); scanErr != nil {
			return row{}, scanErr
		}
		return out, nil
	})
	if err != nil {
		return rekeyReport{}, fmt.Errorf("scanning %s: %w", table, err)
	}

	// #nosec G201 -- table and column are code-local constants.
	updateSQL := fmt.Sprintf("UPDATE %s SET %s = $1, updated_at = NOW() WHERE id = $2", table, column)
	report := rekeyReport{}
	for _, r := range collected {
		report.Scanned++
		if !r.Ciphertext.Valid || r.Ciphertext.String == "" {
			continue
		}
		if !enc.NeedsRekey(r.Ciphertext.String) {
			report.Skipped++
			continue
		}

		plain, decErr := enc.Decrypt(r.Ciphertext.String)
		if decErr != nil {
			report.Errors++
			continue
		}
		fresh, encErr := enc.Encrypt(plain)
		if encErr != nil {
			report.Errors++
			continue
		}
		if _, updateErr := pool.Exec(ctx, updateSQL, fresh, r.ID); updateErr != nil {
			report.Errors++
			continue
		}
		report.Rekeyed++
	}
	return report, nil
}
