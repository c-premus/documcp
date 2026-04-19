package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeMetadataMap(t *testing.T) {
	t.Parallel()

	// 3-byte rune (é encoded U+00E9 is 2 bytes; use U+1F600 for a 4-byte or
	// U+4E2D for 3 bytes). Use U+4E2D "中" (3 bytes) to exercise the
	// rune-boundary walk-back: by placing it so that the first byte sits at
	// offset maxMetadataValueLen-1, truncation must step back one byte to
	// produce a valid UTF-8 tail.
	const threeByteRune = "\u4e2d" // 3 bytes

	// Build a string whose truncation boundary straddles a 3-byte rune:
	// prefix length chosen so the rune starts at maxMetadataValueLen - 1,
	// meaning naive truncation at maxMetadataValueLen would cut mid-rune.
	prefix := strings.Repeat("a", maxMetadataValueLen-1)
	straddle := prefix + threeByteRune + strings.Repeat("b", 10)

	// An over-length safe-ASCII string.
	longASCII := strings.Repeat("x", maxMetadataValueLen+500)

	tests := []struct {
		name string
		in   map[string]any
		want map[string]any
	}{
		{
			name: "nil map returns nil",
			in:   nil,
			want: nil,
		},
		{
			name: "empty map returns empty map",
			in:   map[string]any{},
			want: map[string]any{},
		},
		{
			name: "script tag angle brackets stripped",
			in:   map[string]any{"title": "<script>alert(1)</script>"},
			want: map[string]any{"title": "scriptalert(1)/script"},
		},
		{
			name: "control characters dropped",
			in:   map[string]any{"title": "a\x00b\x01c\x7fd"},
			want: map[string]any{"title": "abcd"},
		},
		{
			name: "newline and tab preserved",
			in:   map[string]any{"description": "line one\nline two\tindented"},
			want: map[string]any{"description": "line one\nline two\tindented"},
		},
		{
			name: "leading and trailing whitespace trimmed",
			in:   map[string]any{"title": "   padded   "},
			want: map[string]any{"title": "padded"},
		},
		{
			name: "oversized ASCII truncated to max length",
			in:   map[string]any{"title": longASCII},
			want: map[string]any{"title": strings.Repeat("x", maxMetadataValueLen)},
		},
		{
			name: "string slice values are each sanitized",
			in: map[string]any{
				"subjects": []string{"science<br>fiction", "history"},
			},
			want: map[string]any{
				"subjects": []string{"sciencebrfiction", "history"},
			},
		},
		{
			name: "any slice with mixed types sanitizes strings and passes scalars",
			in: map[string]any{
				"mixed": []any{"<b>x</b>", 42, true, 3.14},
			},
			want: map[string]any{
				"mixed": []any{"bx/b", 42, true, 3.14},
			},
		},
		{
			name: "nested map recurses",
			in: map[string]any{
				"outer": map[string]any{
					"title": "<em>nested</em>",
					"count": 7,
				},
			},
			want: map[string]any{
				"outer": map[string]any{
					"title": "emnested/em",
					"count": 7,
				},
			},
		},
		{
			name: "non-string scalars pass through unchanged",
			in: map[string]any{
				"pages": 42,
				"ratio": 1.5,
				"final": true,
			},
			want: map[string]any{
				"pages": 42,
				"ratio": 1.5,
				"final": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeMetadataMap(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sanitizeMetadataMap() = %#v, want %#v", got, tt.want)
			}
		})
	}

	// Rune-boundary case: asserts the truncated string is valid UTF-8 and
	// strictly shorter than the input.
	t.Run("oversized string with 3-byte rune at boundary stays valid utf8", func(t *testing.T) {
		t.Parallel()
		got := sanitizeMetadataMap(map[string]any{"title": straddle})
		s, ok := got["title"].(string)
		if !ok {
			t.Fatalf("title was not a string: %T", got["title"])
		}
		if !utf8.ValidString(s) {
			t.Errorf("sanitized string is not valid UTF-8: %q", s)
		}
		if len(s) > maxMetadataValueLen {
			t.Errorf("sanitized length = %d, want <= %d", len(s), maxMetadataValueLen)
		}
		// The straddling 3-byte rune starts at offset maxMetadataValueLen-1,
		// so walk-back must drop it entirely, yielding exactly the prefix.
		if s != prefix {
			t.Errorf("expected walk-back to drop the straddling rune and return the pure prefix")
		}
	})
}

// TestSanitizeMetadataMap_PipelineSeam wires the sanitizer into the exact
// marshal path the document pipeline uses, asserting the on-disk JSONB bytes
// are free of uploader-controlled markup for a representative EPUB-shaped
// payload.
func TestSanitizeMetadataMap_PipelineSeam(t *testing.T) {
	t.Parallel()

	in := map[string]any{
		"title":    "<b>x</b>",
		"subjects": []string{"<script>a</script>"},
	}

	meta, err := json.Marshal(sanitizeMetadataMap(in))
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(meta, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if got["title"] != "bx/b" {
		t.Errorf("title = %v, want %q", got["title"], "bx/b")
	}

	// JSON round-trip produces []any, not []string.
	subj, ok := got["subjects"].([]any)
	if !ok {
		t.Fatalf("subjects type = %T, want []any", got["subjects"])
	}
	if len(subj) != 1 || subj[0] != "scripta/script" {
		t.Errorf("subjects = %v, want [%q]", subj, "scripta/script")
	}
}
