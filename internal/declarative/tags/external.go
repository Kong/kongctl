package tags

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // yaml.v3 required for custom tag processing
)

// ExternalPlaceholderPrefix identifies planner-time external lookup placeholders.
const ExternalPlaceholderPrefix = "__EXTERNAL__:"

// ExternalLookup describes a lookup parsed from !external or !lookup.
type ExternalLookup struct {
	MatchFields map[string]string `json:"match_fields"`
	Line        int               `json:"line,omitempty"`
	Column      int               `json:"column,omitempty"`
}

// ExternalTagResolver converts external lookup tags into planner-time placeholders.
type ExternalTagResolver struct {
	tag string
}

// NewExternalTagResolver creates a resolver for !external or its !lookup alias.
func NewExternalTagResolver(tag string) *ExternalTagResolver {
	return &ExternalTagResolver{tag: tag}
}

// Tag returns the YAML tag handled by this resolver.
func (r *ExternalTagResolver) Tag() string {
	return r.tag
}

// Resolve validates and serializes an external lookup without performing network access.
func (r *ExternalTagResolver) Resolve(node *yaml.Node) (any, error) {
	lookup := ExternalLookup{Line: node.Line, Column: node.Column}

	switch node.Kind {
	case yaml.ScalarNode:
		field, value, ok := strings.Cut(node.Value, ":")
		field = strings.TrimSpace(field)
		value = strings.TrimSpace(value)
		if !ok || field == "" || value == "" {
			return nil, fmt.Errorf("%s scalar must use field:value syntax", r.tag)
		}
		lookup.MatchFields = map[string]string{field: value}
	case yaml.MappingNode:
		lookup.MatchFields = make(map[string]string, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			if i+1 >= len(node.Content) {
				return nil, fmt.Errorf("%s mapping is malformed", r.tag)
			}
			key := node.Content[i]
			value := node.Content[i+1]
			if key.Kind != yaml.ScalarNode || value.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("%s mapping keys and values must be strings", r.tag)
			}
			if (key.Tag != "" && key.Tag != "!!str") || (value.Tag != "" && value.Tag != "!!str") {
				return nil, fmt.Errorf("%s mapping keys and values must be strings", r.tag)
			}
			field := strings.TrimSpace(key.Value)
			match := strings.TrimSpace(value.Value)
			if field == "" || match == "" {
				return nil, fmt.Errorf("%s mapping keys and values cannot be empty", r.tag)
			}
			lookup.MatchFields[field] = match
		}
		if len(lookup.MatchFields) == 0 {
			return nil, fmt.Errorf("%s mapping must contain at least one selector", r.tag)
		}
	case yaml.DocumentNode, yaml.SequenceNode, yaml.AliasNode:
		return nil, fmt.Errorf("%s must be used with a field:value scalar or mapping", r.tag)
	default:
		return nil, fmt.Errorf("%s cannot resolve unsupported YAML node kind %d", r.tag, node.Kind)
	}

	if _, hasID := lookup.MatchFields["id"]; hasID && len(lookup.MatchFields) != 1 {
		return nil, fmt.Errorf("%s id cannot be combined with other selectors", r.tag)
	}

	payload, err := json.Marshal(lookup)
	if err != nil {
		return nil, fmt.Errorf("encode external lookup: %w", err)
	}
	return ExternalPlaceholderPrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

// IsExternalPlaceholder reports whether a string contains a planner-time lookup.
func IsExternalPlaceholder(value string) bool {
	return strings.HasPrefix(value, ExternalPlaceholderPrefix)
}

// ParseExternalPlaceholder decodes a planner-time external lookup placeholder.
func ParseExternalPlaceholder(value string) (ExternalLookup, bool) {
	if !IsExternalPlaceholder(value) {
		return ExternalLookup{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, ExternalPlaceholderPrefix))
	if err != nil {
		return ExternalLookup{}, false
	}
	var lookup ExternalLookup
	if err := json.Unmarshal(payload, &lookup); err != nil || len(lookup.MatchFields) == 0 {
		return ExternalLookup{}, false
	}
	return lookup, true
}

// ExternalLookupKey returns a stable selector representation for caching and diagnostics.
func ExternalLookupKey(matchFields map[string]string) string {
	fields := make([]string, 0, len(matchFields))
	for field := range matchFields {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	var b strings.Builder
	for i, field := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%q=%q", field, matchFields[field])
	}
	return b.String()
}
