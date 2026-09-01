package tags

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // yaml.v3 required for custom tag processing
)

// SecretPlaceholderPrefix identifies serialized deferred secret expressions.
const SecretPlaceholderPrefix = "__SECRET__:"

// SecretSource describes one deferred source in a secret expression.
type SecretSource struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Extract   string `json:"extract,omitempty"`
	RootDir   string `json:"root_dir,omitempty"`
}

// SecretPart is either public literal text or a deferred source.
type SecretPart struct {
	Literal *string       `json:"literal,omitempty"`
	Source  *SecretSource `json:"source,omitempty"`
}

// SecretExpression is an ordered sequence whose resolved value is sensitive.
type SecretExpression struct {
	Parts []SecretPart `json:"parts"`
}

// SecretTagResolver preserves !secret declarations for execution-time resolution.
type SecretTagResolver struct {
	fileResolver *FileTagResolver
}

// NewSecretTagResolver creates a !secret resolver.
func NewSecretTagResolver() *SecretTagResolver {
	return &SecretTagResolver{}
}

// NewSecretTagResolverWithFileScope creates a secret resolver that can preserve
// deferred !file sources without reading their contents.
func NewSecretTagResolverWithFileScope(baseDir, rootDir string) *SecretTagResolver {
	return &SecretTagResolver{fileResolver: NewFileTagResolver(baseDir, rootDir)}
}

// Tag returns the YAML tag handled by this resolver.
func (r *SecretTagResolver) Tag() string {
	return TagSecret
}

// Resolve validates and serializes a deferred secret expression.
func (r *SecretTagResolver) Resolve(node *yaml.Node) (any, error) {
	expression, err := r.parseSecretExpression(node)
	if err != nil {
		return nil, err
	}
	return BuildSecretPlaceholder(expression)
}

func (r *SecretTagResolver) parseSecretExpression(node *yaml.Node) (SecretExpression, error) {
	if node == nil || node.Kind != yaml.MappingNode {
		return SecretExpression{}, fmt.Errorf("!secret must be a map containing exactly one of 'source' or 'parts'")
	}

	var sourceNode, partsNode *yaml.Node
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}
		key := node.Content[i]
		value := node.Content[i+1]
		switch key.Value {
		case "source":
			if sourceNode != nil {
				return SecretExpression{}, fmt.Errorf("!secret source may be specified only once")
			}
			sourceNode = value
		case "parts":
			if partsNode != nil {
				return SecretExpression{}, fmt.Errorf("!secret parts may be specified only once")
			}
			partsNode = value
		default:
			return SecretExpression{}, fmt.Errorf("!secret does not support field %q", key.Value)
		}
	}

	if (sourceNode == nil) == (partsNode == nil) {
		return SecretExpression{}, fmt.Errorf("!secret requires exactly one of 'source' or 'parts'")
	}
	if sourceNode != nil {
		source, err := r.parseSecretSource(sourceNode)
		if err != nil {
			return SecretExpression{}, err
		}
		return SecretExpression{Parts: []SecretPart{{Source: &source}}}, nil
	}

	if partsNode.Kind != yaml.SequenceNode || len(partsNode.Content) == 0 {
		return SecretExpression{}, fmt.Errorf("!secret parts must be a non-empty sequence")
	}

	parts := make([]SecretPart, 0, len(partsNode.Content))
	hasSource := false
	for _, partNode := range partsNode.Content {
		if isCustomTag(partNode.Tag) {
			source, err := r.parseSecretSource(partNode)
			if err != nil {
				return SecretExpression{}, err
			}
			parts = append(parts, SecretPart{Source: &source})
			hasSource = true
			continue
		}
		if partNode.Kind != yaml.ScalarNode || partNode.Tag != "!!str" {
			return SecretExpression{}, fmt.Errorf("!secret parts must contain only strings or deferred sources")
		}
		literal := partNode.Value
		parts = append(parts, SecretPart{Literal: &literal})
	}
	if !hasSource {
		return SecretExpression{}, fmt.Errorf("!secret parts must contain at least one deferred source")
	}
	return SecretExpression{Parts: parts}, nil
}

func (r *SecretTagResolver) parseSecretSource(node *yaml.Node) (SecretSource, error) {
	if node == nil {
		return SecretSource{}, fmt.Errorf("!secret source is required")
	}
	switch node.Tag {
	case TagEnv:
		varRef, extractPath, err := parseEnvNode(node)
		if err != nil {
			return SecretSource{}, err
		}
		return SecretSource{Kind: "env", Reference: varRef, Extract: extractPath}, nil
	case TagFile:
		if r.fileResolver == nil {
			return SecretSource{}, fmt.Errorf("!secret !file source requires file scope")
		}
		path, extractPath, err := parseDeferredFileNode(node)
		if err != nil {
			return SecretSource{}, err
		}
		if err := r.fileResolver.validatePath(path); err != nil {
			return SecretSource{}, err
		}
		resolvedPath := r.fileResolver.resolvePath(path)
		if err := r.fileResolver.validateResolvedPath(path, resolvedPath); err != nil {
			return SecretSource{}, err
		}
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err != nil {
			return SecretSource{}, fmt.Errorf("failed to resolve path %s: %w", path, err)
		}
		if realPath, realErr := filepath.EvalSymlinks(resolvedPath); realErr == nil {
			resolvedPath = realPath
		}
		rootDir := r.fileResolver.rootDirReal
		if rootDir == "" {
			rootDir = r.fileResolver.rootDirAbs
		}
		return SecretSource{
			Kind: "file", Reference: filepath.Clean(resolvedPath), Extract: extractPath, RootDir: filepath.Clean(rootDir),
		}, nil
	default:
		return SecretSource{}, fmt.Errorf("!secret currently supports only !env and !file sources")
	}
}

