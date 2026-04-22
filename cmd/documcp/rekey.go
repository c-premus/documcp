package main

import (
	"github.com/spf13/cobra"

	"github.com/c-premus/documcp/internal/app"
)

var rekeyCmd = &cobra.Command{
	Use:   "rekey",
	Short: "Re-encrypt stored secrets under the current ENCRYPTION_KEY",
	Long: `Re-encrypts every encrypted column (external_services.api_key,
git_templates.git_token) under the current ENCRYPTION_KEY. Rows whose
ciphertext is already under the primary key are skipped.

Intended to be run after rotating ENCRYPTION_KEY. Steps:
  1. Generate a new key:            openssl rand -hex 32
  2. Set ENCRYPTION_KEY_PREVIOUS to the old value; ENCRYPTION_KEY to the new
  3. Restart the server — reads keep working under the retired key
  4. Run: documcp rekey             # upgrades rows to the new primary
  5. Drop ENCRYPTION_KEY_PREVIOUS on the next deploy

Safe to run repeatedly — idempotent when every row already matches the
primary. Exits non-zero if ENCRYPTION_KEY is empty so misconfiguration
surfaces visibly rather than as a silent no-op.

Examples:
  documcp rekey`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return app.RunRekey(cfg)
	},
}

func init() {
	rootCmd.AddCommand(rekeyCmd)
}
