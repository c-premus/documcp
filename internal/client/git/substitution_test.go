package git

import (
	"testing"
)

// ---------------------------------------------------------------------------
// SubstituteVariables tests
// ---------------------------------------------------------------------------

func TestSubstituteVariables(t *testing.T) {
	t.Parallel()

	t.Run("substitutes single variable", func(t *testing.T) {
		t.Parallel()

		content := "Hello, {{name}}!"
		vars := map[string]string{"name": "World"}

		got, unresolved := SubstituteVariables(content, vars)

		if got != "Hello, World!" {
			t.Errorf("content = %q, want %q", got, "Hello, World!")
		}
		if len(unresolved) != 0 {
			t.Errorf("unresolved = %v, want empty", unresolved)
		}
	})

	t.Run("substitutes multiple variables", func(t *testing.T) {
		t.Parallel()

		content := "{{greeting}}, {{name}}! Welcome to {{place}}."
		vars := map[string]string{
			"greeting": "Hello",
			"name":     "Alice",
			"place":    "Wonderland",
		}

		got, unresolved := SubstituteVariables(content, vars)

		want := "Hello, Alice! Welcome to Wonderland."
		if got != want {
			t.Errorf("content = %q, want %q", got, want)
		}
		if len(unresolved) != 0 {
			t.Errorf("unresolved = %v, want empty", unresolved)
		}
	})

	t.Run("tracks unresolved variables", func(t *testing.T) {
		t.Parallel()

		content := "{{found}} and {{missing}}"
		vars := map[string]string{"found": "yes"}

		got, unresolved := SubstituteVariables(content, vars)

		if got != "yes and {{missing}}" {
			t.Errorf("content = %q, want %q", got, "yes and {{missing}}")
		}
		if len(unresolved) != 1 || unresolved[0] != "missing" {
			t.Errorf("unresolved = %v, want [missing]", unresolved)
		}
	})

	t.Run("deduplicates unresolved variables", func(t *testing.T) {
		t.Parallel()

		content := "{{x}} and {{x}} and {{x}}"
		vars := map[string]string{}

		_, unresolved := SubstituteVariables(content, vars)

		if len(unresolved) != 1 {
			t.Errorf("unresolved count = %d, want 1 (deduped)", len(unresolved))
		}
		if len(unresolved) > 0 && unresolved[0] != "x" {
			t.Errorf("unresolved[0] = %q, want %q", unresolved[0], "x")
		}
	})

	t.Run("no variables in content", func(t *testing.T) {
		t.Parallel()

		content := "plain text with no placeholders"
		vars := map[string]string{"key": "value"}

		got, unresolved := SubstituteVariables(content, vars)

		if got != content {
			t.Errorf("content = %q, want %q", got, content)
		}
		if len(unresolved) != 0 {
			t.Errorf("unresolved = %v, want empty", unresolved)
		}
	})

	t.Run("empty content", func(t *testing.T) {
		t.Parallel()

		got, unresolved := SubstituteVariables("", map[string]string{"key": "val"})

		if got != "" {
			t.Errorf("content = %q, want empty", got)
		}
		if len(unresolved) != 0 {
			t.Errorf("unresolved = %v, want empty", unresolved)
		}
	})

	t.Run("empty variables map", func(t *testing.T) {
		t.Parallel()

		content := "{{a}} and {{b}}"
		got, unresolved := SubstituteVariables(content, map[string]string{})

		if got != content {
			t.Errorf("content = %q, want %q", got, content)
		}
		if len(unresolved) != 2 {
			t.Errorf("unresolved count = %d, want 2", len(unresolved))
		}
	})

	t.Run("replaces same variable multiple times in content", func(t *testing.T) {
		t.Parallel()

		content := "{{x}}-{{x}}-{{x}}"
		vars := map[string]string{"x": "A"}

		got, unresolved := SubstituteVariables(content, vars)

		if got != "A-A-A" {
			t.Errorf("content = %q, want %q", got, "A-A-A")
		}
		if len(unresolved) != 0 {
			t.Errorf("unresolved = %v, want empty", unresolved)
		}
	})

	t.Run("variable substituted with empty string", func(t *testing.T) {
		t.Parallel()

		content := "prefix-{{var}}-suffix"
		vars := map[string]string{"var": ""}

		got, unresolved := SubstituteVariables(content, vars)

		if got != "prefix--suffix" {
			t.Errorf("content = %q, want %q", got, "prefix--suffix")
		}
		if len(unresolved) != 0 {
			t.Errorf("unresolved = %v, want empty", unresolved)
		}
	})

	t.Run("mixed resolved and unresolved", func(t *testing.T) {
		t.Parallel()

		content := "host={{host}} port={{port}} db={{db}}"
		vars := map[string]string{"host": "localhost", "port": "5432"}

		got, unresolved := SubstituteVariables(content, vars)

		if got != "host=localhost port=5432 db={{db}}" {
			t.Errorf("content = %q, want %q", got, "host=localhost port=5432 db={{db}}")
		}
		if len(unresolved) != 1 || unresolved[0] != "db" {
			t.Errorf("unresolved = %v, want [db]", unresolved)
		}
	})
}

