package tags

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // yaml.v3 required for custom tag processing
)

// EnvOptions separates storage from value conversion.
type EnvOptions struct {
	Var, Extract, Type string
	Store              bool
}

// StoredEnvTagResolver preserves !env_store until destination types are known.
type StoredEnvTagResolver struct{}

func NewStoredEnvTagResolver() *StoredEnvTagResolver { return &StoredEnvTagResolver{} }
func (*StoredEnvTagResolver) Tag() string            { return TagEnvStore }
func (*StoredEnvTagResolver) Resolve(node *yaml.Node) (any, error) {
	if _, err := ParseEnvOptions(node); err != nil {
		return nil, err
	}
	return node, nil
}

// IsStoredEnvNode reports syntactic opt-in; ParseEnvOptions validates the options.
func IsStoredEnvNode(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Tag == TagEnvStore {
		return true
	}
	if node.Tag != TagEnv {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "store" && node.Content[i+1].Value != "false" {
			return true
		}
	}
	return false
}

// ParseEnvOptions validates both spellings without reading the environment.
func ParseEnvOptions(node *yaml.Node) (EnvOptions, error) {
	opts := EnvOptions{Store: node.Tag == TagEnvStore}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if seen[key.Value] {
				return opts, fmt.Errorf("duplicate !env option %q", key.Value)
			}
			seen[key.Value] = true
			switch key.Value {
			case "store":
				if value.Tag != "!!bool" || (value.Value != "true" && value.Value != "false") {
					return opts, fmt.Errorf("!env store must be a boolean true or false")
				}
				opts.Store = value.Value == "true"
				if node.Tag == TagEnvStore && !opts.Store {
					return opts, fmt.Errorf("!env_store conflicts with store: false")
				}
			case "type", "var", "extract":
				if value.Kind != yaml.ScalarNode || (value.Tag != "!!str" && value.Tag != "") {
					return opts, fmt.Errorf("!env %s must be a string (quote the type name \"null\")", key.Value)
				}
				if key.Value == "type" {
					opts.Type = value.Value
				}
			default:
				return opts, fmt.Errorf("unsupported !env option %q", key.Value)
			}
		}
		if seen["type"] {
			if !opts.Store {
				return opts, fmt.Errorf("!env type requires store: true")
			}
			if !slices.Contains(
				[]string{"string", "boolean", "integer", "number", "null", "array", "object"},
				opts.Type,
			) {
				return opts, fmt.Errorf("unsupported !env type %q; "+
					"expected string, boolean, integer, number, null, array, or object", opts.Type)
			}
		}
	}
	var err error
	opts.Var, opts.Extract, err = parseEnvNode(node)
	return opts, err
}

// ResolveStoredEnv returns a JSON-compatible value using a selected conversion type.
func ResolveStoredEnv(opts EnvOptions, selected string) (any, error) {
	raw, exists := os.LookupEnv(opts.Var)
	if !exists {
		return nil, fmt.Errorf("environment variable not set: %s", opts.Var)
	}
	if selected == "string" && opts.Extract == "" {
		if err := validateStoredString(raw); err != nil {
			return nil, fmt.Errorf("environment variable %s: %w", opts.Var, err)
		}
		return raw, nil
	}
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("environment variable %s is empty; expected %s", opts.Var, selected)
	}
	decoder := yaml.NewDecoder(strings.NewReader(raw))
	var document, extra yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("environment variable %s contains invalid YAML/JSON", opts.Var)
	}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("environment variable %s must contain one YAML/JSON document", opts.Var)
	}
	value, err := storedNodeValue(document.Content[0])
	if err != nil {
		return nil, fmt.Errorf("environment variable %s: %w", opts.Var, err)
	}
	if opts.Extract != "" {
		value, err = ExtractValue(value, opts.Extract)
		if err != nil {
			return nil, fmt.Errorf("environment variable %s extraction %q: %w", opts.Var, opts.Extract, err)
		}
	}
	actual := storedValueType(value)
	if selected == "integer" && actual == "number" {
		n := value.(float64)
		if math.Trunc(n) == n {
			value, actual = int64(n), "integer"
		}
	}
	if selected != actual && (selected != "number" || actual != "integer") {
		return nil, fmt.Errorf("environment variable %s: expected %s, got %s", opts.Var, selected, actual)
	}
	return value, nil
}

