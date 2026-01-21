package resources

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/kong/kongctl/internal/declarative/constants"
)

// ExternalBlock marks a resource as externally managed
type ExternalBlock struct {
	// Direct ID reference (use when you know the exact Konnect ID)
	ID string `yaml:"id,omitempty" json:"id,omitempty"`

	// Selector for querying by fields (use to find by name)
	Selector *ExternalSelector `yaml:"selector,omitempty" json:"selector,omitempty"`

	// Requires declares external steps that must run before resolving this resource.
	Requires *ExternalRequires `yaml:"requires,omitempty" json:"requires,omitempty"`
}

// ExternalSelector defines field matching criteria
type ExternalSelector struct {
	// Field equality matches (initially just support name)
	MatchFields map[string]string `yaml:"matchFields" json:"matchFields"`
}

// ExternalRequires captures external dependency steps.
type ExternalRequires struct {
	Deck []DeckStep `yaml:"deck,omitempty" json:"deck,omitempty"`
}

// DeckStep represents a single deck invocation.
type DeckStep struct {
	Args []string `yaml:"args" json:"args"`
}

// IsExternal returns true if this resource is externally managed
func (e *ExternalBlock) IsExternal() bool {
	return e != nil
}

// HasDeckRequires returns true when deck steps are configured.
func (e *ExternalBlock) HasDeckRequires() bool {
	if e == nil || e.Requires == nil {
		return false
	}
	return len(e.Requires.Deck) > 0
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

	if e.HasDeckRequires() {
		if e.ID != "" {
			return fmt.Errorf("_external requires.deck cannot be used with 'id'")
		}
		if e.Selector == nil {
			return fmt.Errorf("_external requires.deck requires a selector")
		}
		if err := validateDeckRequires(e.Requires); err != nil {
			return err
		}
		if err := validateDeckSelector(e.Selector); err != nil {
			return err
		}
	}

	return nil
}

func validateDeckSelector(selector *ExternalSelector) error {
	if selector == nil {
		return fmt.Errorf("_external requires.deck requires selector.matchFields.name")
	}
	if len(selector.MatchFields) == 0 {
		return fmt.Errorf("_external requires.deck requires selector.matchFields.name")
	}
	if len(selector.MatchFields) != 1 {
		return fmt.Errorf("_external requires.deck only supports selector.matchFields.name")
	}
	if _, ok := selector.MatchFields["name"]; !ok {
		return fmt.Errorf("_external requires.deck only supports selector.matchFields.name")
	}
	return nil
}

func validateDeckRequires(requires *ExternalRequires) error {
	if requires == nil || len(requires.Deck) == 0 {
		return fmt.Errorf("_external requires.deck must include at least one step")
	}

	for i, step := range requires.Deck {
		if len(step.Args) == 0 {
			return fmt.Errorf("_external requires.deck[%d] args cannot be empty", i)
		}
		placeholderCount := 0
		for _, arg := range step.Args {
			if arg == constants.DeckModePlaceholder {
				placeholderCount++
				continue
			}
			if strings.Contains(arg, constants.DeckModePlaceholder) {
				return fmt.Errorf("_external requires.deck[%d] args contains invalid {{kongctl.mode}} usage", i)
			}
		}

		if placeholderCount > 1 {
			return fmt.Errorf("_external requires.deck[%d] args contains multiple {{kongctl.mode}} entries", i)
		}

		if step.Args[0] == "gateway" {
			if len(step.Args) < 2 {
				return fmt.Errorf("_external requires.deck[%d] gateway step must include a verb", i)
			}
			verb := step.Args[1]
			if verb != "sync" && verb != "apply" && verb != constants.DeckModePlaceholder {
				return fmt.Errorf("_external requires.deck[%d] gateway verb must be sync, apply, or {{kongctl.mode}}", i)
			}
		} else if placeholderCount > 0 {
			return fmt.Errorf("_external requires.deck[%d] {{kongctl.mode}} is only allowed for gateway steps", i)
		}

		if placeholderCount > 0 {
			if step.Args[0] != "gateway" || len(step.Args) < 2 || step.Args[1] != constants.DeckModePlaceholder {
				return fmt.Errorf("_external requires.deck[%d] {{kongctl.mode}} must be the gateway verb", i)
			}
		}
	}

	return nil
}

// capitalizeFirst capitalizes the first character of a string
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
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
		titleFieldName := capitalizeFirst(fieldName)
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
