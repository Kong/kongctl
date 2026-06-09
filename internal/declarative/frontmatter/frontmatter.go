package frontmatter

import (
	"fmt"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	FieldTitle       = "title"
	FieldDescription = "description"
	FieldSlug        = "slug"
	FieldStatus      = "status"
)

var (
	PortalPageFields    = []string{FieldTitle, FieldDescription}
	PortalSnippetFields = []string{FieldTitle, FieldDescription}
	APIDocumentFields   = []string{FieldTitle, FieldSlug, FieldStatus}
)

// Metadata contains recognized scalar fields from a YAML frontmatter block.
type Metadata map[string]string

// Parse returns recognized scalar metadata from a leading YAML frontmatter block.
func Parse(content string, recognized []string) (Metadata, error) {
	block, ok, err := extractBlock(content)
	if err != nil {
		return nil, err
	}
	if !ok {
		return Metadata{}, nil
	}

	var doc any
	if err := yaml.Unmarshal([]byte(block), &doc); err != nil {
		return nil, fmt.Errorf("invalid YAML frontmatter: %w", err)
	}

	mapping, ok := doc.(map[string]any)
	if !ok {
		return Metadata{}, nil
	}

	recognizedSet := make(map[string]struct{}, len(recognized))
	for _, field := range recognized {
		recognizedSet[field] = struct{}{}
	}

	fields := Metadata{}
	for field, value := range mapping {
		if _, ok := recognizedSet[field]; !ok {
			continue
		}
		scalar, ok := scalarString(value)
		if !ok {
			continue
		}
		fields[field] = scalar
	}

	return fields, nil
}

func scalarString(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case bool:
		return strconv.FormatBool(v), true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case nil:
		return "", false
	default:
		return "", false
	}
}

func extractBlock(content string) (string, bool, error) {
	rest, ok := trimOpeningDelimiter(content)
	if !ok {
		return "", false, nil
	}

	start := 0
	for {
		lineEnd := strings.IndexByte(rest[start:], '\n')
		if lineEnd < 0 {
			line := strings.TrimSuffix(rest[start:], "\r")
			if line == "---" {
				return rest[:start], true, nil
			}
			return "", false, fmt.Errorf("unclosed YAML frontmatter block")
		}

		lineEnd += start
		line := strings.TrimSuffix(rest[start:lineEnd], "\r")
		if line == "---" {
			return rest[:start], true, nil
		}
		start = lineEnd + 1
	}
}

func trimOpeningDelimiter(content string) (string, bool) {
	switch {
	case strings.HasPrefix(content, "---\n"):
		return content[len("---\n"):], true
	case strings.HasPrefix(content, "---\r\n"):
		return content[len("---\r\n"):], true
	default:
		return "", false
	}
}
