package oauth

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// GenerateUserCode additional tests
//
// Core format, charset, vowel, and uniqueness tests are in token_test.go.
// This file adds boundary and statistical edge cases.
// ---------------------------------------------------------------------------

func TestGenerateUserCode_NoConfusingCharacters(t *testing.T) {
	t.Parallel()

	// Verify that generated codes exclude 0, 1, O, I (in addition to vowels).
	confusing := "01OI"
	for i := range 50 {
		code, err := GenerateUserCode()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		for _, c := range code {
			if c == '-' {
				continue
			}
			if strings.ContainsRune(confusing, c) {
				t.Errorf("code %q contains confusing character %q", code, string(c))
			}
		}
	}
}

func TestGenerateUserCode_AllUppercase(t *testing.T) {
	t.Parallel()

	for i := range 50 {
		code, err := GenerateUserCode()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		stripped := strings.ReplaceAll(code, "-", "")
		if stripped != strings.ToUpper(stripped) {
			t.Errorf("code %q contains lowercase characters", code)
		}
	}
}

func TestGenerateUserCode_BatchUniqueness(t *testing.T) {
	t.Parallel()

	// With 20^8 = 25.6 billion possibilities, 100 codes should never collide.
	seen := make(map[string]bool, 100)
	for i := range 100 {
		code, err := GenerateUserCode()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if seen[code] {
			t.Errorf("duplicate code %q at iteration %d", code, i)
		}
		seen[code] = true
	}
}

// ---------------------------------------------------------------------------
// NormalizeUserCode additional tests
// ---------------------------------------------------------------------------

func TestNormalizeUserCode_OnlyDashes(t *testing.T) {
	t.Parallel()

	got := NormalizeUserCode("---")
	if got != "" {
		t.Errorf("NormalizeUserCode(%q) = %q, want empty string", "---", got)
	}
}

func TestNormalizeUserCode_SingleCharacter(t *testing.T) {
	t.Parallel()

	got := NormalizeUserCode("b")
	if got != "B" {
		t.Errorf("NormalizeUserCode(%q) = %q, want %q", "b", got, "B")
	}
}

func TestNormalizeUserCode_DashAtBoundaries(t *testing.T) {
	t.Parallel()

	got := NormalizeUserCode("-BCDF-")
	if got != "BCDF" {
		t.Errorf("NormalizeUserCode(%q) = %q, want %q", "-BCDF-", got, "BCDF")
	}
}

func TestNormalizeUserCode_NumbersPassThrough(t *testing.T) {
	t.Parallel()

	got := NormalizeUserCode("12-34")
	if got != "1234" {
		t.Errorf("NormalizeUserCode(%q) = %q, want %q", "12-34", got, "1234")
	}
}

func TestNormalizeUserCode_Idempotent(t *testing.T) {
	t.Parallel()

	code, err := GenerateUserCode()
	if err != nil {
		t.Fatalf("GenerateUserCode: %v", err)
	}
	first := NormalizeUserCode(code)
	second := NormalizeUserCode(first)

	if first != second {
		t.Errorf("NormalizeUserCode is not idempotent: %q != %q", first, second)
	}
}

// ---------------------------------------------------------------------------
// deviceCodeCharset validation
// ---------------------------------------------------------------------------

func TestDeviceCodeCharset_Length(t *testing.T) {
	t.Parallel()

	if got := len(deviceCodeCharset); got != 20 {
		t.Errorf("deviceCodeCharset length = %d, want 20", got)
	}
}

func TestDeviceCodeCharset_ExcludesVowels(t *testing.T) {
	t.Parallel()

	for _, vowel := range "AEIOU" {
		if strings.ContainsRune(deviceCodeCharset, vowel) {
			t.Errorf("deviceCodeCharset contains vowel %q", string(vowel))
		}
	}
}

func TestDeviceCodeCharset_OnlyUppercase(t *testing.T) {
	t.Parallel()

	for _, c := range deviceCodeCharset {
		if c < 'A' || c > 'Z' {
			t.Errorf("deviceCodeCharset contains non-uppercase character %q", string(c))
		}
	}
}

func TestDeviceCodeCharset_NoDuplicates(t *testing.T) {
	t.Parallel()

	seen := make(map[rune]bool)
	for _, c := range deviceCodeCharset {
		if seen[c] {
			t.Errorf("deviceCodeCharset contains duplicate character %q", string(c))
		}
		seen[c] = true
	}
}
