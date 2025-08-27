package common

import (
	"fmt"
	"reflect"
)

// MapOptionalStringField maps a string field from source map to target pointer if present
func MapOptionalStringField(target *string, source map[string]any, key string) {
	if value, exists := source[key]; exists {
		if strValue, ok := value.(string); ok {
			*target = strValue
		}
	}
}

// MapOptionalBoolField maps a boolean field from source map to target pointer if present
func MapOptionalBoolField(target *bool, source map[string]any, key string) {
	if value, exists := source[key]; exists {
		if boolValue, ok := value.(bool); ok {
			*target = boolValue
		}
	}
}

// MapOptionalIntField maps an integer field from source map to target pointer if present
func MapOptionalIntField(target *int, source map[string]any, key string) {
	if value, exists := source[key]; exists {
		switch v := value.(type) {
		case int:
			*target = v
		case float64:
			*target = int(v)
		}
	}
}

// MapOptionalSliceField maps a slice field from source map to target pointer if present
func MapOptionalSliceField(target *[]string, source map[string]any, key string) {
	if value, exists := source[key]; exists {
		if sliceValue, ok := value.([]any); ok {
			stringSlice := make([]string, len(sliceValue))
			for i, item := range sliceValue {
				if strItem, ok := item.(string); ok {
					stringSlice[i] = strItem
				}
			}
			*target = stringSlice
		} else if sliceValue, ok := value.([]string); ok {
			*target = sliceValue
		}
	}
}

// ExtractResourceName extracts the resource name from fields map
func ExtractResourceName(fields map[string]any) string {
	if name, ok := fields["name"].(string); ok {
		return name
	}
	return "[unknown]"
}

// ExtractResourceID extracts the resource ID from fields map
func ExtractResourceID(fields map[string]any) string {
	if id, ok := fields["id"].(string); ok {
		return id
	}
	return ""
}

// HasFieldChanged checks if a field value has changed between old and new maps
func HasFieldChanged(oldFields, newFields map[string]any, key string) bool {
	oldValue, oldExists := oldFields[key]
	newValue, newExists := newFields[key]

	// If existence changed, it's a change
	if oldExists != newExists {
		return true
	}

	// If neither exists, no change
	if !oldExists && !newExists {
		return false
	}

	// Compare values using deep equality
	return !reflect.DeepEqual(oldValue, newValue)
}

// CopyField copies a field from source to destination map if it exists
func CopyField(dest, src map[string]any, key string) {
	if value, exists := src[key]; exists {
		dest[key] = value
	}
}

// ValidateRequiredFields checks that all required fields are present and non-empty
func ValidateRequiredFields(fields map[string]any, requiredFields []string) error {
	for _, field := range requiredFields {
		value, exists := fields[field]
		if !exists {
			return fmt.Errorf("required field '%s' is missing", field)
		}

		// Check for empty string values
		if strValue, ok := value.(string); ok && strValue == "" {
			return fmt.Errorf("required field '%s' cannot be empty", field)
		}
	}
	return nil
}

// MapOptionalStringFieldToPtr maps a string field to a double pointer (used by SDK types)
func MapOptionalStringFieldToPtr(target **string, source map[string]any, key string) {
	if value, exists := source[key]; exists {
		if strValue, ok := value.(string); ok {
			*target = &strValue
		}
	}
}

// MapOptionalBoolFieldToPtr maps a boolean field to a double pointer (used by SDK types)
func MapOptionalBoolFieldToPtr(target **bool, source map[string]any, key string) {
	if value, exists := source[key]; exists {
		if boolValue, ok := value.(bool); ok {
			*target = &boolValue
		}
	}
}
