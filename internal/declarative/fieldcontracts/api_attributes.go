package fieldcontracts

import (
	"reflect"
	"slices"

	"github.com/kong/kongctl/internal/declarative/attributes"
	"github.com/kong/kongctl/internal/util/normalizers"
)

// Requests and change diagnostics retain explicit per-key null removals.
// Only comparison canonicalization drops null keys and sorts list values.
func normalizeAPIAttributes(value any) any {
	if normalized, ok := attributes.NormalizeAPIAttributes(value); ok {
		return normalized
	}
	return value
}

func attributesEqual(current, desired any) bool {
	if normalized, ok := attributes.NormalizeAPIAttributes(current); ok {
		current = canonicalizeAttributesForCompare(normalized)
	}
	if normalized, ok := attributes.NormalizeAPIAttributes(desired); ok {
		desired = canonicalizeAttributesForCompare(normalized)
	}

	if current == nil && desired == nil {
		return true
	}

	currentJSON, err := normalizers.SpecToJSON(current)
	if err != nil {
		return reflect.DeepEqual(current, desired)
	}

	desiredJSON, err := normalizers.SpecToJSON(desired)
	if err != nil {
		return reflect.DeepEqual(current, desired)
	}

	return currentJSON == desiredJSON
}

func canonicalizeAttributesForCompare(attrs map[string]any) any {
	canonical := make(map[string]any, len(attrs))
	for key, value := range attrs {
		if value == nil {
			continue
		}
		if values, ok := value.([]string); ok {
			values = slices.Clone(values)
			slices.Sort(values)
			canonical[key] = values
			continue
		}
		canonical[key] = value
	}

	if len(canonical) == 0 {
		return nil
	}

	return canonical
}

func emptyAPIAttributes(v any) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)

	//exhaustive:ignore // Other reflection kinds are not considered empty.
	switch val.Kind() {
	case reflect.Pointer, reflect.Interface:
		if val.IsNil() {
			return true
		}
		return emptyAPIAttributes(val.Elem().Interface())
	case reflect.Map, reflect.Slice, reflect.Array, reflect.Chan, reflect.String:
		return val.Len() == 0
	case reflect.Struct:
		zero := reflect.Zero(val.Type())
		return reflect.DeepEqual(v, zero.Interface())
	}

	return false
}
