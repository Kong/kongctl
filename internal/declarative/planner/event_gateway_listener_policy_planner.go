package planner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// planEventGatewayListenerPolicyChanges plans changes for Event Gateway Listener Policies
// for a specific listener within a specific gateway.
func (p *Planner) planEventGatewayListenerPolicyChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayID string,
	gatewayRef string,
	listenerName string,
	listenerID string,
	listenerRef string,
	listenerChangeID string,
	desired []resources.EventGatewayListenerPolicyResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning Event Gateway Listener Policy changes",
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"listener_name", listenerName,
		"listener_id", listenerID,
		"listener_ref", listenerRef,
		"listener_change_id", listenerChangeID,
		"desired_count", len(desired),
		"namespace", namespace,
	)

	if listenerID != "" && gatewayID != "" {
		// Listener exists: full diff
		return p.planListenerPolicyChangesForExistingListener(
			ctx, namespace, gatewayID, gatewayRef, listenerID, listenerRef, listenerName, desired, plan,
		)
	}

	// Listener doesn't exist yet: plan creates only with dependency on listener creation
	p.planListenerPolicyCreatesForNewListener(
		namespace, gatewayRef, listenerRef, listenerName, listenerChangeID, desired, plan,
	)
	return nil
}

// planListenerPolicyChangesForExistingListener handles full diff for listener policies
// when both the gateway and listener already exist.
func (p *Planner) planListenerPolicyChangesForExistingListener(
	ctx context.Context,
	namespace string,
	gatewayID string,
	gatewayRef string,
	listenerID string,
	listenerRef string,
	listenerName string,
	desired []resources.EventGatewayListenerPolicyResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning changes for existing listener policies",
		"gateway_id", gatewayID,
		"listener_id", listenerID,
		"listener_ref", listenerRef,
		"desired_count", len(desired),
	)

	// 1. List current policies for this listener
	currentPolicies, err := p.client.ListEventGatewayListenerPolicies(ctx, gatewayID, listenerID)
	if err != nil {
		return fmt.Errorf("failed to list listener policies for listener %s: %w", listenerID, err)
	}

	p.logger.Debug("Fetched current listener policies",
		"listener_id", listenerID,
		"current_count", len(currentPolicies),
	)

	// 2. Index by name
	currentByName := make(map[string]state.EventGatewayListenerPolicyInfo)
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
			p.logger.Debug("Planning listener policy CREATE",
				"policy_name", policyName,
				"listener_ref", listenerRef,
			)
			p.planListenerPolicyCreate(
				namespace, gatewayID, gatewayRef, listenerID, listenerRef, listenerName,
				desiredPolicy, []string{}, plan,
			)
		} else {
			// CHECK UPDATE
			p.logger.Debug("Checking if listener policy needs update",
				"policy_name", policyName,
				"policy_id", current.ID,
			)

			needsUpdate, updateFields := p.shouldUpdateListenerPolicy(current, desiredPolicy)
			if needsUpdate {
				p.logger.Debug("Planning listener policy UPDATE",
					"policy_name", policyName,
					"policy_id", current.ID,
					"update_fields", updateFields,
				)
				p.planListenerPolicyUpdate(
					namespace, gatewayID, gatewayRef, listenerID, listenerRef,
					current.ID, desiredPolicy, updateFields, plan,
				)
			}
		}
	}

	// 4. SYNC MODE: Delete unmanaged policies
	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				p.logger.Debug("Planning listener policy DELETE (sync mode)",
					"policy_name", name,
					"policy_id", current.ID,
				)
				p.planListenerPolicyDelete(
					gatewayID, gatewayRef, listenerID, listenerRef,
					current.ID, name, plan,
				)
			}
		}
	}

	return nil
}

// planListenerPolicyCreatesForNewListener plans creates for listener policies
// when the parent listener doesn't exist yet.
func (p *Planner) planListenerPolicyCreatesForNewListener(
	namespace string,
	gatewayRef string,
	listenerRef string,
	listenerName string,
	listenerChangeID string,
	policies []resources.EventGatewayListenerPolicyResource,
	plan *Plan,
) {
	p.logger.Debug("Planning listener policy creates for new listener",
		"listener_ref", listenerRef,
		"listener_change_id", listenerChangeID,
		"policy_count", len(policies),
	)

	// Build dependencies - policies depend on listener being created first
	var dependsOn []string
	if listenerChangeID != "" {
		dependsOn = []string{listenerChangeID}
	}

	for _, policy := range policies {
		// Gateway ID is empty because gateway may also be new
		p.planListenerPolicyCreate(
			namespace, "", gatewayRef, "", listenerRef, listenerName,
			policy, dependsOn, plan,
		)
	}
}

