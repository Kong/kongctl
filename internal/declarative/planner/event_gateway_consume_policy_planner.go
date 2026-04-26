package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// planEventGatewayConsumePolicyChanges plans changes for Event Gateway Consume Policies
// for a specific virtual cluster within a specific gateway.
func (p *Planner) planEventGatewayConsumePolicyChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayID string,
	gatewayRef string,
	virtualClusterName string,
	virtualClusterID string,
	virtualClusterRef string,
	virtualClusterChangeID string,
	desired []resources.EventGatewayConsumePolicyResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning Event Gateway Consume Policy changes",
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"virtual_cluster_name", virtualClusterName,
		"virtual_cluster_id", virtualClusterID,
		"virtual_cluster_ref", virtualClusterRef,
		"virtual_cluster_change_id", virtualClusterChangeID,
		"desired_count", len(desired),
		"namespace", namespace,
	)

	if virtualClusterID != "" && gatewayID != "" {
		return p.planConsumePolicyChangesForExistingVirtualCluster(
			ctx, namespace, gatewayID, gatewayRef, virtualClusterID, virtualClusterRef, virtualClusterName,
			desired, plan,
		)
	}

	// Virtual cluster doesn't exist yet: plan creates only, with dependency on virtual cluster creation
	p.planConsumePolicyCreatesForNewVirtualCluster(
		namespace, gatewayRef, virtualClusterRef, virtualClusterName, virtualClusterChangeID, desired, plan,
	)
	return nil
}

// planConsumePolicyChangesForExistingVirtualCluster handles full diff for consume policies
// when both the gateway and virtual cluster already exist.
func (p *Planner) planConsumePolicyChangesForExistingVirtualCluster(
	ctx context.Context,
	namespace string,
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	virtualClusterName string,
	desired []resources.EventGatewayConsumePolicyResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning changes for existing virtual cluster consume policies",
		"gateway_id", gatewayID,
		"virtual_cluster_id", virtualClusterID,
		"virtual_cluster_ref", virtualClusterRef,
		"desired_count", len(desired),
	)

	// 1. List current consume policies for this virtual cluster
	currentPolicies, err := p.client.ListEventGatewayConsumePolicies(ctx, gatewayID, virtualClusterID)
	if err != nil {
		return fmt.Errorf("failed to list consume policies for virtual cluster %s: %w", virtualClusterID, err)
	}

	p.logger.Debug("Fetched current consume policies",
		"virtual_cluster_id", virtualClusterID,
		"current_count", len(currentPolicies),
	)

	// 2. Index current policies by name
	currentByName := make(map[string]state.EventGatewayConsumePolicyInfo)
	for _, policy := range currentPolicies {
		if policy.Name != nil {
			currentByName[*policy.Name] = policy
		}
	}

	// 3. Compare desired vs current
	desiredNames := make(map[string]bool)
	for _, desiredPolicy := range desired {
		policyName := desiredPolicy.GetMoniker()
		desiredNames[policyName] = true
		current, exists := currentByName[policyName]
		if !exists {
			p.logger.Debug("Planning consume policy CREATE",
				"policy_name", policyName,
				"virtual_cluster_ref", virtualClusterRef,
			)
			p.planConsumePolicyCreate(
				namespace, gatewayID, gatewayRef, virtualClusterID, virtualClusterRef, virtualClusterName,
				desiredPolicy, []string{}, plan,
			)
		} else {
			p.logger.Debug("Checking if consume policy needs update",
				"policy_name", policyName,
				"policy_id", current.ID,
			)

			needsUpdate, updateFields, changedFields := p.shouldUpdateConsumePolicy(current, desiredPolicy)
			if needsUpdate {
				p.logger.Debug("Planning consume policy UPDATE",
					"policy_name", policyName,
					"policy_id", current.ID,
					"changed_fields", changedFields,
				)
				p.planConsumePolicyUpdate(
					namespace, gatewayID, gatewayRef, virtualClusterID, virtualClusterRef,
					current.ID, desiredPolicy, updateFields, changedFields, plan,
				)
			}
		}
	}

	// 4. SYNC MODE: Delete policies no longer in desired state
	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				p.logger.Debug("Planning consume policy DELETE (sync mode)",
					"policy_name", name,
					"policy_id", current.ID,
				)
				p.planConsumePolicyDelete(
					gatewayID, gatewayRef, virtualClusterID, virtualClusterRef,
					current.ID, name, plan,
				)
			}
		}
	}

	return nil
}