func parseDeferredFileNode(node *yaml.Node) (string, string, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		path, extractPath := node.Value, ""
		if before, after, ok := strings.Cut(path, "#"); ok {
			path, extractPath = before, after
		}
		if strings.TrimSpace(path) == "" {
			return "", "", fmt.Errorf("!file tag requires a path")
		}
		return path, extractPath, nil
	case yaml.MappingNode:
		var ref FileRef
		if err := node.Decode(&ref); err != nil {
			return "", "", fmt.Errorf("invalid !file tag format: %w", err)
		}
		if strings.TrimSpace(ref.Path) == "" {
			return "", "", fmt.Errorf("!file tag requires 'path' field")
		}
		return ref.Path, ref.Extract, nil
	case yaml.DocumentNode, yaml.SequenceNode, yaml.AliasNode:
		return "", "", fmt.Errorf("!file tag must be used with a string or map, got %v", node.Kind)
	}
	return "", "", fmt.Errorf("!file tag has unsupported node kind %v", node.Kind)
}

// BuildSecretPlaceholder serializes a secret expression without resolving it.
func BuildSecretPlaceholder(expression SecretExpression) (string, error) {
	encoded, err := json.Marshal(expression)
	if err != nil {
		return "", fmt.Errorf("failed to encode !secret expression: %w", err)
	}
	return SecretPlaceholderPrefix + base64.RawURLEncoding.EncodeToString(encoded), nil
}

// IsSecretPlaceholder reports whether a value is a serialized secret expression.
func IsSecretPlaceholder(value string) bool {
	return strings.HasPrefix(value, SecretPlaceholderPrefix)
}

// ParseSecretPlaceholder decodes a serialized secret expression.
func ParseSecretPlaceholder(value string) (SecretExpression, error) {
	if !IsSecretPlaceholder(value) {
		return SecretExpression{}, fmt.Errorf("invalid deferred secret expression")
	}
	encoded := strings.TrimPrefix(value, SecretPlaceholderPrefix)
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return SecretExpression{}, fmt.Errorf("invalid deferred secret expression encoding: %w", err)
	}
	var expression SecretExpression
	if err := json.Unmarshal(data, &expression); err != nil {
		return SecretExpression{}, fmt.Errorf("invalid deferred secret expression: %w", err)
	}
	if len(expression.Parts) == 0 {
		return SecretExpression{}, fmt.Errorf("invalid empty deferred secret expression")
	}
	return expression, nil
}

// SecretExpressionFromEnvPlaceholder converts a legacy deferred !env value.
func SecretExpressionFromEnvPlaceholder(value string) (SecretExpression, error) {
	varRef, extractPath, ok := ParseEnvPlaceholder(value)
	if !ok {
		return SecretExpression{}, fmt.Errorf("invalid deferred environment reference")
	}
	return SecretExpression{Parts: []SecretPart{{Source: &SecretSource{
		Kind:      "env",
		Reference: varRef,
		Extract:   extractPath,
	}}}}, nil
}

// ResolveSecretExpression resolves and concatenates all expression parts.
func ResolveSecretExpression(expression SecretExpression) (string, error) {
	var result strings.Builder
	for _, part := range expression.Parts {
		switch {
		case part.Literal != nil && part.Source == nil:
			result.WriteString(*part.Literal)
		case part.Source != nil && part.Literal == nil:
			value, err := resolveSecretSource(*part.Source)
			if err != nil {
				return "", err
			}
			if value == "" {
				return "", fmt.Errorf("secret source %s %s resolved to an empty value", part.Source.Kind, part.Source.Reference)
			}
			result.WriteString(value)
		default:
			return "", fmt.Errorf("invalid secret expression part")
		}
	}
	if result.Len() == 0 {
		return "", fmt.Errorf("secret expression resolved to an empty value")
	}
	return result.String(), nil
}

func resolveSecretSource(source SecretSource) (string, error) {
	switch source.Kind {
	case "env":
		return resolveEnvStringValue(source.Reference, source.Extract)
	case "file":
		if source.RootDir == "" || !filepath.IsAbs(source.Reference) {
			return "", fmt.Errorf("invalid deferred file source")
		}
		resolvedPath, err := filepath.EvalSymlinks(source.Reference)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("secret source file not found: %s", source.Reference)
			}
			return "", fmt.Errorf("failed to resolve secret source file %s: %w", source.Reference, err)
		}
		rootDir := source.RootDir
		if realRoot, rootErr := filepath.EvalSymlinks(source.RootDir); rootErr == nil {
			rootDir = realRoot
		}
		if !pathWithinBase(filepath.Clean(rootDir), filepath.Clean(resolvedPath)) {
			return "", fmt.Errorf("secret source file resolves outside base dir %s: %s", rootDir, source.Reference)
		}
		resolver := NewFileTagResolver(filepath.Dir(resolvedPath), rootDir)
		data, err := resolver.readFile(resolvedPath)
		if err != nil {
			return "", err
		}
		value, err := resolver.parseContent(resolvedPath, data)
		if err != nil {
			return "", fmt.Errorf("failed to parse %s: %w", source.Reference, err)
		}
		if source.Extract != "" {
			value, err = ExtractValue(value, source.Extract)
			if err != nil {
				return "", fmt.Errorf("failed to extract %q from %s: %w", source.Extract, source.Reference, err)
			}
		}
		stringValue, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("secret source file %s did not resolve to a string", source.Reference)
		}
		return stringValue, nil
	default:
		return "", fmt.Errorf("unsupported secret source kind %q", source.Kind)
	}
}
