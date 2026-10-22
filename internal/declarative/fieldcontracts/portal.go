package fieldcontracts

import kkComps "github.com/Kong/sdk-konnect-go/models/components"

// ExtractPortalFields projects the nine ordinary optional fields. Identity,
// labels, references and secrets remain the caller's responsibility.
func ExtractPortalFields(desired kkComps.CreatePortal, fields map[string]any) {
	extractPortalBooleans(desired, fields)
	extractPortalText(desired, fields)
}

// ComparePortalFields compares only declared ordinary fields against list state.
// Unknown boolean observations remain nil in diagnostics.
func ComparePortalFields(
	desired kkComps.CreatePortal, current kkComps.ListPortalsResponsePortal,
) map[string]FieldChange {
	observed := portalListObservation{&current}
	changes := comparePortalText(desired, observed)
	for name, change := range comparePortalBooleans(desired, observed) {
		var old any
		if change.Old != nil {
			old = *change.Old
		}
		changes[name] = FieldChange{Old: old, New: change.New}
	}
	return changes
}

// MapPortalCreateFields leaves absent and wrong-typed fields untouched.
func MapPortalCreateFields(fields map[string]any, target *kkComps.CreatePortal) {
	mapPortalBooleanCreateFields(fields, target)
	mapPortalTextCreateFields(fields, target)
}

// MapPortalUpdateField handles one ordinary field, preserving caller iteration
// and error timing. Wrong-typed values are recognized but ignored.
func MapPortalUpdateField(name string, value any, target *kkComps.UpdatePortal) bool {
	return mapPortalBooleanUpdateField(name, value, target) || mapPortalTextUpdateField(name, value, target)
}

// ExportPortalFields projects ordinary fields from a Portal list response.
func ExportPortalFields(current kkComps.ListPortalsResponsePortal, target *kkComps.CreatePortal) {
	exportPortalFields(portalListObservation{&current}, target)
}

// ExportPortalResponseFields projects the same fields from a detail response.
func ExportPortalResponseFields(current kkComps.PortalResponse, target *kkComps.CreatePortal) {
	exportPortalFields(portalDetailObservation{&current}, target)
}

type portalObservation interface {
	portalBooleanObservation
	portalTextObservation
}

func exportPortalFields(current portalObservation, target *kkComps.CreatePortal) {
	exportPortalBooleans(current, target)
	exportPortalText(current, target)
}

// SDK response models differ only in the named visibility enum types needed
// here. Keep that conversion inside the module rather than in dump callers.
type portalListObservation struct {
	*kkComps.ListPortalsResponsePortal
}

func (p portalListObservation) defaultAPIVisibility() string {
	return string(p.GetDefaultAPIVisibility())
}

func (p portalListObservation) defaultPageVisibility() string {
	return string(p.GetDefaultPageVisibility())
}

type portalDetailObservation struct{ *kkComps.PortalResponse }

func (p portalDetailObservation) defaultAPIVisibility() string {
	return string(p.GetDefaultAPIVisibility())
}

func (p portalDetailObservation) defaultPageVisibility() string {
	return string(p.GetDefaultPageVisibility())
}
