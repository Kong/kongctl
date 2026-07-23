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
	for fieldName, expectedValue := range s.MatchFields {
		field, ok := externalSelectorStringField(v, fieldName)
		if !ok || field != expectedValue {
			return false
		}
	}

	return true
}

func externalSelectorStringField(value reflect.Value, selectorName string) (string, bool) {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return "", false
	}

	valueType := value.Type()
	for i := range value.NumField() {
		structField := valueType.Field(i)
		if structField.PkgPath != "" {
			continue
		}
		fieldValue := value.Field(i)
		if externalSelectorFieldMatches(structField, selectorName) {
			return externalSelectorStringValue(fieldValue)
		}
		if structField.Anonymous {
			if result, ok := externalSelectorStringField(fieldValue, selectorName); ok {
				return result, true
			}
		}
	}
	return "", false
}

func externalSelectorStringValue(value reflect.Value) (string, bool) {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.String {
		return "", false
	}
	return value.String(), true
}

func externalSelectorFieldMatches(field reflect.StructField, selectorName string) bool {
	if strings.EqualFold(field.Name, selectorName) {
		return true
	}
	for _, tagName := range []string{"json", "yaml"} {
		name, _, _ := strings.Cut(field.Tag.Get(tagName), ",")
		if name != "" && name != "-" && name == selectorName {
			return true
		}
	}
	return false
}
