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

// ParseVariablesJSON decodes a JSON string into a map of variable substitutions.
// Returns an empty map if the input is empty.
func ParseVariablesJSON(raw string) (map[string]string, error) {
	if raw == "" {
		return map[string]string{}, nil
	}
	var vars map[string]string
	if err := json.Unmarshal([]byte(raw), &vars); err != nil {
		return nil, fmt.Errorf("decoding variables JSON: %w", err)
	}
	return vars, nil
}
