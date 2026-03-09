package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// planEventGatewayClusterPolicyChanges plans changes for Event Gateway Cluster Policies
// for a specific virtual cluster within a specific gateway.
func (p *Planner) planEventGatewayClusterPolicyChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayID string,
	gatewayRef string,
	virtualClusterName string,
	virtualClusterID string,
	virtualClusterRef string,
	virtualClusterChangeID string,
	desired []resources.EventGatewayClusterPolicyResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning Event Gateway Cluster Policy changes",
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
		// Virtual cluster exists: full diff
		return p.planClusterPolicyChangesForExistingVirtualCluster(
			ctx, namespace, gatewayID, gatewayRef, virtualClusterID, virtualClusterRef, virtualClusterName,
			desired, plan,
		)
	}

	// Virtual cluster doesn't exist yet: plan creates only with dependency on virtual cluster creation
	p.planClusterPolicyCreatesForNewVirtualCluster(
		namespace, gatewayRef, virtualClusterRef, virtualClusterName, virtualClusterChangeID, desired, plan,
	)
	return nil
}

// planClusterPolicyChangesForExistingVirtualCluster handles full diff for cluster policies
// when both the gateway and virtual cluster already exist.
func (p *Planner) planClusterPolicyChangesForExistingVirtualCluster(
	ctx context.Context,
	namespace string,
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	virtualClusterName string,
	desired []resources.EventGatewayClusterPolicyResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning changes for existing virtual cluster policies",
		"gateway_id", gatewayID,
		"virtual_cluster_id", virtualClusterID,
		"virtual_cluster_ref", virtualClusterRef,
		"desired_count", len(desired),
	)

	// 1. List current policies for this virtual cluster
	currentPolicies, err := p.client.ListEventGatewayClusterPolicies(ctx, gatewayID, virtualClusterID)
	if err != nil {
		return fmt.Errorf("failed to list cluster policies for virtual cluster %s: %w", virtualClusterID, err)
	}

	p.logger.Debug("Fetched current cluster policies",
		"virtual_cluster_id", virtualClusterID,
		"current_count", len(currentPolicies),
	)

	// 2. Index by name
	currentByName := make(map[string]state.EventGatewayClusterPolicyInfo)
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
			// CREATE
			p.logger.Debug("Planning cluster policy CREATE",
				"policy_name", policyName,
				"virtual_cluster_ref", virtualClusterRef,
			)
			p.planClusterPolicyCreate(
				namespace, gatewayID, gatewayRef, virtualClusterID, virtualClusterRef, virtualClusterName,
				desiredPolicy, []string{}, plan,
			)
		} else {
			// CHECK UPDATE
			p.logger.Debug("Checking if cluster policy needs update",
				"policy_name", policyName,
				"policy_id", current.ID,
			)

			needsUpdate, updateFields, changedFields := p.shouldUpdateClusterPolicy(current, desiredPolicy)
			if needsUpdate {
				p.logger.Debug("Planning cluster policy UPDATE",
					"policy_name", policyName,
					"policy_id", current.ID,
					"update_fields", updateFields,
					"changed_fields", changedFields,
				)
				p.planClusterPolicyUpdate(
					namespace, gatewayID, gatewayRef, virtualClusterID, virtualClusterRef,
					current.ID, desiredPolicy, updateFields, changedFields, plan,
				)
			}
		}
	}

	// 4. SYNC MODE: Delete unmanaged policies
	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				p.logger.Debug("Planning cluster policy DELETE (sync mode)",
					"policy_name", name,
					"policy_id", current.ID,
				)
				p.planClusterPolicyDelete(
					gatewayID, gatewayRef, virtualClusterID, virtualClusterRef,
					current.ID, name, plan,
				)
			}
		}
	}

	return nil
}

// planClusterPolicyCreatesForNewVirtualCluster plans creates for cluster policies
// when the parent virtual cluster doesn't exist yet.
func (p *Planner) planClusterPolicyCreatesForNewVirtualCluster(
	namespace string,
	gatewayRef string,
	virtualClusterRef string,
	virtualClusterName string,
	virtualClusterChangeID string,
	policies []resources.EventGatewayClusterPolicyResource,
	plan *Plan,
) {
	p.logger.Debug("Planning cluster policy creates for new virtual cluster",
		"virtual_cluster_ref", virtualClusterRef,
		"virtual_cluster_change_id", virtualClusterChangeID,
		"policy_count", len(policies),
	)

	// Build dependencies - policies depend on virtual cluster being created first
	var dependsOn []string
	if virtualClusterChangeID != "" {
		dependsOn = []string{virtualClusterChangeID}
	}

	for _, policy := range policies {
		// Gateway ID is empty because gateway may also be new
		p.planClusterPolicyCreate(
			namespace, "", gatewayRef, "", virtualClusterRef, virtualClusterName,
			policy, dependsOn, plan,
		)
	}
}

