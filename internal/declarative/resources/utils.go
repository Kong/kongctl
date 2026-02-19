package resources

import (
	"reflect"
)

// resolveStringField safely extracts a string value from a struct field.
// It also searches embedded structs for the field.
func resolveStringField(v reflect.Value, fieldName string) string {
	// Try direct field access first
	field := v.FieldByName(fieldName)
	//nolint: exhaustive
	if field.IsValid() {
		switch field.Kind() {
		case reflect.String:
			return field.String()
		case reflect.Ptr:
			if !field.IsNil() && field.Elem().Kind() == reflect.String {
				return field.Elem().String()
			}
		}
	}

	// Field wasn't found directly, searching embedded structs
	// including anonymous fields - structs embedded without a name
	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			fieldValue := v.Field(i)
			fieldType := v.Type().Field(i)

			if fieldType.Anonymous && fieldValue.Kind() == reflect.Struct {
				result := resolveStringField(fieldValue, fieldName)
				if result != "" {
					return result
				}
			}
		}
	}

	return ""
}

// ExtractNameAndID extracts Name and ID from a Konnect response struct.
// Use this in resource files that need custom extraction from named embedded structs.
//
// Example for API resource with APIResponseSchema:
//
//	func (a *APIResource) TryMatchKonnectResource(kr any) bool {
//	    name, id := ExtractNameAndID(kr, "APIResponseSchema")
//	    if name == a.Name && id != "" {
//	        a.konnectID = id
//	        return true
//	    }
//	    return false
//	}
func extractNameAndID(konnectResource any, embeddedStructName string) (name, id string) {
	kv := reflect.ValueOf(konnectResource)
	if kv.Kind() == reflect.Ptr {
		kv = kv.Elem()
	}

	if kv.Kind() != reflect.Struct {
		return "", ""
	}

	// Try direct access first
	name = resolveStringField(kv, "Name")
	id = resolveStringField(kv, "ID")

	// If not found and embeddedStructName provided, try that
	if (name == "" || id == "") && embeddedStructName != "" {
		embeddedField := kv.FieldByName(embeddedStructName)
		if embeddedField.IsValid() && embeddedField.Kind() == reflect.Struct {
			if name == "" {
				name = resolveStringField(embeddedField, "Name")
			}
			if id == "" {
				id = resolveStringField(embeddedField, "ID")
			}
		}
	}

	return name, id
}

// tryMatchByField attempts to match a Konnect resource by comparing a field value.
// Returns the Konnect ID if the specified field matches expectedValue, empty string otherwise.
// Handles both string and *string field types in the Konnect resource.
//
// Example usage:
//
//	func (p *PortalPageResource) tryMatchKonnectResource(kr any) bool {
//	    if id := tryMatchByField(kr, "Slug", p.Slug); id != "" {
//	        p.konnectID = id
//	        return true
//	    }
//	    return false
//	}
func tryMatchByField(konnectResource any, fieldName, expectedValue string) string {
	v := reflect.ValueOf(konnectResource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}

	matchValue := resolveStringField(v, fieldName)
	id := resolveStringField(v, "ID")

	if matchValue == expectedValue && id != "" {
		return id
	}
	return ""
}

// tryMatchByNameWithExternal attempts to match a resource
// with a Konnect resource by name.
// It handles both external (selector/ID-based) and name-based matching.
// On successful match, sets konnectID and returns true.
func tryMatchByNameWithExternal(
	resourceName string,
	konnectResource any,
	opts matchOptions,
	external *ExternalBlock,
) (string, bool) {
	name, id := extractNameAndID(konnectResource, opts.sdkType)
	if id == "" {
		return "", false
	}

	// External matching
	if external != nil && external.IsExternal() {
		if external.ID != "" && id == external.ID {
			return id, true
		}
		if external.Selector != nil && external.Selector.Match(konnectResource) {
			return id, true
		}
		return "", false
	}

	// Name-based matching
	if name == resourceName {
		return id, true
	}
	return "", false
}
