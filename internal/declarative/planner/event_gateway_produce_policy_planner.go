package planner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// planEventGatewayVirtualClusterProducePolicyChanges plans changes for Event Gateway Produce Policies
// for a specific virtual cluster within a specific gateway.
func (p *Planner) planEventGatewayVirtualClusterProducePolicyChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayID string,
	gatewayRef string,
	virtualClusterName string,
	virtualClusterID string,
	virtualClusterRef string,
	virtualClusterChangeID string,
	desired []resources.EventGatewayProducePolicyResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning Event Gateway Produce Policy changes",
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
		return p.planProducePolicyChangesForExistingVirtualCluster(
			ctx, namespace, gatewayID, gatewayRef, virtualClusterID, virtualClusterRef, virtualClusterName,
			desired, plan,
		)
	}

	// Parent doesn't exist yet: plan creates only with dependency
	p.planProducePolicyCreatesForNewVirtualCluster(
		namespace, gatewayRef, virtualClusterRef, virtualClusterName, virtualClusterChangeID, desired, plan,
	)
	return nil
}

func (p *Planner) planProducePolicyChangesForExistingVirtualCluster(
	ctx context.Context,
	namespace string,
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	virtualClusterName string,
	desired []resources.EventGatewayProducePolicyResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning changes for existing virtual cluster produce policies",
		"gateway_id", gatewayID,
		"virtual_cluster_id", virtualClusterID,
		"virtual_cluster_ref", virtualClusterRef,
		"desired_count", len(desired),
	)

	currentPolicies, err := p.client.ListEventGatewayVirtualClusterProducePolicies(ctx, gatewayID, virtualClusterID)
	if err != nil {
		return fmt.Errorf("failed to list produce policies for virtual cluster %s: %w", virtualClusterID, err)
	}

	currentByName := make(map[string]state.EventGatewayVirtualClusterProducePolicyInfo)
	for _, policy := range currentPolicies {
		if policy.Name != nil && *policy.Name != "" {
			currentByName[*policy.Name] = policy
		}
	}

	desiredNames := make(map[string]bool)
	for _, desiredPolicy := range desired {
		policyName := desiredPolicy.GetMoniker()
		desiredNames[policyName] = true
		current, exists := currentByName[policyName]
		if !exists {
			p.logger.Debug("Planning produce policy CREATE",
				"policy_name", policyName,
				"virtual_cluster_ref", virtualClusterRef,
			)
			p.planProducePolicyCreate(
				namespace, gatewayID, gatewayRef, virtualClusterID, virtualClusterRef, virtualClusterName,
				desiredPolicy, []string{}, plan,
			)
		} else {
			needsUpdate, updateFields, changedFields := p.shouldUpdateProducePolicy(current, desiredPolicy)
			if needsUpdate {
				if _, typeChanged := changedFields["type"]; typeChanged {
					// Type changes are not supported by the API; force DELETE + CREATE.
					p.logger.Debug("Planning produce policy DELETE+CREATE due to type change",
						"policy_name", policyName,
						"policy_id", current.ID,
					)
					p.planProducePolicyDelete(
						gatewayID, gatewayRef, virtualClusterID, virtualClusterRef,
						current.ID, policyName, plan,
					)
					p.planProducePolicyCreate(
						namespace, gatewayID, gatewayRef, virtualClusterID, virtualClusterRef, virtualClusterName,
						desiredPolicy, []string{}, plan,
					)
				} else {
					p.logger.Debug("Planning produce policy UPDATE",
						"policy_name", policyName,
						"policy_id", current.ID,
						"update_fields", updateFields,
					)
					p.planProducePolicyUpdate(
						namespace, gatewayID, gatewayRef, virtualClusterID, virtualClusterRef,
						current.ID, desiredPolicy, updateFields, changedFields, plan,
					)
				}
			}
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				p.logger.Debug("Planning produce policy DELETE (sync mode)",
					"policy_name", name,
					"policy_id", current.ID,
				)
				p.planProducePolicyDelete(
					gatewayID, gatewayRef, virtualClusterID, virtualClusterRef,
					current.ID, name, plan,
				)
			}
		}
	}

	return nil
}

func (p *Planner) planProducePolicyCreatesForNewVirtualCluster(
	namespace string,
	gatewayRef string,
	virtualClusterRef string,
	virtualClusterName string,
	virtualClusterChangeID string,
	policies []resources.EventGatewayProducePolicyResource,
	plan *Plan,
) {
	p.logger.Debug("Planning produce policy creates for new virtual cluster",
		"virtual_cluster_ref", virtualClusterRef,
		"virtual_cluster_change_id", virtualClusterChangeID,
		"policy_count", len(policies),
	)

	var dependsOn []string
	if virtualClusterChangeID != "" {
		dependsOn = []string{virtualClusterChangeID}
	}

	for _, policy := range policies {
		p.planProducePolicyCreate(
			namespace, "", gatewayRef, "", virtualClusterRef, virtualClusterName,
			policy, dependsOn, plan,
		)
	}
}

