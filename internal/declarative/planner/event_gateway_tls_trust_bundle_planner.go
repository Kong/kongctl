package planner

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// planEventGatewayTLSTrustBundleChanges plans changes for Event Gateway TLS Trust Bundles
// for a specific gateway.
func (p *Planner) planEventGatewayTLSTrustBundleChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayName string,
	gatewayID string,
	gatewayRef string,
	gatewayChangeID string,
	desired []resources.EventGatewayTLSTrustBundleResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning Event Gateway TLS Trust Bundle changes",
		"gateway_name", gatewayName,
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"desired_count", len(desired),
		"namespace", namespace,
	)

	if gatewayID != "" {
		return p.planTrustBundleChangesForExistingGateway(
			ctx, namespace, gatewayID, gatewayRef, gatewayName, desired, plan,
		)
	}

	// Gateway doesn't exist yet: plan creates only with dependency on gateway creation
	p.planTrustBundleCreatesForNewGateway(
		namespace, gatewayRef, gatewayName, gatewayChangeID, desired, plan)
	return nil
}

// planTrustBundleChangesForExistingGateway handles full diff for trust bundles of an existing gateway.
func (p *Planner) planTrustBundleChangesForExistingGateway(
	ctx context.Context,
	namespace string,
	gatewayID string,
	gatewayRef string,
	gatewayName string,
	desired []resources.EventGatewayTLSTrustBundleResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning changes for existing gateway TLS trust bundles",
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"desired_count", len(desired),
	)

	currentBundles, err := p.client.ListEventGatewayTLSTrustBundles(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list TLS trust bundles for gateway %s: %w", gatewayID, err)
	}

	p.logger.Debug("Fetched current TLS trust bundles",
		"gateway_id", gatewayID,
		"current_count", len(currentBundles),
	)

	currentByName := make(map[string]state.EventGatewayTLSTrustBundle)
	for _, tb := range currentBundles {
		currentByName[tb.Name] = tb
	}

	desiredNames := make(map[string]bool)

	for _, desiredTB := range desired {
		name := desiredTB.GetMoniker()
		desiredNames[name] = true

		current, exists := currentByName[name]

		if !exists {
			p.logger.Debug("Planning TLS trust bundle CREATE",
				"bundle_name", name,
				"gateway_ref", gatewayRef,
			)
			p.planTrustBundleCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredTB, []string{}, plan)
		} else {
			p.logger.Debug("Checking if TLS trust bundle needs update",
				"bundle_name", name,
				"bundle_id", current.ID,
			)

			needsUpdate, updateFields, changedFields := p.shouldUpdateTrustBundle(current, desiredTB)
			if needsUpdate {
				p.logger.Debug("Planning TLS trust bundle UPDATE",
					"bundle_name", name,
					"bundle_id", current.ID,
					"update_fields", updateFields,
				)
				p.planTrustBundleUpdate(
					namespace, gatewayRef, gatewayID,
					current.ID, desiredTB, updateFields, changedFields, plan)
			}
		}
	}

	// SYNC MODE: Delete unmanaged trust bundles
	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				p.logger.Debug("Planning TLS trust bundle DELETE (sync mode)",
					"bundle_name", name,
					"bundle_id", current.ID,
				)
				p.planTrustBundleDelete(gatewayRef, gatewayID, current.ID, name, plan)
			}
		}
	}

	return nil
}

// planTrustBundleCreatesForNewGateway plans creates for trust bundles when the gateway doesn't exist yet.
func (p *Planner) planTrustBundleCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	bundles []resources.EventGatewayTLSTrustBundleResource,
	plan *Plan,
) {
	p.logger.Debug("Planning TLS trust bundle creates for new gateway",
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"bundle_count", len(bundles),
	)

	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}

	for _, tb := range bundles {
		p.planTrustBundleCreate(namespace, gatewayRef, gatewayName, "", tb, dependsOn, plan)
	}
}

