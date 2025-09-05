package resources

import (
	"fmt"
	"reflect"
	"strings"
)

// ExternalBlock marks a resource as externally managed
type ExternalBlock struct {
	// Direct ID reference (use when you know the exact Konnect ID)
	ID string `yaml:"id,omitempty" json:"id,omitempty"`

	// Selector for querying by fields (use to find by name)
	Selector *ExternalSelector `yaml:"selector,omitempty" json:"selector,omitempty"`
}

// ExternalSelector defines field matching criteria
type ExternalSelector struct {
	// Field equality matches (initially just support name)
	MatchFields map[string]string `yaml:"matchFields" json:"matchFields"`
}

// IsExternal returns true if this resource is externally managed
func (e *ExternalBlock) IsExternal() bool {
	return e != nil
}

// Validate ensures the external block is properly configured
func (e *ExternalBlock) Validate() error {
	if e == nil {
		return nil
	}

	if e.ID != "" && e.Selector != nil {
		return fmt.Errorf("_external block cannot have both 'id' and 'selector'")
	}

	if e.ID == "" && e.Selector == nil {
		return fmt.Errorf("_external block must have either 'id' or 'selector'")
	}

	if e.Selector != nil && len(e.Selector.MatchFields) == 0 {
		return fmt.Errorf("_external selector must have at least one matchField")
	}

	return nil
}

// Match checks if the given Konnect resource matches this selector
func (s *ExternalSelector) Match(konnectResource any) bool {
	if s == nil || len(s.MatchFields) == 0 {
		return false
	}

	v := reflect.ValueOf(konnectResource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return false
	}

	// Check all match fields
	for fieldName, expectedValue := range s.MatchFields {
		// Convert field name to title case for reflection (e.g., "name" -> "Name")
		titleFieldName := strings.Title(fieldName)
		field := v.FieldByName(titleFieldName)

		// Try embedded structs if direct field not found
		if !field.IsValid() {
			// Look in embedded Portal struct
			if portalField := v.FieldByName("Portal"); portalField.IsValid() && portalField.Kind() == reflect.Struct {
				field = portalField.FieldByName(titleFieldName)
			}
		}

		if !field.IsValid() || field.Kind() != reflect.String || field.String() != expectedValue {
			return false
		}
	}

	return true
}
