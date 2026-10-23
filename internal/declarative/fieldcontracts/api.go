package fieldcontracts

import kkComps "github.com/Kong/sdk-konnect-go/models/components"

// Planner constants retain compatibility aliases for these contract identifiers.
const (
	FieldVersion    = "version"
	FieldSlug       = "slug"
	FieldAttributes = "attributes"
)

type apiTextField struct {
	name        string
	declaration func(*kkComps.CreateAPIRequest) **string
	observation func(*kkComps.APIResponseSchema) *string
	update      func(*kkComps.UpdateAPIRequest) **string
}

// All API text fields preserve pointer presence on export and normalize nil
// observations to empty only for comparison. Omitted declarations are unmanaged.
var apiTextFields = [...]apiTextField{
	{
		FieldDescription,
		func(p *kkComps.CreateAPIRequest) **string { return &p.Description },
		func(p *kkComps.APIResponseSchema) *string { return p.Description },
		func(p *kkComps.UpdateAPIRequest) **string { return &p.Description },
	},
	{
		FieldVersion,
		func(p *kkComps.CreateAPIRequest) **string { return &p.Version },
		func(p *kkComps.APIResponseSchema) *string { return p.Version },
		func(p *kkComps.UpdateAPIRequest) **string { return &p.Version },
	},
	{
		FieldSlug,
		func(p *kkComps.CreateAPIRequest) **string { return &p.Slug },
		func(p *kkComps.APIResponseSchema) *string { return p.Slug },
		func(p *kkComps.UpdateAPIRequest) **string { return &p.Slug },
	},
}

// ExtractAPIFields projects optional fields, leaving identity and labels to callers.
func ExtractAPIFields(desired kkComps.CreateAPIRequest, fields map[string]any) {
	for _, field := range apiTextFields {
		if value := *field.declaration(&desired); value != nil {
			fields[field.name] = *value
		}
	}
	if desired.Attributes != nil {
		fields[FieldAttributes] = normalizeAPIAttributes(desired.Attributes)
	}
}

// CompareAPIFields compares only declared fields; attributes retain their
// request representation in diagnostics rather than comparison canonicalization.
func CompareAPIFields(desired kkComps.CreateAPIRequest, current kkComps.APIResponseSchema) map[string]FieldChange {
	changes := make(map[string]FieldChange)
	for _, field := range apiTextFields {
		value := *field.declaration(&desired)
		if value == nil {
			continue
		}
		old := ""
		if observed := field.observation(&current); observed != nil {
			old = *observed
		}
		if old != *value {
			changes[field.name] = FieldChange{Old: old, New: *value}
		}
	}
	if desired.Attributes != nil {
		old := normalizeAPIAttributes(current.Attributes)
		value := normalizeAPIAttributes(desired.Attributes)
		if !attributesEqual(old, value) {
			changes[FieldAttributes] = FieldChange{Old: old, New: value}
		}
	}
	return changes
}

// MapAPICreateFields ignores absent or wrong-typed text fields. Attributes
// normalize when supported and otherwise retain the supplied value, including nil.
func MapAPICreateFields(fields map[string]any, target *kkComps.CreateAPIRequest) {
	for _, field := range apiTextFields {
		if value, ok := fields[field.name].(string); ok {
			*field.declaration(target) = &value
		}
	}
	if value, ok := fields[FieldAttributes]; ok {
		target.Attributes = normalizeAPIAttributes(value)
	}
}

// MapAPIUpdateFields maps only supplied changes, preserving PATCH sparsity.
func MapAPIUpdateFields(fields map[string]any, target *kkComps.UpdateAPIRequest) {
	for _, field := range apiTextFields {
		if value, ok := fields[field.name].(string); ok {
			*field.update(target) = &value
		}
	}
	if value, ok := fields[FieldAttributes]; ok {
		target.Attributes = normalizeAPIAttributes(value)
	}
}

// ExportAPIFields preserves observed text presence and raw nonempty attributes.
// Export does not apply request normalization or comparison canonicalization.
func ExportAPIFields(current kkComps.APIResponseSchema, target *kkComps.CreateAPIRequest) {
	for _, field := range apiTextFields {
		*field.declaration(target) = field.observation(&current)
	}
	target.Attributes = current.Attributes
	if emptyAPIAttributes(target.Attributes) {
		target.Attributes = nil
	}
}