// planTrustBundleCreate plans a CREATE change for a TLS trust bundle.
func (p *Planner) planTrustBundleCreate(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	tb resources.EventGatewayTLSTrustBundleResource,
	dependsOn []string,
	plan *Plan,
) {
	fields := buildTrustBundleFields(tb)

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeEventGatewayTLSTrustBundle, tb.Ref),
		ResourceType: ResourceTypeEventGatewayTLSTrustBundle,
		ResourceRef:  tb.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}

	if gatewayID != "" {
		change.Parent = &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		}
	} else {
		change.References = map[string]ReferenceInfo{
			FieldEventGatewayID: {
				Ref: gatewayRef,
				ID:  "", // to be resolved at runtime
				LookupFields: map[string]string{
					FieldName: gatewayName,
				},
			},
		}
	}

	plan.AddChange(change)
}

// planTrustBundleUpdate plans an UPDATE change for a TLS trust bundle.
func (p *Planner) planTrustBundleUpdate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	bundleID string,
	tb resources.EventGatewayTLSTrustBundleResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypeEventGatewayTLSTrustBundle, tb.Ref),
		ResourceType: ResourceTypeEventGatewayTLSTrustBundle,
		ResourceRef:  tb.Ref,
		ResourceID:   bundleID,
		Action:       ActionUpdate,
		Fields:       updateFields,
		Namespace:    namespace,
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
	}
	if len(changedFields) > 0 {
		change.ChangedFields = changedFields
	}

	plan.AddChange(change)
}

// planTrustBundleDelete plans a DELETE change for a TLS trust bundle.
func (p *Planner) planTrustBundleDelete(
	gatewayRef string,
	gatewayID string,
	bundleID string,
	bundleName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeEventGatewayTLSTrustBundle, bundleName),
		ResourceType: ResourceTypeEventGatewayTLSTrustBundle,
		ResourceRef:  bundleName,
		ResourceID:   bundleID,
		Action:       ActionDelete,
		Namespace:    "",
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
	}

	plan.AddChange(change)
}

// shouldUpdateTrustBundle compares current and desired TLS trust bundle state.
// It returns whether an update is needed, the fields to send in the PUT request,
// and a map of which specific fields changed.
func (p *Planner) shouldUpdateTrustBundle(
	current state.EventGatewayTLSTrustBundle,
	desired resources.EventGatewayTLSTrustBundleResource,
) (bool, map[string]any, map[string]FieldChange) {
	updates := make(map[string]any)
	changes := make(map[string]FieldChange)

	// Compare name
	if current.Name != desired.Name {
		changes[FieldName] = FieldChange{Old: current.Name, New: desired.Name}
	}

	// Compare description
	currentDesc := ""
	if current.Description != nil {
		currentDesc = *current.Description
	}
	desiredDesc := ""
	if desired.Description != nil {
		desiredDesc = *desired.Description
	}
	if currentDesc != desiredDesc {
		changes[FieldDescription] = FieldChange{Old: currentDesc, New: desiredDesc}
	}

	// Compare config.trusted_ca (nested object comparison)
	if current.Config.TrustedCa != desired.Config.TrustedCa {
		changes["config.trusted_ca"] = FieldChange{Old: current.Config.TrustedCa, New: desired.Config.TrustedCa}
	}

	// Compare labels
	if !labelsEqual(current.NormalizedLabels, desired.Labels) {
		changes[FieldLabels] = FieldChange{Old: current.NormalizedLabels, New: desired.Labels}
	}

	if len(changes) > 0 {
		updates = buildTrustBundleFields(desired)
	}

	return len(changes) > 0, updates, changes
}

// buildTrustBundleFields builds the field map used in PlannedChange.Fields.
func buildTrustBundleFields(tb resources.EventGatewayTLSTrustBundleResource) map[string]any {
	fields := make(map[string]any)

	fields[FieldName] = tb.Name

	if tb.Description != nil {
		fields[FieldDescription] = *tb.Description
	}

	fields[FieldConfig] = kkComps.TLSTrustBundleConfig{
		TrustedCa: tb.Config.TrustedCa,
	}

	if len(tb.Labels) > 0 {
		fields[FieldLabels] = tb.Labels
	}

	return fields
}
