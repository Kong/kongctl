package tags

import (
	"fmt"
	"reflect"
	"strings"
)

// ExtractValue extracts a value from structured data using dot notation path
// Supports paths like "info.title" or "servers.0.url" (array access in future)
func ExtractValue(data any, path string) (any, error) {
	if path == "" {
		return data, nil
	}

	parts := strings.Split(path, ".") // split once so we can re-use the same slice during traversal
	current := data                    // mutable pointer to where we are in the structure

	for i := 0; i < len(parts); {
		part := parts[i]

		// Convert current to a reflectable value
		val := reflect.ValueOf(current)
		for val.Kind() == reflect.Ptr && !val.IsNil() {
			val = val.Elem()
		}

		switch val.Kind() {
		case reflect.Map:
			// Handle map access
			mapVal := val.MapIndex(reflect.ValueOf(part))
			if !mapVal.IsValid() {
				return nil, fmt.Errorf("path not found: %s (failed at '%s')", path, strings.Join(parts[:i+1], "."))
			}
			current = mapVal.Interface()

		case reflect.Struct:
			// Handle struct field access
			// Try to find field by name (case-insensitive)
			fieldVal := findStructField(val, part)
			if !fieldVal.IsValid() {
				return nil, fmt.Errorf("path not found: %s (failed at '%s')", path, strings.Join(parts[:i+1], "."))
			}
			current = fieldVal.Interface()

		case reflect.Interface:
			// If it's an interface, get the underlying value and retry
			if val.IsNil() {
				return nil, fmt.Errorf("path not found: %s (nil interface at '%s')", path, strings.Join(parts[:i], "."))
			}
			current = val.Interface()
			// Retry this part with the unwrapped value without advancing the index.
			// This avoids wasted assignments and keeps traversal logic consistent.
			continue

		case reflect.Invalid,
			reflect.Bool,
			reflect.Int,
			reflect.Int8,
			reflect.Int16,
			reflect.Int32,
			reflect.Int64,
			reflect.Uint,
			reflect.Uint8,
			reflect.Uint16,
			reflect.Uint32,
			reflect.Uint64,
			reflect.Uintptr,
			reflect.Float32,
			reflect.Float64,
			reflect.Complex64,
			reflect.Complex128,
			reflect.Array,
			reflect.Chan,
			reflect.Func,
			reflect.Pointer,
			reflect.Slice,
			reflect.String,
			reflect.UnsafePointer:
			return nil, fmt.Errorf("cannot traverse path %s: unexpected type %v at '%s'",
				path, val.Kind(), strings.Join(parts[:i], "."))
		}

		i++
	}

	return current, nil
}

// findStructField finds a struct field by name (case-insensitive)
func findStructField(val reflect.Value, fieldName string) reflect.Value {
	typ := val.Type()

	// First try exact match
	if field, ok := typ.FieldByName(fieldName); ok {
		return val.FieldByIndex(field.Index)
	}

	// Try case-insensitive match
	fieldNameLower := strings.ToLower(fieldName)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if strings.ToLower(field.Name) == fieldNameLower {
			return val.Field(i)
		}

		// Also check JSON/YAML tags
		if tag := field.Tag.Get("json"); tag != "" {
			tagName := strings.Split(tag, ",")[0]
			if strings.ToLower(tagName) == fieldNameLower {
				return val.Field(i)
			}
		}
		if tag := field.Tag.Get("yaml"); tag != "" {
			tagName := strings.Split(tag, ",")[0]
			if strings.ToLower(tagName) == fieldNameLower {
				return val.Field(i)
			}
		}
	}

	return reflect.Value{}
}

// GetAvailablePaths returns available paths from a data structure for error messages
func GetAvailablePaths(data any, prefix string, maxDepth int) []string {
	if maxDepth <= 0 {
		return nil
	}

	var paths []string
	val := reflect.ValueOf(data)

	// Dereference pointers
	for val.Kind() == reflect.Ptr && !val.IsNil() {
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Map:
		for _, key := range val.MapKeys() {
			keyStr := fmt.Sprintf("%v", key.Interface())
			fullPath := keyStr
			if prefix != "" {
				fullPath = prefix + "." + keyStr
			}
			paths = append(paths, fullPath)

			// Recursively get paths from map values
			if childPaths := GetAvailablePaths(val.MapIndex(key).Interface(), fullPath, maxDepth-1); len(
				childPaths,
			) > 0 {
				paths = append(paths, childPaths...)
			}
		}

	case reflect.Struct:
		typ := val.Type()
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" { // Skip unexported fields
				continue
			}

			fieldName := field.Name
			// Prefer JSON/YAML tag name if available
			if tag := field.Tag.Get("json"); tag != "" {
				if tagName := strings.Split(tag, ",")[0]; tagName != "" && tagName != "-" {
					fieldName = tagName
				}
			} else if tag := field.Tag.Get("yaml"); tag != "" {
				if tagName := strings.Split(tag, ",")[0]; tagName != "" && tagName != "-" {
					fieldName = tagName
				}
			}

			fullPath := fieldName
			if prefix != "" {
				fullPath = prefix + "." + fieldName
			}
			paths = append(paths, fullPath)

			// Recursively get paths from struct fields
			if childPaths := GetAvailablePaths(val.Field(i).Interface(), fullPath, maxDepth-1); len(childPaths) > 0 {
				paths = append(paths, childPaths...)
			}
		}

	case reflect.Invalid,
		reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Array,
		reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Pointer,
		reflect.Slice,
		reflect.String,
		reflect.UnsafePointer:
		// For other types (slices, arrays, scalars), we can't extract paths.
		// Just return the current path if any.
		if prefix != "" {
			paths = append(paths, prefix)
		}
	}

	return paths
}