// planConsumePolicyCreatesForNewVirtualCluster plans creates for consume policies
// when the parent virtual cluster doesn't exist yet.
func (p *Planner) planConsumePolicyCreatesForNewVirtualCluster(
	namespace string,
	gatewayRef string,
	virtualClusterRef string,
	virtualClusterName string,
	virtualClusterChangeID string,
	policies []resources.EventGatewayConsumePolicyResource,
	plan *Plan,
) {
	p.logger.Debug("Planning consume policy creates for new virtual cluster",
		"virtual_cluster_ref", virtualClusterRef,
		"virtual_cluster_change_id", virtualClusterChangeID,
		"policy_count", len(policies),
	)

	var dependsOn []string
	if virtualClusterChangeID != "" {
		dependsOn = []string{virtualClusterChangeID}
	}

	for _, policy := range policies {
		p.planConsumePolicyCreate(
			namespace, "", gatewayRef, "", virtualClusterRef, virtualClusterName,
			policy, dependsOn, plan,
		)
	}
}

// planConsumePolicyCreate plans a CREATE change for a consume policy.
func (p *Planner) planConsumePolicyCreate(
	namespace string,
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	virtualClusterName string,
	policy resources.EventGatewayConsumePolicyResource,
	dependsOn []string,
	plan *Plan,
) {
	fields := p.consumePolicyToFields(policy)

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeEventGatewayConsumePolicy, policy.Ref),
		ResourceType: ResourceTypeEventGatewayConsumePolicy,
		ResourceRef:  policy.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}

	if virtualClusterID != "" {
		change.Parent = &ParentInfo{
			Ref: virtualClusterRef,
			ID:  virtualClusterID,
		}
	}

	change.References = map[string]ReferenceInfo{
		FieldEventGatewayID: {
			Ref: gatewayRef,
			ID:  gatewayID,
		},
		FieldEventGatewayVirtualClusterID: {
			Ref: virtualClusterRef,
			ID:  virtualClusterID,
			LookupFields: map[string]string{
				FieldName: virtualClusterName,
			},
		},
	}

	p.logger.Debug("Enqueuing consume policy CREATE",
		"policy_ref", policy.Ref,
		"policy_name", policy.GetMoniker(),
		"virtual_cluster_ref", virtualClusterRef,
		"gateway_ref", gatewayRef,
	)
	plan.AddChange(change)
}

// planConsumePolicyUpdate plans an UPDATE change for a consume policy.
func (p *Planner) planConsumePolicyUpdate(
	namespace string,
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	policyID string,
	policy resources.EventGatewayConsumePolicyResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	if len(updateFields) == 0 {
		return
	}

	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeEventGatewayConsumePolicy, policy.Ref),
		ResourceType:  ResourceTypeEventGatewayConsumePolicy,
		ResourceRef:   policy.Ref,
		ResourceID:    policyID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		Parent: &ParentInfo{
			Ref: virtualClusterRef,
			ID:  virtualClusterID,
		},
		References: map[string]ReferenceInfo{
			FieldEventGatewayID: {
				Ref: gatewayRef,
				ID:  gatewayID,
			},
			FieldEventGatewayVirtualClusterID: {
				Ref: virtualClusterRef,
				ID:  virtualClusterID,
			},
		},
	}

	p.logger.Debug("Enqueuing consume policy UPDATE",
		"policy_ref", policy.Ref,
		"policy_name", policy.GetMoniker(),
		"policy_id", policyID,
	)
	plan.AddChange(change)
}

// planConsumePolicyDelete plans a DELETE change for a consume policy.
func (p *Planner) planConsumePolicyDelete(
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	policyID string,
	policyName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeEventGatewayConsumePolicy, policyName),
		ResourceType: ResourceTypeEventGatewayConsumePolicy,
		ResourceRef:  policyName,
		ResourceID:   policyID,
		Action:       ActionDelete,
		Parent: &ParentInfo{
			Ref: virtualClusterRef,
			ID:  virtualClusterID,
		},
		References: map[string]ReferenceInfo{
			FieldEventGatewayID: {
				Ref: gatewayRef,
				ID:  gatewayID,
			},
			FieldEventGatewayVirtualClusterID: {
				Ref: virtualClusterRef,
				ID:  virtualClusterID,
			},
		},
	}

	p.logger.Debug("Enqueuing consume policy DELETE",
		"policy_name", policyName,
		"policy_id", policyID,
	)
	plan.AddChange(change)
}

// consumePolicyToFields converts a consume policy resource to a fields map
// by serializing the embedded union type to JSON.
func (p *Planner) consumePolicyToFields(policy resources.EventGatewayConsumePolicyResource) map[string]any {
	data, err := json.Marshal(policy.EventGatewayConsumePolicyCreate)
	if err != nil {
		p.logger.Warn("Failed to marshal consume policy to fields", "error", err)
		return map[string]any{}
	}

	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		p.logger.Warn("Failed to unmarshal consume policy fields", "error", err)
		return map[string]any{}
	}

	return fields
}

