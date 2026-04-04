// Package testutil provides builder functions that create model instances
// with sensible defaults for use in tests. Each builder accepts functional
// options to override individual fields.
package testutil

import (
	"database/sql"
	"time"
)

// nullString returns a valid sql.NullString.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

// nullInt64 returns a valid sql.NullInt64.
func nullInt64(n int64) sql.NullInt64 {
	return sql.NullInt64{Int64: n, Valid: true}
}

// nullTime returns a valid sql.NullTime for the given time.
func nullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}
