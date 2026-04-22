package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// expandTagsParam flattens a repeated ?tags=a&tags=b list AND a single
// comma-delimited ?tags=a,b into a single de-duplicated []string. Empty
// entries are skipped. Callers expecting AND-logic tag filtering see the
// union of the two accepted shapes.
func expandTagsParam(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, value := range raw {
		for token := range strings.SplitSeq(value, ",") {
			t := strings.TrimSpace(token)
			if t == "" {
				continue
			}
			if _, dup := seen[t]; dup {
				continue
			}
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	return out
}

// parsePagination extracts limit and offset query parameters with bounds enforcement.
func parsePagination(r *http.Request, defaultLimit, maxLimit int) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// parsePaginationParam is like parsePagination but reads the limit from a custom query parameter.
func parsePaginationParam(r *http.Request, limitParam string, defaultLimit, maxLimit int) (limit, offset int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get(limitParam))
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// listResponse builds the standard paginated list envelope.
func listResponse(data any, total, limit, offset int) map[string]any {
	return map[string]any{
		"data": data,
		"meta": map[string]any{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	}
}

// parseIDParam extracts and validates a chi URL parameter as int64.
// Returns the parsed ID and true on success, or writes an error response and returns false.
func parseIDParam(w http.ResponseWriter, r *http.Request, param, label string) (int64, bool) { //nolint:unparam // param is generic by design
	raw := chi.URLParam(r, param)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		errorResponse(w, http.StatusBadRequest, "invalid "+label)
		return 0, false
	}
	return id, true
}

// nullTimeToString formats a sql.NullTime as RFC3339, or returns empty string if null.
func nullTimeToString(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}

// nullTimePtr formats a sql.NullTime as a *string (RFC3339) for optional JSON fields.
func nullTimePtr(t sql.NullTime) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.UTC().Format(time.RFC3339)
	return &s
}

// nullStringValue returns the string value from a sql.NullString, or empty string if null.
func nullStringValue(s sql.NullString) string {
	if !s.Valid {
		return ""
	}
	return s.String
}

// nullStringPtr returns a *string from a sql.NullString — nil when invalid,
// so the JSON encoder serializes the field as `null` rather than an empty string.
func nullStringPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

// nullInt64Value returns the int64 value from a sql.NullInt64, or 0 if null.
func nullInt64Value(n sql.NullInt64) int64 {
	if !n.Valid {
		return 0
	}
	return n.Int64
}
