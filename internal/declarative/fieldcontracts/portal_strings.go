package fieldcontracts

import kkComps "github.com/Kong/sdk-konnect-go/models/components"

// Planner constants retain compatibility aliases for these contract identifiers.
const (
	FieldDisplayName           = "display_name"
	FieldDescription           = "description"
	FieldDefaultAPIVisibility  = "default_api_visibility"
	FieldDefaultPageVisibility = "default_page_visibility"
)

type portalTextObservation interface {
	GetDisplayName() string
	GetDescription() *string
	defaultAPIVisibility() string
	defaultPageVisibility() string
}

type portalTextField struct {
	name            string
	declaration     func(*kkComps.CreatePortal) *string
	observation     func(portalTextObservation) *string
	create          func(*kkComps.CreatePortal, *string)
	update          func(*kkComps.UpdatePortal, string)
	omitEmptyExport bool
}

// The SDK uses different named string types for create and update visibility.
// Convert only at typed accessors; do not add enum validation or defaults here.
func portalTextFieldFor[C ~string, U ~string](
	name string,
	declaration func(*kkComps.CreatePortal) **C,
	observation func(portalTextObservation) *string,
	update func(*kkComps.UpdatePortal) **U,
	omitEmptyExport bool,
) portalTextField {
	return portalTextField{
		name: name,
		declaration: func(p *kkComps.CreatePortal) *string {
			value := *declaration(p)
			if value == nil {
				return nil
			}
			text := string(*value)
			return &text
		},
		observation: observation,
		create: func(p *kkComps.CreatePortal, text *string) {
			if text == nil {
				*declaration(p) = nil
				return
			}
			value := C(*text)
			*declaration(p) = &value
		},
		update:          func(p *kkComps.UpdatePortal, text string) { value := U(text); *update(p) = &value },
		omitEmptyExport: omitEmptyExport,
	}
}

// Comparison normalizes absent descriptions to empty. Export preserves their
// presence, but omits empty display names and visibility values.
var portalTextFields = [...]portalTextField{
	portalTextFieldFor(
		FieldDisplayName,
		func(p *kkComps.CreatePortal) **string { return &p.DisplayName },
		func(p portalTextObservation) *string { value := p.GetDisplayName(); return &value },
		func(p *kkComps.UpdatePortal) **string { return &p.DisplayName },
		true,
	),
	portalTextFieldFor(
		FieldDescription,
		func(p *kkComps.CreatePortal) **string { return &p.Description },
		portalTextObservation.GetDescription,
		func(p *kkComps.UpdatePortal) **string { return &p.Description },
		false,
	),
	portalTextFieldFor(
		FieldDefaultAPIVisibility,
		func(p *kkComps.CreatePortal) **kkComps.DefaultAPIVisibility { return &p.DefaultAPIVisibility },
		func(p portalTextObservation) *string { value := p.defaultAPIVisibility(); return &value },
		func(p *kkComps.UpdatePortal) **kkComps.UpdatePortalDefaultAPIVisibility {
			return &p.DefaultAPIVisibility
		},
		true,
	),
	portalTextFieldFor(
		FieldDefaultPageVisibility,
		func(p *kkComps.CreatePortal) **kkComps.DefaultPageVisibility { return &p.DefaultPageVisibility },
		func(p portalTextObservation) *string { value := p.defaultPageVisibility(); return &value },
		func(p *kkComps.UpdatePortal) **kkComps.UpdatePortalDefaultPageVisibility {
			return &p.DefaultPageVisibility
		},
		true,
	),
}

func extractPortalText(desired kkComps.CreatePortal, fields map[string]any) {
	for _, field := range portalTextFields {
		if value := field.declaration(&desired); value != nil {
			fields[field.name] = *value
		}
	}
}

func comparePortalText(desired kkComps.CreatePortal, current portalTextObservation) map[string]FieldChange {
	changes := make(map[string]FieldChange)
	for _, field := range portalTextFields {
		value := field.declaration(&desired)
		if value == nil {
			continue
		}
		old := ""
		if observed := field.observation(current); observed != nil {
			old = *observed
		}
		if old != *value {
			changes[field.name] = FieldChange{Old: old, New: *value}
		}
	}
	return changes
}

func mapPortalTextCreateFields(fields map[string]any, target *kkComps.CreatePortal) {
	for _, field := range portalTextFields {
		if value, ok := fields[field.name].(string); ok {
			field.create(target, &value)
		}
	}
}

func mapPortalTextUpdateField(name string, value any, target *kkComps.UpdatePortal) bool {
	for _, field := range portalTextFields {
		if field.name == name {
			if value, ok := value.(string); ok {
				field.update(target, value)
			}
			return true
		}
	}
	return false
}

func exportPortalText(current portalTextObservation, target *kkComps.CreatePortal) {
	for _, field := range portalTextFields {
		value := field.observation(current)
		if field.omitEmptyExport && (value == nil || *value == "") {
			continue
		}
		field.create(target, value)
	}
}