// extractConsumePolicyLabels extracts labels from whichever union variant is set.
func (p *Planner) extractConsumePolicyLabels(
	policy resources.EventGatewayConsumePolicyResource,
) map[string]string {
	v := reflect.ValueOf(policy.EventGatewayConsumePolicyCreate)
	for _, field := range v.Fields() {
		if field.Kind() != reflect.Pointer || field.IsNil() {
			continue
		}
		labelsField := field.Elem().FieldByName("Labels")
		if labelsField.IsValid() && !labelsField.IsNil() {
			if labels, ok := labelsField.Interface().(map[string]string); ok {
				return labels
			}
		}
	}
	return nil
}

// extractConsumePolicyDescription extracts description from whichever union variant is set.
func (p *Planner) extractConsumePolicyDescription(
	policy resources.EventGatewayConsumePolicyResource,
) string {
	v := reflect.ValueOf(policy.EventGatewayConsumePolicyCreate)
	for _, field := range v.Fields() {
		if field.Kind() != reflect.Pointer || field.IsNil() {
			continue
		}
		descField := field.Elem().FieldByName("Description")
		if descField.IsValid() && descField.Kind() == reflect.Pointer && !descField.IsNil() {
			return descField.Elem().String()
		}
	}
	return ""
}

// extractConsumePolicyEnabled extracts the enabled flag from whichever union variant is set.
func (p *Planner) extractConsumePolicyEnabled(
	policy resources.EventGatewayConsumePolicyResource,
) bool {
	v := reflect.ValueOf(policy.EventGatewayConsumePolicyCreate)
	for _, field := range v.Fields() {
		if field.Kind() != reflect.Pointer || field.IsNil() {
			continue
		}
		enabledField := field.Elem().FieldByName("Enabled")
		if enabledField.IsValid() && enabledField.Kind() == reflect.Pointer && !enabledField.IsNil() {
			return enabledField.Elem().Bool()
		}
	}
	return true // default is enabled
}

// extractConsumePolicyConfig extracts config from whichever union variant is set.
func (p *Planner) extractConsumePolicyConfig(
	policy resources.EventGatewayConsumePolicyResource,
) map[string]any {
	v := reflect.ValueOf(policy.EventGatewayConsumePolicyCreate)
	for _, field := range v.Fields() {
		if field.Kind() != reflect.Pointer || field.IsNil() {
			continue
		}
		configField := field.Elem().FieldByName("Config")
		if !configField.IsValid() {
			continue
		}

		data, err := json.Marshal(configField.Interface())
		if err != nil {
			continue
		}

		var config map[string]any
		if err := json.Unmarshal(data, &config); err != nil {
			continue
		}

		if len(config) > 0 {
			return config
		}
	}
	return nil
}

// shouldUpdateConsumePolicy compares current and desired consume policy state.
func (p *Planner) shouldUpdateConsumePolicy(
	current state.EventGatewayConsumePolicyInfo,
	desired resources.EventGatewayConsumePolicyResource,
) (bool, map[string]any, map[string]FieldChange) {
	var needsUpdate bool
	changes := make(map[string]FieldChange)

	// Compare name
	desiredName := desired.GetMoniker()
	currentName := ""
	if current.Name != nil {
		currentName = *current.Name
	}
	if currentName != desiredName {
		needsUpdate = true
		changes[FieldName] = FieldChange{Old: currentName, New: desiredName}
	}

	// Compare description
	currentDesc := ""
	if current.Description != nil {
		currentDesc = *current.Description
	}
	desiredDesc := p.extractConsumePolicyDescription(desired)
	if currentDesc != desiredDesc {
		needsUpdate = true
		changes[FieldDescription] = FieldChange{Old: currentDesc, New: desiredDesc}
	}

	// Compare enabled
	currentEnabled := true
	if current.Enabled != nil {
		currentEnabled = *current.Enabled
	}
	desiredEnabled := p.extractConsumePolicyEnabled(desired)
	if currentEnabled != desiredEnabled {
		needsUpdate = true
		changes[FieldEnabled] = FieldChange{Old: currentEnabled, New: desiredEnabled}
	}

	// Compare labels
	desiredLabels := p.extractConsumePolicyLabels(desired)
	if desiredLabels != nil {
		if !compareMaps(current.NormalizedLabels, desiredLabels) {
			needsUpdate = true
			changes[FieldLabels] = FieldChange{Old: current.NormalizedLabels, New: desiredLabels}
		}
	} else if len(current.NormalizedLabels) > 0 {
		needsUpdate = true
		changes[FieldLabels] = FieldChange{Old: current.NormalizedLabels, New: map[string]string{}}
	}

	// Compare config
	desiredConfig := p.extractConsumePolicyConfig(desired)
	if desiredConfig != nil && !configFieldsMatch(current.RawConfig, desiredConfig) {
		needsUpdate = true
		changes[FieldConfig] = FieldChange{Old: current.RawConfig, New: desiredConfig}
	}

	var updateFields map[string]any
	if needsUpdate {
		updateFields = p.consumePolicyToFields(desired)
		updateFields[FieldCurrentLabels] = current.NormalizedLabels
	}

	return needsUpdate, updateFields, changes
}
