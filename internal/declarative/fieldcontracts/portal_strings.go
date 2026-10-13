package fieldcontracts

import kkComps "github.com/Kong/sdk-konnect-go/models/components"

// Planner constants retain compatibility aliases for these contract identifiers.
const (
	FieldDisplayName = "display_name"
	FieldDescription = "description"
)

// PortalStringObservation supports both Portal list and detail responses.
type PortalStringObservation interface {
	GetDisplayName() string
	GetDescription() *string
}

type portalStringField struct {
	name            string
	declaration     func(*kkComps.CreatePortal) **string
	observation     func(PortalStringObservation) *string
	update          func(*kkComps.UpdatePortal) **string
	omitEmptyExport bool
}

// Comparison normalizes nil observations to empty for both fields. Export
// instead preserves description presence and omits an empty display name.
var portalStringFields = [...]portalStringField{
	{
		FieldDisplayName,
		func(p *kkComps.CreatePortal) **string { return &p.DisplayName },
		func(p PortalStringObservation) *string { value := p.GetDisplayName(); return &value },
		func(p *kkComps.UpdatePortal) **string { return &p.DisplayName },
		true,
	},
	{
		FieldDescription,
		func(p *kkComps.CreatePortal) **string { return &p.Description },
		PortalStringObservation.GetDescription,
		func(p *kkComps.UpdatePortal) **string { return &p.Description },
		false,
	},
}

// ExtractPortalStrings includes explicitly declared strings, including empty.
func ExtractPortalStrings(desired kkComps.CreatePortal, fields map[string]any) {
	for _, field := range portalStringFields {
		if value := *field.declaration(&desired); value != nil {
			fields[field.name] = *value
		}
	}
}

// PortalStringChange contains comparison values after observation normalization.
type PortalStringChange struct {
	Old string
	New string
}

// ComparePortalStrings treats omitted declarations as unmanaged and absent
// descriptions as empty, preserving Portal's existing comparison semantics.
func ComparePortalStrings(desired kkComps.CreatePortal, current PortalStringObservation) map[string]PortalStringChange {
	changes := make(map[string]PortalStringChange)
	for _, field := range portalStringFields {
		value := *field.declaration(&desired)
		if value == nil {
			continue
		}
		old := ""
		if observed := field.observation(current); observed != nil {
			old = *observed
		}
		if old != *value {
			changes[field.name] = PortalStringChange{Old: old, New: *value}
		}
	}
	return changes
}

// MapPortalStringCreateFields leaves absent or non-string fields untouched.
func MapPortalStringCreateFields(fields map[string]any, target *kkComps.CreatePortal) {
	for _, field := range portalStringFields {
		if value, ok := fields[field.name].(string); ok {
			*field.declaration(target) = &value
		}
	}
}

// MapPortalStringUpdateField handles one field to retain caller error timing.
// Wrong-typed values are recognized but ignored.
func MapPortalStringUpdateField(name string, value any, target *kkComps.UpdatePortal) bool {
	for _, field := range portalStringFields {
		if field.name == name {
			if value, ok := value.(string); ok {
				*field.update(target) = &value
			}
			return true
		}
	}
	return false
}

// ExportPortalStrings preserves description presence and omits empty display names.
func ExportPortalStrings(current PortalStringObservation, target *kkComps.CreatePortal) {
	for _, field := range portalStringFields {
		value := field.observation(current)
		if field.omitEmptyExport && (value == nil || *value == "") {
			continue
		}
		*field.declaration(target) = value
	}
}
