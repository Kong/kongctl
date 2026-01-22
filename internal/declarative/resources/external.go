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
	Deck *DeckRequires `yaml:"deck,omitempty" json:"deck,omitempty"`
}

// DeckRequires describes deck state files to apply or sync.
type DeckRequires struct {
	Files []string `yaml:"files"          json:"files"`
	Flags []string `yaml:"flags,omitempty" json:"flags,omitempty"`
}

// IsExternal returns true if this resource is externally managed
func (e *ExternalBlock) IsExternal() bool {
	return e != nil
}

// HasDeckRequires returns true when deck requirements are configured.
func (e *ExternalBlock) HasDeckRequires() bool {
	if e == nil || e.Requires == nil {
		return false
	}
	return e.Requires.Deck != nil
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
		if err := validateDeckRequires(e.Requires.Deck); err != nil {
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

func validateDeckRequires(requires *DeckRequires) error {
	if requires == nil || len(requires.Files) == 0 {
		return fmt.Errorf("_external requires.deck.files must include at least one state file")
	}

	for i, file := range requires.Files {
		value := strings.TrimSpace(file)
		if value == "" {
			return fmt.Errorf("_external requires.deck.files[%d] cannot be empty", i)
		}
		if strings.HasPrefix(value, "-") {
			return fmt.Errorf("_external requires.deck.files[%d] must be a file path, not a flag", i)
		}
		if strings.Contains(value, constants.DeckModePlaceholder) {
			return fmt.Errorf("_external requires.deck.files[%d] cannot include {{kongctl.mode}}", i)
		}
	}

	for i, flag := range requires.Flags {
		value := strings.TrimSpace(flag)
		if value == "" {
			return fmt.Errorf("_external requires.deck.flags[%d] cannot be empty", i)
		}
		if !strings.HasPrefix(value, "-") {
			return fmt.Errorf("_external requires.deck.flags[%d] must be a flag", i)
		}
		if strings.Contains(value, constants.DeckModePlaceholder) {
			return fmt.Errorf("_external requires.deck.flags[%d] cannot include {{kongctl.mode}}", i)
		}
		if deckFlagConflicts(value) {
			return fmt.Errorf("_external requires.deck.flags[%d] cannot include %s", i, value)
		}
	}

	return nil
}

func deckFlagConflicts(flag string) bool {
	denied := []string{
		"--konnect-token",
		"--konnect-control-plane-name",
		"--konnect-addr",
		"--json-output",
		"--output",
	}

	for _, candidate := range denied {
		if flag == candidate || strings.HasPrefix(flag, candidate+"=") {
			return true
		}
	}

	return false
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
