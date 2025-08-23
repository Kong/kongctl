package normalizers

import (
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

const Indentation = `  `

type source struct {
	string
}

// LongDesc normalizes a command's long description following
// a convention
func LongDesc(s string) string {
	return source{s}.trim().string
}

// Examples normalizes a command's examples following
// a convention
func Examples(s string) string {
	if len(s) == 0 {
		return s
	}
	return source{s}.trim().indent().string
}

func (s source) trim() source {
	s.string = strings.TrimSpace(s.string)
	return s
}

func (s source) indent() source {
	indentedLines := []string{}
	for _, line := range strings.Split(s.string, "\n") {
		trimmed := strings.TrimSpace(line)
		indented := Indentation + trimmed
		indentedLines = append(indentedLines, indented)
	}
	s.string = strings.Join(indentedLines, "\n")
	return s
}

// SpecToJSON converts a spec (YAML/JSON string or object) to normalized JSON string
func SpecToJSON(spec interface{}) (string, error) {
	var data interface{}

	switch v := spec.(type) {
	case string:
		// Try to parse as YAML (which also handles JSON)
		if err := yaml.Unmarshal([]byte(v), &data); err != nil {
			return "", fmt.Errorf("failed to parse spec: %w", err)
		}
	case map[string]interface{}:
		data = v
	default:
		// For any other type, use it directly
		data = spec
	}

	// Convert to compact JSON
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal spec to JSON: %w", err)
	}

	return string(jsonBytes), nil
}