// planClusterPolicyCreate plans a CREATE change for a cluster policy
func (p *Planner) planClusterPolicyCreate(
	namespace string,
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	virtualClusterName string,
	policy resources.EventGatewayClusterPolicyResource,
	dependsOn []string,
	plan *Plan,
) {
	// Serialize the union type fields to a map for the executor
	fields := p.clusterPolicyToFields(policy)

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeEventGatewayClusterPolicy, policy.Ref),
		ResourceType: ResourceTypeEventGatewayClusterPolicy,
		ResourceRef:  policy.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}

	// Set parent and references based on what we know
	if virtualClusterID != "" {
		change.Parent = &ParentInfo{
			Ref: virtualClusterRef,
			ID:  virtualClusterID,
		}
	}

	// Both gateway and virtual cluster references are needed for the grandchild pattern
	change.References = map[string]ReferenceInfo{
		"event_gateway_id": {
			Ref: gatewayRef,
			ID:  gatewayID, // may be empty if gateway doesn't exist yet
		},
		"event_gateway_virtual_cluster_id": {
			Ref: virtualClusterRef,
			ID:  virtualClusterID, // may be empty if virtual cluster doesn't exist yet
			LookupFields: map[string]string{
				"name": virtualClusterName,
			},
		},
	}

	p.logger.Debug("Enqueuing cluster policy CREATE",
		"policy_ref", policy.Ref,
		"policy_name", policy.GetMoniker(),
		"virtual_cluster_ref", virtualClusterRef,
		"gateway_ref", gatewayRef,
	)
	plan.AddChange(change)
}

// planClusterPolicyUpdate plans an UPDATE change for a cluster policy
func (p *Planner) planClusterPolicyUpdate(
	namespace string,
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	policyID string,
	policy resources.EventGatewayClusterPolicyResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	if len(updateFields) == 0 {
		return
	}

	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeEventGatewayClusterPolicy, policy.Ref),
		ResourceType:  ResourceTypeEventGatewayClusterPolicy,
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
			"event_gateway_id": {
				Ref: gatewayRef,
				ID:  gatewayID,
			},
			"event_gateway_virtual_cluster_id": {
				Ref: virtualClusterRef,
				ID:  virtualClusterID,
			},
		},
	}

	p.logger.Debug("Enqueuing cluster policy UPDATE",
		"policy_ref", policy.Ref,
		"policy_name", policy.GetMoniker(),
		"policy_id", policyID,
	)
	plan.AddChange(change)
}

// planClusterPolicyDelete plans a DELETE change for a cluster policy
func (p *Planner) planClusterPolicyDelete(
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	policyID string,
	policyName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeEventGatewayClusterPolicy, policyName),
		ResourceType: ResourceTypeEventGatewayClusterPolicy,
		ResourceRef:  policyName,
		ResourceID:   policyID,
		Action:       ActionDelete,
		Parent: &ParentInfo{
			Ref: virtualClusterRef,
			ID:  virtualClusterID,
		},
		References: map[string]ReferenceInfo{
			"event_gateway_id": {
				Ref: gatewayRef,
				ID:  gatewayID,
			},
			"event_gateway_virtual_cluster_id": {
				Ref: virtualClusterRef,
				ID:  virtualClusterID,
			},
		},
	}

	p.logger.Debug("Enqueuing cluster policy DELETE",
		"policy_name", policyName,
		"policy_id", policyID,
	)
	plan.AddChange(change)
}

// clusterPolicyToFields converts a cluster policy resource to a fields map.
// Since the resource embeds a union type, we serialize to JSON and back to map.
func (p *Planner) clusterPolicyToFields(policy resources.EventGatewayClusterPolicyResource) map[string]any {
	// Marshal the embedded union type to JSON
	data, err := json.Marshal(policy.EventGatewayClusterPolicyModify)
	if err != nil {
		p.logger.Warn("Failed to marshal cluster policy to fields", "error", err)
		return map[string]any{}
	}

	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		p.logger.Warn("Failed to unmarshal cluster policy fields", "error", err)
		return map[string]any{}
	}

	// Add labels from the union variant if present
	if labels := p.extractClusterPolicyLabels(policy); labels != nil {
		fields["labels"] = labels
	}

	return fields
}

// extractClusterPolicyLabels extracts labels from whichever union variant is set
func (p *Planner) extractClusterPolicyLabels(
	policy resources.EventGatewayClusterPolicyResource,
) map[string]string {
	if policy.EventGatewayACLsPolicy != nil && policy.EventGatewayACLsPolicy.Labels != nil {
		return policy.EventGatewayACLsPolicy.Labels
	}
	return nil
}