// ---------------------------------------------------------------------------
// ParseVariablesJSON tests
// ---------------------------------------------------------------------------

func TestParseVariablesJSON(t *testing.T) {
	t.Parallel()

	t.Run("valid JSON with key-value pairs", func(t *testing.T) {
		t.Parallel()

		raw := `{"name":"Alice","age":"30"}`
		vars, err := ParseVariablesJSON(raw)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vars["name"] != "Alice" {
			t.Errorf("vars[name] = %q, want Alice", vars["name"])
		}
		if vars["age"] != "30" {
			t.Errorf("vars[age] = %q, want 30", vars["age"])
		}
	})

	t.Run("empty string returns empty map", func(t *testing.T) {
		t.Parallel()

		vars, err := ParseVariablesJSON("")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vars == nil {
			t.Fatal("vars should not be nil")
		}
		if len(vars) != 0 {
			t.Errorf("vars length = %d, want 0", len(vars))
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()

		_, err := ParseVariablesJSON("{not valid json}")

		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("empty JSON object returns empty map", func(t *testing.T) {
		t.Parallel()

		vars, err := ParseVariablesJSON("{}")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vars) != 0 {
			t.Errorf("vars length = %d, want 0", len(vars))
		}
	})

	t.Run("nested JSON returns error", func(t *testing.T) {
		t.Parallel()

		raw := `{"key":{"nested":"value"}}`
		_, err := ParseVariablesJSON(raw)

		if err == nil {
			t.Error("expected error for nested JSON, got nil")
		}
	})

	t.Run("JSON array returns error", func(t *testing.T) {
		t.Parallel()

		_, err := ParseVariablesJSON(`["a","b"]`)

		if err == nil {
			t.Error("expected error for JSON array, got nil")
		}
	})

	t.Run("JSON with numeric value returns error", func(t *testing.T) {
		t.Parallel()

		_, err := ParseVariablesJSON(`{"key":123}`)

		if err == nil {
			t.Error("expected error for non-string value, got nil")
		}
	})

	t.Run("single key-value pair", func(t *testing.T) {
		t.Parallel()

		vars, err := ParseVariablesJSON(`{"host":"localhost"}`)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vars) != 1 {
			t.Errorf("vars length = %d, want 1", len(vars))
		}
		if vars["host"] != "localhost" {
			t.Errorf("vars[host] = %q, want localhost", vars["host"])
		}
	})

	t.Run("values with special characters", func(t *testing.T) {
		t.Parallel()

		raw := `{"url":"https://example.com/path?q=1&r=2","path":"/usr/local/bin"}`
		vars, err := ParseVariablesJSON(raw)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vars["url"] != "https://example.com/path?q=1&r=2" {
			t.Errorf("vars[url] = %q, want https://example.com/path?q=1&r=2", vars["url"])
		}
		if vars["path"] != "/usr/local/bin" {
			t.Errorf("vars[path] = %q, want /usr/local/bin", vars["path"])
		}
	})
}