func storedValueType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case int64:
		return "integer"
	case float64:
		return "number"
	case []any:
		return "array"
	default:
		return "object"
	}
}

func validateStoredString(value string) error {
	for _, prefix := range []string{
		EnvPlaceholderPrefix, SecretPlaceholderPrefix, RefPlaceholderPrefix, ExternalPlaceholderPrefix,
	} {
		if strings.HasPrefix(value, prefix) {
			return fmt.Errorf("stored strings cannot begin with reserved placeholder prefix %s", prefix)
		}
	}
	return nil
}

var storedNumberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

func storedNodeValue(node *yaml.Node) (any, error) {
	if node.Anchor != "" || node.Kind == yaml.AliasNode {
		return nil, fmt.Errorf("stored values do not support YAML anchors or aliases")
	}
	if (node.Tag == "!!map" && node.Kind != yaml.MappingNode) ||
		(node.Tag == "!!seq" && node.Kind != yaml.SequenceNode) ||
		(node.Tag != "!!map" && node.Tag != "!!seq" && node.Kind != yaml.ScalarNode) {
		return nil, fmt.Errorf("stored YAML tag does not match its value kind")
	}
	switch node.Tag {
	case "!!map":
		result := map[string]any{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Tag != "!!str" || key.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("stored object keys must be strings")
			}
			if _, exists := result[key.Value]; exists {
				return nil, fmt.Errorf("stored objects cannot contain duplicate keys")
			}
			value, err := storedNodeValue(node.Content[i+1])
			if err != nil {
				return nil, err
			}
			result[key.Value] = value
		}
		return result, nil
	case "!!seq":
		result := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := storedNodeValue(child)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		return result, nil
	case "!!str":
		return node.Value, validateStoredString(node.Value)
	case "!!null":
		if node.Value != "" && node.Value != "~" && !strings.EqualFold(node.Value, "null") {
			return nil, fmt.Errorf("invalid stored null value")
		}
		return nil, nil
	case "!!bool":
		if !strings.EqualFold(node.Value, "true") && !strings.EqualFold(node.Value, "false") {
			return nil, fmt.Errorf("invalid stored boolean; expected true or false")
		}
		return strings.EqualFold(node.Value, "true"), nil
	case "!!int", "!!float":
		if !storedNumberPattern.MatchString(node.Value) {
			return nil, fmt.Errorf("stored numbers must use finite JSON decimal syntax")
		}
		n, err := strconv.ParseFloat(node.Value, 64)
		if err != nil || math.IsInf(n, 0) {
			return nil, fmt.Errorf("stored number is outside the supported binary64 range")
		}
		if n == 0 {
			mantissa := strings.FieldsFunc(node.Value, func(r rune) bool { return r == 'e' || r == 'E' })[0]
			if strings.Trim(mantissa, "-0.") != "" {
				return nil, fmt.Errorf("stored number is outside the supported binary64 range")
			}
			if node.Tag == "!!int" {
				return int64(0), nil
			}
			return n, nil
		}
		exact, ok := new(big.Rat).SetString(node.Value)
		if !ok {
			return nil, fmt.Errorf("invalid stored number")
		}
		limit := big.NewRat(9007199254740991, 1)
		if new(big.Rat).Abs(exact).Cmp(limit) > 0 {
			return nil, fmt.Errorf("stored numbers must be within ±(2^53-1)")
		}
		if node.Tag == "!!int" {
			if !exact.IsInt() {
				return nil, fmt.Errorf("invalid stored integer value")
			}
			return exact.Num().Int64(), nil
		}
		if !exact.IsInt() && math.Trunc(n) == n {
			return nil, fmt.Errorf("stored number loses its fractional part in binary64")
		}
		return n, nil
	default:
		return nil, fmt.Errorf("unsupported stored YAML tag %s; use JSON-compatible values", node.Tag)
	}
}