func (p *Planner) planProducePolicyCreate(
	namespace string,
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	virtualClusterName string,
	policy resources.EventGatewayProducePolicyResource,
	dependsOn []string,
	plan *Plan,
) {
	fields := p.producePolicyToFields(policy)

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeEventGatewayProducePolicy, policy.Ref),
		ResourceType: ResourceTypeEventGatewayProducePolicy,
		ResourceRef:  policy.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}

	if virtualClusterID != "" {
		change.Parent = &ParentInfo{Ref: virtualClusterRef, ID: virtualClusterID}
	}

	change.References = map[string]ReferenceInfo{
		"event_gateway_id": {
			Ref: gatewayRef,
			ID:  gatewayID,
		},
		"event_gateway_virtual_cluster_id": {
			Ref: virtualClusterRef,
			ID:  virtualClusterID,
			LookupFields: map[string]string{
				"name": virtualClusterName,
			},
		},
	}

	p.logger.Debug("Enqueuing produce policy CREATE",
		"policy_ref", policy.Ref,
		"policy_name", policy.GetMoniker(),
	)
	plan.AddChange(change)
}

func (p *Planner) planProducePolicyUpdate(
	namespace string,
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	policyID string,
	policy resources.EventGatewayProducePolicyResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	if len(updateFields) == 0 {
		return
	}

	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeEventGatewayProducePolicy, policy.Ref),
		ResourceType:  ResourceTypeEventGatewayProducePolicy,
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

	p.logger.Debug("Enqueuing produce policy UPDATE",
		"policy_ref", policy.Ref,
		"policy_name", policy.GetMoniker(),
		"policy_id", policyID,
	)
	plan.AddChange(change)
}

func (p *Planner) planProducePolicyDelete(
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	policyID string,
	policyName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeEventGatewayProducePolicy, policyName),
		ResourceType: ResourceTypeEventGatewayProducePolicy,
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

	p.logger.Debug("Enqueuing produce policy DELETE",
		"policy_name", policyName,
		"policy_id", policyID,
	)
	plan.AddChange(change)
}

func (p *Planner) producePolicyToFields(policy resources.EventGatewayProducePolicyResource) map[string]any {
	// Marshal the active variant directly to get a complete field map including
	// the "type" discriminator, config, and all other fields. Bypassing the union
	// wrapper avoids the "all fields null" error when the variant pointer is nil.
	var (
		variantData []byte
		err         error
	)
	if policy.EventGatewayModifyHeadersPolicyCreate != nil {
		variantData, err = json.Marshal(policy.EventGatewayModifyHeadersPolicyCreate)
	} else if policy.EventGatewayProduceSchemaValidationPolicy != nil {
		variantData, err = json.Marshal(policy.EventGatewayProduceSchemaValidationPolicy)
	} else if policy.EventGatewayEncryptPolicy != nil {
		variantData, err = json.Marshal(policy.EventGatewayEncryptPolicy)
	}

	if err != nil {
		p.logger.Warn("Failed to marshal produce policy to fields", "error", err)
		return map[string]any{}
	}
	if variantData == nil {
		p.logger.Warn("No active variant found for produce policy", "ref", policy.Ref)
		return map[string]any{}
	}

	var fields map[string]any
	if err := json.Unmarshal(variantData, &fields); err != nil {
		p.logger.Warn("Failed to unmarshal produce policy fields", "error", err)
		return map[string]any{}
	}

	// Explicitly add labels from the active variant (mirrors clusterPolicyToFields).
	if lbl := extractProducePolicyVariantLabels(policy); lbl != nil {
		fields["labels"] = lbl
	}

	return fields
}

// extractProducePolicyVariantLabels extracts labels from whichever union variant is set.
func extractProducePolicyVariantLabels(policy resources.EventGatewayProducePolicyResource) map[string]string {
	if policy.EventGatewayModifyHeadersPolicyCreate != nil &&
		policy.EventGatewayModifyHeadersPolicyCreate.Labels != nil {
		return policy.EventGatewayModifyHeadersPolicyCreate.Labels
	}
	if policy.EventGatewayProduceSchemaValidationPolicy != nil &&
		policy.EventGatewayProduceSchemaValidationPolicy.Labels != nil {
		return policy.EventGatewayProduceSchemaValidationPolicy.Labels
	}
	if policy.EventGatewayEncryptPolicy != nil && policy.EventGatewayEncryptPolicy.Labels != nil {
		return policy.EventGatewayEncryptPolicy.Labels
	}
	return nil
}

