package tags

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3" //nolint:gomodguard // yaml.v3 required for custom tag processing
)

// RefPlaceholderPrefix is the special prefix for serialized placeholders
const RefPlaceholderPrefix = "__REF__:"

// RefTagResolver handles !ref tags for resource references
type RefTagResolver struct {
	baseDir string
}

// NewRefTagResolver creates a new ref tag resolver
func NewRefTagResolver(baseDir string) *RefTagResolver {
	return &RefTagResolver{
		baseDir: baseDir,
	}
}

// Tag returns the YAML tag this resolver handles
func (r *RefTagResolver) Tag() string {
	return "!ref"
}

// Resolve processes a YAML node with the !ref tag
func (r *RefTagResolver) Resolve(node *yaml.Node) (any, error) {
	// Only support scalar nodes
	if node.Kind != yaml.ScalarNode {
		return nil, fmt.Errorf("!ref tag must be used with a string, got %v", node.Kind)
	}

	// Parse ref syntax: resource-ref#field
	refStr := node.Value
	resourceRef := refStr
	field := "id" // default field

	if idx := strings.Index(refStr, "#"); idx != -1 {
		field = refStr[idx+1:]
		resourceRef = refStr[:idx]
	}

	// Validate
	if resourceRef == "" {
		return nil, fmt.Errorf("!ref tag requires a resource reference")
	}
	if field == "" {
		return nil, fmt.Errorf("!ref tag field cannot be empty after #")
	}

	// Return serialized placeholder string
	return fmt.Sprintf("%s%s#%s", RefPlaceholderPrefix, resourceRef, field), nil
}

// IsRefPlaceholder checks if a string is a reference placeholder
func IsRefPlaceholder(value string) bool {
	return strings.HasPrefix(value, RefPlaceholderPrefix)
}

// ParseRefPlaceholder extracts ref and field from a placeholder string
func ParseRefPlaceholder(placeholder string) (resourceRef, field string, ok bool) {
	if !IsRefPlaceholder(placeholder) {
		return "", "", false
	}

	refPart := strings.TrimPrefix(placeholder, RefPlaceholderPrefix)
	if idx := strings.Index(refPart, "#"); idx != -1 {
		return refPart[:idx], refPart[idx+1:], true
	}
	return "", "", false
}