// planListenerPolicyCreate plans a CREATE change for a listener policy
func (p *Planner) planListenerPolicyCreate(
	namespace string,
	gatewayID string,
	gatewayRef string,
	listenerID string,
	listenerRef string,
	listenerName string,
	policy resources.EventGatewayListenerPolicyResource,
	dependsOn []string,
	plan *Plan,
) {
	// Serialize the union type fields to a map for the executor
	fields := p.listenerPolicyToFields(policy)

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeEventGatewayListenerPolicy, policy.Ref),
		ResourceType: ResourceTypeEventGatewayListenerPolicy,
		ResourceRef:  policy.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}

	// Set parent and references based on what we know
	if listenerID != "" {
		change.Parent = &ParentInfo{
			Ref: listenerRef,
			ID:  listenerID,
		}
	}

	// Both gateway and listener references are needed for the grandchild pattern
	change.References = map[string]ReferenceInfo{
		"event_gateway_id": {
			Ref: gatewayRef,
			ID:  gatewayID, // may be empty if gateway doesn't exist yet
		},
		"event_gateway_listener_id": {
			Ref: listenerRef,
			ID:  listenerID, // may be empty if listener doesn't exist yet
			LookupFields: map[string]string{
				"name": listenerName,
			},
		},
	}

	p.logger.Debug("Enqueuing listener policy CREATE",
		"policy_ref", policy.Ref,
		"policy_name", policy.GetMoniker(),
		"listener_ref", listenerRef,
		"gateway_ref", gatewayRef,
	)
	plan.AddChange(change)
}

// planListenerPolicyUpdate plans an UPDATE change for a listener policy
func (p *Planner) planListenerPolicyUpdate(
	namespace string,
	gatewayID string,
	gatewayRef string,
	listenerID string,
	listenerRef string,
	policyID string,
	policy resources.EventGatewayListenerPolicyResource,
	updateFields map[string]any,
	plan *Plan,
) {
	if len(updateFields) == 0 {
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypeEventGatewayListenerPolicy, policy.Ref),
		ResourceType: ResourceTypeEventGatewayListenerPolicy,
		ResourceRef:  policy.Ref,
		ResourceID:   policyID,
		Action:       ActionUpdate,
		Fields:       updateFields,
		Namespace:    namespace,
		Parent: &ParentInfo{
			Ref: listenerRef,
			ID:  listenerID,
		},
		References: map[string]ReferenceInfo{
			"event_gateway_id": {
				Ref: gatewayRef,
				ID:  gatewayID,
			},
			"event_gateway_listener_id": {
				Ref: listenerRef,
				ID:  listenerID,
			},
		},
	}

	p.logger.Debug("Enqueuing listener policy UPDATE",
		"policy_ref", policy.Ref,
		"policy_name", policy.GetMoniker(),
		"policy_id", policyID,
	)
	plan.AddChange(change)
}

// planListenerPolicyDelete plans a DELETE change for a listener policy
func (p *Planner) planListenerPolicyDelete(
	gatewayID string,
	gatewayRef string,
	listenerID string,
	listenerRef string,
	policyID string,
	policyName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeEventGatewayListenerPolicy, policyName),
		ResourceType: ResourceTypeEventGatewayListenerPolicy,
		ResourceRef:  policyName,
		ResourceID:   policyID,
		Action:       ActionDelete,
		Parent: &ParentInfo{
			Ref: listenerRef,
			ID:  listenerID,
		},
		References: map[string]ReferenceInfo{
			"event_gateway_id": {
				Ref: gatewayRef,
				ID:  gatewayID,
			},
			"event_gateway_listener_id": {
				Ref: listenerRef,
				ID:  listenerID,
			},
		},
	}

	p.logger.Debug("Enqueuing listener policy DELETE",
		"policy_name", policyName,
		"policy_id", policyID,
	)
	plan.AddChange(change)
}