func (p *Planner) shouldUpdateProducePolicy(
	current state.EventGatewayVirtualClusterProducePolicyInfo,
	desired resources.EventGatewayProducePolicyResource,
) (bool, map[string]any, map[string]FieldChange) {
	var needsUpdate bool
	changes := make(map[string]FieldChange)

	// Type comparison - type changes are not supported by the API and require a DELETE+CREATE.
	currentType := current.Type
	desiredType := ""
	if desired.EventGatewayModifyHeadersPolicyCreate != nil {
		desiredType = desired.EventGatewayModifyHeadersPolicyCreate.GetType()
	} else if desired.EventGatewayProduceSchemaValidationPolicy != nil {
		desiredType = desired.EventGatewayProduceSchemaValidationPolicy.GetType()
	} else if desired.EventGatewayEncryptPolicy != nil {
		desiredType = desired.EventGatewayEncryptPolicy.GetType()
	}
	if currentType != desiredType {
		needsUpdate = true
		changes["type"] = FieldChange{Old: currentType, New: desiredType}
	}

	desiredName := desired.GetMoniker()
	currentName := ""
	if current.Name != nil && *current.Name != "" {
		currentName = *current.Name
	}
	if currentName != desiredName {
		needsUpdate = true
		changes["name"] = FieldChange{Old: currentName, New: desiredName}
	}

	// Description comparison (if present) - use the union discriminator to pick the desired variant
	currentDesc := ""
	if current.Description != nil {
		currentDesc = *current.Description
	}
	desiredDesc := ""
	// Check each known variant for description
	if desired.EventGatewayModifyHeadersPolicyCreate != nil &&
		desired.EventGatewayModifyHeadersPolicyCreate.Description != nil {
		desiredDesc = *desired.EventGatewayModifyHeadersPolicyCreate.Description
	} else if desired.EventGatewayProduceSchemaValidationPolicy != nil &&
		desired.EventGatewayProduceSchemaValidationPolicy.Description != nil {
		desiredDesc = *desired.EventGatewayProduceSchemaValidationPolicy.Description
	} else if desired.EventGatewayEncryptPolicy != nil && desired.EventGatewayEncryptPolicy.Description != nil {
		desiredDesc = *desired.EventGatewayEncryptPolicy.Description
	}
	if currentDesc != desiredDesc {
		needsUpdate = true
		changes["description"] = FieldChange{Old: currentDesc, New: desiredDesc}
	}

	// Enabled - variant-aware
	currentEnabled := true
	if current.Enabled != nil {
		currentEnabled = *current.Enabled
	}
	desiredEnabled := true
	if desired.EventGatewayModifyHeadersPolicyCreate != nil &&
		desired.EventGatewayModifyHeadersPolicyCreate.Enabled != nil {
		desiredEnabled = *desired.EventGatewayModifyHeadersPolicyCreate.Enabled
	} else if desired.EventGatewayProduceSchemaValidationPolicy != nil &&
		desired.EventGatewayProduceSchemaValidationPolicy.Enabled != nil {
		desiredEnabled = *desired.EventGatewayProduceSchemaValidationPolicy.Enabled
	} else if desired.EventGatewayEncryptPolicy != nil && desired.EventGatewayEncryptPolicy.Enabled != nil {
		desiredEnabled = *desired.EventGatewayEncryptPolicy.Enabled
	}
	if currentEnabled != desiredEnabled {
		needsUpdate = true
		changes["enabled"] = FieldChange{Old: currentEnabled, New: desiredEnabled}
	}

	// Config comparison
	desiredConfig := p.extractProducePolicyConfig(desired)
	if !configFieldsMatch(current.RawConfig, desiredConfig) {
		needsUpdate = true
		changes["config"] = FieldChange{Old: current.RawConfig, New: desiredConfig}
	}

	// Labels comparison
	desiredLabels := extractProducePolicyVariantLabels(desired)
	if desiredLabels != nil {
		if !compareMaps(current.Labels, desiredLabels) {
			needsUpdate = true
			changes["labels"] = FieldChange{Old: current.Labels, New: desiredLabels}
		}
	} else if len(current.Labels) > 0 {
		needsUpdate = true
		changes["labels"] = FieldChange{Old: current.Labels, New: map[string]string{}}
	}

	var updateFields map[string]any
	if needsUpdate {
		updateFields = p.producePolicyToFields(desired)
		// Include current labels to allow the executor to remove labels when needed
		updateFields[FieldCurrentLabels] = current.Labels
	}

	return needsUpdate, updateFields, changes
}

func (p *Planner) extractProducePolicyConfig(policy resources.EventGatewayProducePolicyResource) map[string]any {
	// Config fields are value types on each variant; marshal the active variant's config.
	var cfg any
	if policy.EventGatewayModifyHeadersPolicyCreate != nil {
		cfg = policy.EventGatewayModifyHeadersPolicyCreate.Config
	} else if policy.EventGatewayProduceSchemaValidationPolicy != nil {
		cfg = policy.EventGatewayProduceSchemaValidationPolicy.Config
	} else if policy.EventGatewayEncryptPolicy != nil {
		cfg = policy.EventGatewayEncryptPolicy.Config
	}

	if cfg == nil {
		return nil
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}
