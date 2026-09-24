package loader

import (
	"encoding"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // yaml.v3 required for custom tag processing
)

func resolveStoredEnvContent(content []byte) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, err
	}
	if err := resolveStoredEnvNode(&document, []reflect.Type{reflect.TypeFor[temporaryParseResult]()}, nil); err != nil {
		return nil, err
	}
	return yaml.Marshal(&document)
}

func resolveStoredEnvNode(node *yaml.Node, destinations []reflect.Type, path []string) error {
	if tags.IsStoredEnvNode(node) {
		if err := resolveStoredEnvValue(node, destinations); err != nil {
			return fmt.Errorf("line %d, column %d, field %s: %w", node.Line, node.Column, strings.Join(path, "."), err)
		}
		return nil
	}
	// Other tag resolvers own their nested source semantics.
	if strings.HasPrefix(node.Tag, "!") && !strings.HasPrefix(node.Tag, "!!") {
		return nil
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := resolveStoredEnvNode(child, destinations, path); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if tags.IsStoredEnvNode(node.Content[i+1]) && storedIdentityField(destinations, key) {
				return fmt.Errorf("stored environment tags are not supported on refs or kongctl.namespace (field %s)",
					strings.Join(append(path, key), "."))
			}
			childTypes := storedChildTypes(destinations, key, false)
			if err := resolveStoredEnvNode(node.Content[i+1], childTypes, append(path, key)); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		childTypes := storedChildTypes(destinations, "", true)
		for i, child := range node.Content {
			if err := resolveStoredEnvNode(child, childTypes, append(path, strconv.Itoa(i))); err != nil {
				return err
			}
		}
	case yaml.ScalarNode, yaml.AliasNode:
	}
	return nil
}

func resolveStoredEnvValue(node *yaml.Node, destinations []reflect.Type) error {
	opts, err := tags.ParseEnvOptions(node)
	if err != nil {
		return err
	}
	inferred := storedDestinationKind(destinations)
	selected := opts.Type
	if selected == "" {
		selected = inferred
	}
	if selected == "" {
		return fmt.Errorf("cannot infer type for stored environment variable %s: "+
			"destination is unknown or ambiguous; "+
			"specify type: string, boolean, integer, number, null, array, or object", opts.Var)
	}
	if inferred != "" && selected != inferred && (selected != "integer" || inferred != "number") &&
		(selected != "null" || !storedDestinationsNullable(destinations)) {
		return fmt.Errorf(
			"environment variable %s: explicit type %s conflicts with destination type %s",
			opts.Var,
			selected,
			inferred,
		)
	}
	value, err := tags.ResolveStoredEnv(opts, selected)
	if err != nil {
		return err
	}
	// The SDK decoder remains authoritative for destination validation, including
	// narrower integer ranges, enum values, and custom JSON representations.
	return node.Encode(value)
}

func storedIdentityField(parents []reflect.Type, key string) bool {
	for _, parent := range parents {
		parent = derefType(parent)
		if parent == nil {
			continue
		}
		if key == "ref" && reflect.PointerTo(parent).Implements(reflect.TypeFor[resources.Resource]()) {
			return true
		}
		if key == "namespace" && (parent == reflect.TypeFor[resources.KongctlMeta]() ||
			parent == reflect.TypeFor[resources.KongctlMetaDefaults]()) {
			return true
		}
	}
	return false
}

func storedDestinationsNullable(types []reflect.Type) bool {
	for _, typ := range types {
		if typ == nil {
			continue
		}
		//exhaustive:ignore
		switch typ.Kind() {
		case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface:
		default:
			return false
		}
	}
	return true
}

func storedDestinationKind(types []reflect.Type) string {
	result := ""
	for _, typ := range types {
		typ = derefType(typ)
		if typ == nil {
			return ""
		}
		if typ.Kind() == reflect.Struct &&
			reflect.PointerTo(typ).Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) {
			return ""
		}
		var kind string
		//exhaustive:ignore
		switch typ.Kind() {
		case reflect.String:
			kind = "string"
		case reflect.Bool:
			kind = "boolean"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			kind = "integer"
		case reflect.Float32, reflect.Float64:
			kind = "number"
		case reflect.Slice, reflect.Array:
			// []byte has a JSON string representation, not an array.
			if typ.Elem().Kind() == reflect.Uint8 {
				return ""
			}
			kind = "array"
		case reflect.Map:
			kind = "object"
		case reflect.Struct:
			// SDK unions may represent scalars as well as objects. Don't guess.
			for field := range typ.Fields() {
				if field.Tag.Get("union") == "member" {
					return ""
				}
			}
			kind = "object"
		default:
			return ""
		}
		if result != "" && result != kind {
			return ""
		}
		result = kind
	}
	return result
}

func storedChildTypes(parents []reflect.Type, key string, sequence bool) []reflect.Type {
	var result []reflect.Type
	for _, parent := range parents {
		parent = derefType(parent)
		if parent == nil {
			result = append(result, nil)
			continue
		}
		if sequence {
			if parent.Kind() == reflect.Slice || parent.Kind() == reflect.Array {
				result = append(result, parent.Elem())
			} else {
				result = append(result, nil)
			}
			continue
		}
		//exhaustive:ignore
		switch parent.Kind() {
		case reflect.Map:
			result = append(result, parent.Elem())
		case reflect.Struct:
			matches := storedStructFieldTypes(parent, key, nil)
			if len(matches) == 0 {
				matches = []reflect.Type{nil}
			}
			result = append(result, matches...)
		default:
			result = append(result, nil)
		}
	}
	return result
}

// Collect all candidate union fields rather than selecting the first branch.
// Unknown/custom JSON layouts require an explicit type instead of field tables.
func storedStructFieldTypes(typ reflect.Type, key string, stack []reflect.Type) []reflect.Type {
	typ = derefType(typ)
	if typ == nil || typ.Kind() != reflect.Struct || slices.Contains(stack, typ) {
		return nil
	}
	stack = append(stack, typ)
	var direct, embedded []reflect.Type
	for field := range typ.Fields() {
		if field.PkgPath != "" || field.Tag.Get("json") == "-" {
			continue
		}
		if field.Tag.Get("union") == "member" || field.Anonymous || structuredFieldInline(field) {
			embedded = append(embedded, storedStructFieldTypes(field.Type, key, stack)...)
		} else if matchesStructuredFieldName(field, key) {
			direct = append(direct, field.Type)
		}
	}
	if len(direct) > 0 {
		return direct
	}
	return embedded
}
