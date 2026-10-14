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

// portalBooleanObservation preserves absent response values. Both Portal list
// and detail SDK responses satisfy this interface.
type portalBooleanObservation interface {
	GetAuthenticationEnabled() *bool
	GetRbacEnabled() *bool
	GetSiprEnabled() *bool
	GetAutoApproveDevelopers() *bool
	GetAutoApproveApplications() *bool
}

type portalBooleanField struct {
	name        string
	declaration func(*kkComps.CreatePortal) **bool
	observation func(portalBooleanObservation) *bool
	update      func(*kkComps.UpdatePortal) **bool
}

// Each enrollment supplies typed SDK access once; every projection uses it.
var portalBooleanFields = [...]portalBooleanField{
	{
		FieldAuthenticationEnabled,
		func(p *kkComps.CreatePortal) **bool { return &p.AuthenticationEnabled },
		portalBooleanObservation.GetAuthenticationEnabled,
		func(p *kkComps.UpdatePortal) **bool { return &p.AuthenticationEnabled },
	},
	{
		FieldRBACEnabled,
		func(p *kkComps.CreatePortal) **bool { return &p.RbacEnabled },
		portalBooleanObservation.GetRbacEnabled,
		func(p *kkComps.UpdatePortal) **bool { return &p.RbacEnabled },
	},
	{
		FieldSIPREnabled,
		func(p *kkComps.CreatePortal) **bool { return &p.SiprEnabled },
		portalBooleanObservation.GetSiprEnabled,
		func(p *kkComps.UpdatePortal) **bool { return &p.SiprEnabled },
	},
	{
		FieldAutoApproveDevelopers,
		func(p *kkComps.CreatePortal) **bool { return &p.AutoApproveDevelopers },
		portalBooleanObservation.GetAutoApproveDevelopers,
		func(p *kkComps.UpdatePortal) **bool { return &p.AutoApproveDevelopers },
	},
	{
		FieldAutoApproveApplications,
		func(p *kkComps.CreatePortal) **bool { return &p.AutoApproveApplications },
		portalBooleanObservation.GetAutoApproveApplications,
		func(p *kkComps.UpdatePortal) **bool { return &p.AutoApproveApplications },
	},
}

// extractPortalBooleans includes explicitly declared values, including false.
func extractPortalBooleans(desired kkComps.CreatePortal, fields map[string]any) {
	for _, field := range portalBooleanFields {
		if value := *field.declaration(&desired); value != nil {
			fields[field.name] = *value
		}
	}
}

// portalBooleanChange retains unknown observations separately from false.
type portalBooleanChange struct {
	Old *bool
	New bool
}

// comparePortalBooleans treats an omitted declaration as unmanaged. An explicit
// declaration differs from an absent observation, even when declaring false.
func comparePortalBooleans(
	desired kkComps.CreatePortal, current portalBooleanObservation,
) map[string]portalBooleanChange {
	changes := make(map[string]portalBooleanChange)
	for _, field := range portalBooleanFields {
		value := *field.declaration(&desired)
		if value == nil {
			continue
		}
		old := field.observation(current)
		if old == nil || *old != *value {
			changes[field.name] = portalBooleanChange{Old: old, New: *value}
		}
	}
	return changes
}

// mapPortalBooleanCreateFields leaves absent or non-boolean fields untouched.
func mapPortalBooleanCreateFields(fields map[string]any, target *kkComps.CreatePortal) {
	for _, field := range portalBooleanFields {
		if value, ok := fields[field.name].(bool); ok {
			*field.declaration(target) = &value
		}
	}
}

// mapPortalBooleanUpdateField handles one field so callers retain their field
// iteration and error timing. Wrong-typed values are recognized but ignored.
func mapPortalBooleanUpdateField(name string, value any, target *kkComps.UpdatePortal) bool {
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

// exportPortalBooleans copies observed values without manufacturing defaults.
func exportPortalBooleans(current portalBooleanObservation, target *kkComps.CreatePortal) {
	for _, field := range portalBooleanFields {
		if value := field.observation(current); value != nil {
			*field.declaration(target) = value
		}
	}
}