// listenerPolicyToFields converts a listener policy resource to a fields map.
// Since the resource embeds a union type, we serialize to JSON and back to map.
func (p *Planner) listenerPolicyToFields(policy resources.EventGatewayListenerPolicyResource) map[string]any {
	// Marshal the embedded union type to JSON
	data, err := json.Marshal(policy.EventGatewayListenerPolicyCreate)
	if err != nil {
		p.logger.Warn("Failed to marshal listener policy to fields", "error", err)
		return map[string]any{}
	}

	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		p.logger.Warn("Failed to unmarshal listener policy fields", "error", err)
		return map[string]any{}
	}

	// Add labels from the union variant if present
	if labels := p.extractListenerPolicyLabels(policy); labels != nil {
		fields["labels"] = labels
	}

	return fields
}

// extractListenerPolicyLabels extracts labels from whichever union variant is set
func (p *Planner) extractListenerPolicyLabels(
	policy resources.EventGatewayListenerPolicyResource,
) map[string]string {
	if policy.EventGatewayTLSListenerPolicy != nil && policy.EventGatewayTLSListenerPolicy.Labels != nil {
		return policy.EventGatewayTLSListenerPolicy.Labels
	}
	if policy.ForwardToVirtualClusterPolicy != nil && policy.ForwardToVirtualClusterPolicy.Labels != nil {
		return policy.ForwardToVirtualClusterPolicy.Labels
	}
	return nil
}

// shouldUpdateListenerPolicy compares current and desired listener policy state
func (p *Planner) shouldUpdateListenerPolicy(
	current state.EventGatewayListenerPolicyInfo,
	desired resources.EventGatewayListenerPolicyResource,
) (bool, map[string]any) {
	var needsUpdate bool

	// Compare name
	desiredName := desired.GetMoniker()
	currentName := ""
	if current.Name != nil {
		currentName = *current.Name
	}
	if currentName != desiredName {
		needsUpdate = true
	}

	// Compare description
	currentDesc := ""
	if current.Description != nil {
		currentDesc = *current.Description
	}
	desiredDesc := p.getListenerPolicyDescription(desired)
	if currentDesc != desiredDesc {
		needsUpdate = true
	}

	// Compare enabled
	currentEnabled := true
	if current.Enabled != nil {
		currentEnabled = *current.Enabled
	}
	desiredEnabled := p.getListenerPolicyEnabled(desired)
	if currentEnabled != desiredEnabled {
		needsUpdate = true
	}

	// Compare labels
	desiredLabels := p.extractListenerPolicyLabels(desired)
	if desiredLabels != nil {
		if !compareMaps(current.Labels, desiredLabels) {
			needsUpdate = true
		}
	} else if len(current.Labels) > 0 {
		needsUpdate = true
	}

	// If any changes detected, serialize ALL fields from desired state for PUT request
	if needsUpdate {
		return true, p.listenerPolicyToFields(desired)
	}

	return false, nil
}

// getListenerPolicyDescription extracts description from whichever union variant is set
func (p *Planner) getListenerPolicyDescription(
	policy resources.EventGatewayListenerPolicyResource,
) string {
	if policy.EventGatewayTLSListenerPolicy != nil && policy.EventGatewayTLSListenerPolicy.Description != nil {
		return *policy.EventGatewayTLSListenerPolicy.Description
	}
	if policy.ForwardToVirtualClusterPolicy != nil && policy.ForwardToVirtualClusterPolicy.Description != nil {
		return *policy.ForwardToVirtualClusterPolicy.Description
	}
	return ""
}

// getListenerPolicyEnabled extracts enabled from whichever union variant is set
func (p *Planner) getListenerPolicyEnabled(
	policy resources.EventGatewayListenerPolicyResource,
) bool {
	if policy.EventGatewayTLSListenerPolicy != nil && policy.EventGatewayTLSListenerPolicy.Enabled != nil {
		return *policy.EventGatewayTLSListenerPolicy.Enabled
	}
	if policy.ForwardToVirtualClusterPolicy != nil && policy.ForwardToVirtualClusterPolicy.Enabled != nil {
		return *policy.ForwardToVirtualClusterPolicy.Enabled
	}
	return true // Default is enabled
}
