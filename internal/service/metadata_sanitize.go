package service

import (
	"strings"
	"unicode/utf8"
)

// maxMetadataValueLen caps any single string value stored in
// documents.metadata. Protects the JSONB column from pathological inputs and
// keeps the stored payload within a sensible bound for frontend rendering.
const maxMetadataValueLen = 2048

// sanitizeMetadataMap returns a copy of m with every string value stripped of
// control characters (C0 / DEL), angle brackets, and length-capped at
// maxMetadataValueLen. It recurses into nested map[string]any and iterates
// []string / []any for list-valued fields (e.g. EPUB Dublin Core subjects).
// Non-string scalar types pass through.
//
// Purpose: any future consumer that binds doc.title / doc.creator / etc. in an
// HTML context (v-html, email template) cannot inherit uploader-controlled
// markup. Defense-in-depth; current consumers (Vue text interpolation, MCP
// LLM-facing JSON) are already safe.
func sanitizeMetadataMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = sanitizeMetadataValue(v)
	}
	return out
}

func sanitizeMetadataValue(v any) any {
	switch t := v.(type) {
	case string:
		return sanitizeMetadataString(t)
	case []string:
		out := make([]string, len(t))
		for i, s := range t {
			out[i] = sanitizeMetadataString(s)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = sanitizeMetadataValue(x)
		}
		return out
	case map[string]any:
		return sanitizeMetadataMap(t)
	default:
		return v
	}
}

// sanitizeMetadataString removes control characters and angle brackets and
// truncates to maxMetadataValueLen bytes (respecting rune boundaries).
func sanitizeMetadataString(s string) string {
	if s == "" {
		return ""
	}
	// Truncate early on the source so the loop's max work is bounded.
	if len(s) > maxMetadataValueLen {
		// Step back to a rune boundary to avoid producing an invalid UTF-8 tail.
		cut := maxMetadataValueLen
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut]
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7F:
			// Drop C0 / DEL.
		case r == '<' || r == '>':
			// Drop angle brackets — defense against latent v-html XSS.
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
