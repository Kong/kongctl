// Package fieldcontracts owns resource field policies shared by planning,
// request mapping and declarative export. It does not depend on those stages.
package fieldcontracts

import kkComps "github.com/Kong/sdk-konnect-go/models/components"

// Planner constants retain compatibility aliases for these contract identifiers.
const (
	FieldAuthenticationEnabled   = "authentication_enabled"
	FieldRBACEnabled             = "rbac_enabled"
	FieldSIPREnabled             = "sipr_enabled"
	FieldAutoApproveDevelopers   = "auto_approve_developers"
	FieldAutoApproveApplications = "auto_approve_applications"
)

// PortalBooleanObservation preserves absent response values. Both Portal list
// and detail SDK responses satisfy this interface.
type PortalBooleanObservation interface {
	GetAuthenticationEnabled() *bool
	GetRbacEnabled() *bool
	GetSiprEnabled() *bool
	GetAutoApproveDevelopers() *bool
	GetAutoApproveApplications() *bool
}

type portalBooleanField struct {
	name        string
	declaration func(*kkComps.CreatePortal) **bool
	observation func(PortalBooleanObservation) *bool
	update      func(*kkComps.UpdatePortal) **bool
}

// Each enrollment supplies typed SDK access once; every projection uses it.
var portalBooleanFields = [...]portalBooleanField{
	{
		FieldAuthenticationEnabled,
		func(p *kkComps.CreatePortal) **bool { return &p.AuthenticationEnabled },
		PortalBooleanObservation.GetAuthenticationEnabled,
		func(p *kkComps.UpdatePortal) **bool { return &p.AuthenticationEnabled },
	},
	{
		FieldRBACEnabled,
		func(p *kkComps.CreatePortal) **bool { return &p.RbacEnabled },
		PortalBooleanObservation.GetRbacEnabled,
		func(p *kkComps.UpdatePortal) **bool { return &p.RbacEnabled },
	},
	{
		FieldSIPREnabled,
		func(p *kkComps.CreatePortal) **bool { return &p.SiprEnabled },
		PortalBooleanObservation.GetSiprEnabled,
		func(p *kkComps.UpdatePortal) **bool { return &p.SiprEnabled },
	},
	{
		FieldAutoApproveDevelopers,
		func(p *kkComps.CreatePortal) **bool { return &p.AutoApproveDevelopers },
		PortalBooleanObservation.GetAutoApproveDevelopers,
		func(p *kkComps.UpdatePortal) **bool { return &p.AutoApproveDevelopers },
	},
	{
		FieldAutoApproveApplications,
		func(p *kkComps.CreatePortal) **bool { return &p.AutoApproveApplications },
		PortalBooleanObservation.GetAutoApproveApplications,
		func(p *kkComps.UpdatePortal) **bool { return &p.AutoApproveApplications },
	},
}

// ExtractPortalBooleans includes explicitly declared values, including false.
func ExtractPortalBooleans(desired kkComps.CreatePortal, fields map[string]any) {
	for _, field := range portalBooleanFields {
		if value := *field.declaration(&desired); value != nil {
			fields[field.name] = *value
		}
	}
}

// PortalBooleanChange retains unknown observations separately from false.
type PortalBooleanChange struct {
	Old *bool
	New bool
}

// ComparePortalBooleans treats an omitted declaration as unmanaged. An explicit
// declaration differs from an absent observation, even when declaring false.
func ComparePortalBooleans(
	desired kkComps.CreatePortal, current PortalBooleanObservation,
) map[string]PortalBooleanChange {
	changes := make(map[string]PortalBooleanChange)
	for _, field := range portalBooleanFields {
		value := *field.declaration(&desired)
		if value == nil {
			continue
		}
		old := field.observation(current)
		if old == nil || *old != *value {
			changes[field.name] = PortalBooleanChange{Old: old, New: *value}
		}
	}
	return changes
}

// MapPortalBooleanCreateFields leaves absent or non-boolean fields untouched.
func MapPortalBooleanCreateFields(fields map[string]any, target *kkComps.CreatePortal) {
	for _, field := range portalBooleanFields {
		if value, ok := fields[field.name].(bool); ok {
			*field.declaration(target) = &value
		}
	}
}

// MapPortalBooleanUpdateField handles one field so callers retain their field
// iteration and error timing. Wrong-typed values are recognized but ignored.
func MapPortalBooleanUpdateField(name string, value any, target *kkComps.UpdatePortal) bool {
	for _, field := range portalBooleanFields {
		if field.name == name {
			if value, ok := value.(bool); ok {
				*field.update(target) = &value
			}
			return true
		}
	}
	return false
}

// ExportPortalBooleans copies observed values without manufacturing defaults.
func ExportPortalBooleans(current PortalBooleanObservation, target *kkComps.CreatePortal) {
	for _, field := range portalBooleanFields {
		if value := field.observation(current); value != nil {
			*field.declaration(target) = value
		}
	}
}
