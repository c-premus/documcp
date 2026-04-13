package crypto

import (
	"database/sql"
	"fmt"
)

// EncryptNullString encrypts a sql.NullString value for storage.
// Returns the original value unchanged if it is NULL or empty.
// The label parameter is used in error messages (e.g., "api key", "git token").
func EncryptNullString(enc *Encryptor, val sql.NullString, label string) (sql.NullString, error) {
	if !val.Valid || val.String == "" {
		return val, nil
	}
	encrypted, err := enc.Encrypt(val.String)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("encrypting %s: %w", label, err)
	}
	return sql.NullString{String: encrypted, Valid: true}, nil
}
