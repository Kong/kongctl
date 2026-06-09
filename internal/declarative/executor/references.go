package executor

import (
	"strconv"
	"strings"

	"github.com/kong/kongctl/internal/declarative/planner"
)

func setResolvedFieldValue(fields map[string]any, fieldPath string, value any) bool {
	if fields == nil || fieldPath == "" {
		return false
	}

	if current, exists := fields[fieldPath]; exists {
		fields[fieldPath] = resolvedFieldReplacement(current, value)
		return true
	}

	_, ok := setResolvedFieldPathValue(fields, strings.Split(fieldPath, "."), value)
	return ok
}

func resolvedFieldReplacement(current any, value any) any {
	if fieldChange, ok := current.(planner.FieldChange); ok {
		fieldChange.New = value
		return fieldChange
	}
	return value
}

func setResolvedFieldPathValue(current any, segments []string, value any) (any, bool) {
	if len(segments) == 0 {
		return current, false
	}

	switch typed := current.(type) {
	case planner.FieldChange:
		updated, ok := setResolvedFieldPathValue(typed.New, segments, value)
		if !ok {
			return current, false
		}
		typed.New = updated
		return typed, true
	case map[string]any:
		child, ok := typed[segments[0]]
		if !ok {
			return current, false
		}
		if len(segments) == 1 {
			typed[segments[0]] = resolvedFieldReplacement(child, value)
			return typed, true
		}

		updated, ok := setResolvedFieldPathValue(child, segments[1:], value)
		if !ok {
			return current, false
		}
		typed[segments[0]] = updated
		return typed, true
	case []any:
		index, err := strconv.Atoi(segments[0])
		if err != nil || index < 0 || index >= len(typed) {
			return current, false
		}
		if len(segments) == 1 {
			typed[index] = resolvedFieldReplacement(typed[index], value)
			return typed, true
		}

		updated, ok := setResolvedFieldPathValue(typed[index], segments[1:], value)
		if !ok {
			return current, false
		}
		typed[index] = updated
		return typed, true
	default:
		return current, false
	}
}
