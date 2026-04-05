package tags

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"         //nolint:gomodguard // yaml.v3 required for custom tag processing
	k8syaml "sigs.k8s.io/yaml" // For JSON/YAML extraction support
)

// EnvPlaceholderPrefix is the special prefix used to serialize deferred !env values.
const EnvPlaceholderPrefix = "__ENV__:"

// EnvTagMode controls whether !env resolves to a concrete value or a deferred placeholder.
type EnvTagMode int

const (
	// EnvTagModeResolve resolves !env tags to their current environment variable values.
	EnvTagModeResolve EnvTagMode = iota
	// EnvTagModePlaceholder preserves !env tags as deferred placeholders.
	EnvTagModePlaceholder
)

// EnvTagResolver handles !env tags for loading environment-backed values.
type EnvTagResolver struct {
	mode EnvTagMode
}

// NewEnvTagResolver creates a new env tag resolver.
func NewEnvTagResolver(mode EnvTagMode) *EnvTagResolver {
	return &EnvTagResolver{mode: mode}
}

// Tag returns the YAML tag this resolver handles.
func (r *EnvTagResolver) Tag() string {
	return "!env"
}

// Resolve processes a YAML node with the !env tag.
func (r *EnvTagResolver) Resolve(node *yaml.Node) (any, error) {
	varRef, extractPath, err := parseEnvNode(node)
	if err != nil {
		return nil, err
	}

	if r.mode == EnvTagModePlaceholder {
		return BuildEnvPlaceholder(varRef, extractPath), nil
	}

	return resolveEnvValue(varRef, extractPath)
}

func parseEnvNode(node *yaml.Node) (string, string, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		varRef := node.Value
		extractPath := ""
		if before, after, found := strings.Cut(varRef, "#"); found {
			extractPath = after
			varRef = before
		}
		if strings.TrimSpace(varRef) == "" {
			return "", "", fmt.Errorf("!env tag requires an environment variable name")
		}
		if extractPath == "" && strings.HasSuffix(node.Value, "#") {
			return "", "", fmt.Errorf("!env tag extract path cannot be empty after #")
		}
		return strings.TrimSpace(varRef), strings.TrimSpace(extractPath), nil
	case yaml.MappingNode:
		var envRef EnvRef
		if err := node.Decode(&envRef); err != nil {
			return "", "", fmt.Errorf("invalid !env tag format: %w", err)
		}
		if strings.TrimSpace(envRef.Var) == "" {
			return "", "", fmt.Errorf("!env tag requires 'var' field")
		}
		return strings.TrimSpace(envRef.Var), strings.TrimSpace(envRef.Extract), nil
	default:
		return "", "", fmt.Errorf("!env tag must be used with a string or map, got %v", node.Kind)
	}
}

func resolveEnvValue(varRef, extractPath string) (any, error) {
	value, ok := os.LookupEnv(varRef)
	if !ok {
		return nil, fmt.Errorf("environment variable not set: %s", varRef)
	}

	if extractPath == "" {
		return value, nil
	}

	var parsed any
	if err := k8syaml.Unmarshal([]byte(value), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse environment variable %s: %w", varRef, err)
	}

	result, err := ExtractValue(parsed, extractPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract '%s' from environment variable %s: %w", extractPath, varRef, err)
	}

	return result, nil
}

// BuildEnvPlaceholder serializes an env reference into a deferred placeholder string.
func BuildEnvPlaceholder(varRef, extractPath string) string {
	if extractPath == "" {
		return EnvPlaceholderPrefix + varRef
	}
	return fmt.Sprintf("%s%s#%s", EnvPlaceholderPrefix, varRef, extractPath)
}

// IsEnvPlaceholder checks if a string contains a deferred !env placeholder.
func IsEnvPlaceholder(value string) bool {
	return strings.HasPrefix(value, EnvPlaceholderPrefix)
}

// ParseEnvPlaceholder extracts the variable name and extract path from a deferred !env placeholder.
func ParseEnvPlaceholder(placeholder string) (varRef, extractPath string, ok bool) {
	if !IsEnvPlaceholder(placeholder) {
		return "", "", false
	}

	raw := strings.TrimPrefix(placeholder, EnvPlaceholderPrefix)
	if raw == "" {
		return "", "", false
	}
	if before, after, found := strings.Cut(raw, "#"); found {
		return before, after, true
	}
	return raw, "", true
}

// ResolveEnvPlaceholder resolves a deferred !env placeholder using the current environment.
func ResolveEnvPlaceholder(placeholder string) (string, error) {
	varRef, extractPath, ok := ParseEnvPlaceholder(placeholder)
	if !ok {
		return "", fmt.Errorf("invalid env placeholder: %s", placeholder)
	}

	value, err := resolveEnvValue(varRef, extractPath)
	if err != nil {
		return "", err
	}

	strValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("!env value must resolve to a string for deferred execution")
	}

	return strValue, nil
}
