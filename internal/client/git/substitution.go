package git

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SubstituteVariables replaces {{key}} placeholders in content using the
// provided variable map. It returns the substituted content and a list of
// any unresolved variable names.
func SubstituteVariables(content string, variables map[string]string) (result string, missingVars []string) {
	result = content

	matches := VariablePattern.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	for _, match := range matches {
		key := match[1]
		if val, ok := variables[key]; ok {
			result = strings.ReplaceAll(result, "{{"+key+"}}", val)
		} else if !seen[key] {
			missingVars = append(missingVars, key)
			seen[key] = true
		}
	}
	return result, missingVars
}

// maxVariableValueLen limits individual variable values to prevent memory
// amplification when a variable appears multiple times in a template.
const maxVariableValueLen = 10 * 1024 // 10 KiB

// ValidateVariables enforces the per-value size cap. Used by callers that
// receive the variables map directly (e.g. the MCP tools, where the schema
// already shapes the payload as an object).
func ValidateVariables(vars map[string]string) error {
	for k, v := range vars {
		if len(v) > maxVariableValueLen {
			return fmt.Errorf("variable %q value exceeds maximum length of %d bytes", k, maxVariableValueLen)
		}
	}
	return nil
}

// ParseVariablesJSON decodes a JSON string into a map of variable substitutions.
// Returns an empty map if the input is empty. Rejects values exceeding 10 KiB.
// Still used by the REST API where variables arrive JSON-encoded on the query
// string.
func ParseVariablesJSON(raw string) (map[string]string, error) {
	if raw == "" {
		return map[string]string{}, nil
	}
	var vars map[string]string
	if err := json.Unmarshal([]byte(raw), &vars); err != nil {
		return nil, fmt.Errorf("decoding variables JSON: %w", err)
	}
	if err := ValidateVariables(vars); err != nil {
		return nil, err
	}
	return vars, nil
}