// shouldUpdateClusterPolicy compares current and desired cluster policy state
func (p *Planner) shouldUpdateClusterPolicy(
	current state.EventGatewayClusterPolicyInfo,
	desired resources.EventGatewayClusterPolicyResource,
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
		changes["name"] = FieldChange{
			Old: currentName,
			New: desiredName,
		}
	}

	// Compare description
	currentDesc := ""
	if current.Description != nil {
		currentDesc = *current.Description
	}
	desiredDesc := p.getClusterPolicyDescription(desired)
	if currentDesc != desiredDesc {
		needsUpdate = true
		changes["description"] = FieldChange{
			Old: currentDesc,
			New: desiredDesc,
		}
	}

	// Compare enabled
	currentEnabled := true
	if current.Enabled != nil {
		currentEnabled = *current.Enabled
	}
	desiredEnabled := p.getClusterPolicyEnabled(desired)
	if currentEnabled != desiredEnabled {
		needsUpdate = true
		changes["enabled"] = FieldChange{
			Old: currentEnabled,
			New: desiredEnabled,
		}
	}

	// Compare labels
	desiredLabels := p.extractClusterPolicyLabels(desired)
	if desiredLabels != nil {
		if !compareMaps(current.NormalizedLabels, desiredLabels) {
			needsUpdate = true
			changes["labels"] = FieldChange{
				Old: current.NormalizedLabels,
				New: desiredLabels,
			}
		}
	} else if len(current.NormalizedLabels) > 0 {
		needsUpdate = true
		changes["labels"] = FieldChange{
			Old: current.NormalizedLabels,
			New: map[string]string{},
		}
	}

	// Compare config
	desiredConfig := p.extractClusterPolicyConfig(desired)
	if !compareAnyMaps(current.RawConfig, desiredConfig) {
		needsUpdate = true
		changes["config"] = FieldChange{
			Old: current.RawConfig,
			New: desiredConfig,
		}
	}

	// If there are changes, build full update fields
	var updateFields map[string]any
	if needsUpdate {
		updateFields = p.clusterPolicyToFields(desired)
		// Include current labels for removal logic
		updateFields[FieldCurrentLabels] = current.NormalizedLabels
	}

	return needsUpdate, updateFields, changes
}

// getClusterPolicyDescription extracts description from whichever union variant is set
func (p *Planner) getClusterPolicyDescription(
	policy resources.EventGatewayClusterPolicyResource,
) string {
	if policy.EventGatewayACLsPolicy != nil && policy.EventGatewayACLsPolicy.Description != nil {
		return *policy.EventGatewayACLsPolicy.Description
	}
	return ""
}

// getClusterPolicyEnabled extracts enabled from whichever union variant is set
func (p *Planner) getClusterPolicyEnabled(
	policy resources.EventGatewayClusterPolicyResource,
) bool {
	if policy.EventGatewayACLsPolicy != nil && policy.EventGatewayACLsPolicy.Enabled != nil {
		return *policy.EventGatewayACLsPolicy.Enabled
	}
	return true // default is enabled
}

// extractClusterPolicyConfig extracts config from whichever union variant is set
// and converts it to a map[string]any for comparison
func (p *Planner) extractClusterPolicyConfig(
	policy resources.EventGatewayClusterPolicyResource,
) map[string]any {
	if policy.EventGatewayACLsPolicy == nil {
		return nil
	}

	// Marshal the config struct to JSON and back to map[string]any
	data, err := json.Marshal(policy.EventGatewayACLsPolicy.Config)
	if err != nil {
		return nil
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return nil
	}

	return config
}

// compareAnyMaps compares fields present in desired against current.
// Extra fields in current (e.g., new API defaults) are ignored to prevent
// unnecessary updates when API adds new fields.
func compareAnyMaps(current, desired map[string]any) bool {
	// Both nil/empty is equal
	currentEmpty := len(current) == 0 || isEffectivelyEmpty(current)
	desiredEmpty := len(desired) == 0 || isEffectivelyEmpty(desired)
	if currentEmpty && desiredEmpty {
		return true
	}

	// If desired is empty but current has values, no update needed
	// (user hasn't specified config, keep what API has)
	if desiredEmpty {
		return true
	}

	// If current is empty but desired has values, update needed
	if currentEmpty {
		return false
	}

	// Compare only fields present in desired
	return configFieldsMatch(current, desired)
}

// configFieldsMatch recursively checks if all fields in desired exist and match in current.
// Extra fields in current are ignored (handles API adding new fields with defaults).
func configFieldsMatch(current, desired map[string]any) bool {
	for key, desiredVal := range desired {
		currentVal, exists := current[key]
		if !exists {
			return false
		}

		// Recursive comparison for nested maps
		desiredMap, desiredIsMap := desiredVal.(map[string]any)
		currentMap, currentIsMap := currentVal.(map[string]any)
		if desiredIsMap && currentIsMap {
			if !configFieldsMatch(currentMap, desiredMap) {
				return false
			}
			continue
		}

		// For slices, use DeepEqual (order matters for rules)
		if !reflect.DeepEqual(currentVal, desiredVal) {
			return false
		}
	}
	return true
}

// isEffectivelyEmpty checks if a config map is effectively empty
// (e.g., only contains empty slices or nil values)
func isEffectivelyEmpty(m map[string]any) bool {
	for _, v := range m {
		switch val := v.(type) {
		case nil:
			continue
		case []any:
			if len(val) > 0 {
				return false
			}
		case map[string]any:
			if !isEffectivelyEmpty(val) {
				return false
			}
		default:
			return false
		}
	}
	return true
}
