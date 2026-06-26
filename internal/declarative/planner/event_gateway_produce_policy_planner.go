package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
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
	p.logger.Debug(
		"Planning Event Gateway Produce Policy changes",
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
	return p.planProducePolicyCreatesForNewVirtualCluster(
		namespace, gatewayRef, virtualClusterRef, virtualClusterName, virtualClusterChangeID, desired, plan,
	)
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
	p.logger.Debug(
		"Planning changes for existing virtual cluster produce policies",
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

	desired, err = p.prepareProducePolicyParentRefs(desired, currentByName)
	if err != nil {
		return err
	}

	desiredNames := make(map[string]bool)
	for _, desiredPolicy := range desired {
		policyName := desiredPolicy.GetMoniker()
		desiredNames[policyName] = true
		current, exists := currentByName[policyName]
		if !exists {
			p.logger.Debug(
				"Planning produce policy CREATE",
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
				if producePolicyRequiresRecreate(changedFields) {
					// Type and parent-policy changes are not supported by the API; force DELETE + CREATE.
					p.logger.Debug(
						"Planning produce policy DELETE+CREATE due to immutable field change",
						"policy_name", policyName,
						"policy_id", current.ID,
					)
					deleteID := p.planProducePolicyDelete(
						gatewayID, gatewayRef, virtualClusterID, virtualClusterRef,
						current.ID, policyName, plan,
					)
					p.planProducePolicyCreate(
						namespace, gatewayID, gatewayRef, virtualClusterID, virtualClusterRef, virtualClusterName,
						desiredPolicy, []string{deleteID}, plan,
					)
				} else {
					p.logger.Debug(
						"Planning produce policy UPDATE",
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
				p.logger.Debug(
					"Planning produce policy DELETE (sync mode)",
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
) error {
	p.logger.Debug(
		"Planning produce policy creates for new virtual cluster",
		"virtual_cluster_ref", virtualClusterRef,
		"virtual_cluster_change_id", virtualClusterChangeID,
		"policy_count", len(policies),
	)

	policies, err := p.prepareProducePolicyParentRefs(policies, nil)
	if err != nil {
		return err
	}

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
	return nil
}

func (p *Planner) prepareProducePolicyParentRefs(
	policies []resources.EventGatewayProducePolicyResource,
	currentByName map[string]state.EventGatewayVirtualClusterProducePolicyInfo,
) ([]resources.EventGatewayProducePolicyResource, error) {
	if len(policies) == 0 {
		return policies, nil
	}

	byRef := make(map[string]resources.EventGatewayProducePolicyResource, len(policies))
	for _, policy := range policies {
		byRef[policy.Ref] = policy
	}

	prepared := make([]resources.EventGatewayProducePolicyResource, len(policies))
	copy(prepared, policies)

	for i, policy := range prepared {
		parentPolicyID := producePolicyParentPolicyID(policy)
		if parentPolicyID == "" || !tags.IsRefPlaceholder(parentPolicyID) {
			continue
		}

		targetRef := referenceTargetRef(parentPolicyID)
		parentPolicy, ok := byRef[targetRef]
		if !ok {
			return nil, fmt.Errorf(
				"produce policy %q parent_policy_id references unknown policy ref %q",
				policy.GetMoniker(),
				targetRef,
			)
		}
		if !producePolicyIsSchemaValidation(parentPolicy) {
			return nil, fmt.Errorf(
				"produce policy %q parent_policy_id must reference a schema_validation produce policy, got %q",
				policy.GetMoniker(),
				producePolicyType(parentPolicy),
			)
		}

		if current, ok := currentByName[parentPolicy.GetMoniker()]; ok && current.ID != "" &&
			current.Type == string(kkComps.EventGatewayProducePolicyCreateTypeSchemaValidation) {
			prepared[i] = producePolicyWithParentPolicyID(policy, current.ID)
		}
	}

	return prepared, nil
}

func producePolicyRequiresRecreate(changedFields map[string]FieldChange) bool {
	if _, typeChanged := changedFields[FieldType]; typeChanged {
		return true
	}
	if _, parentPolicyChanged := changedFields[FieldParentPolicyID]; parentPolicyChanged {
		return true
	}
	return false
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
	p.addProducePolicyConfigReferences(&change)

	p.logger.Debug(
		"Enqueuing produce policy CREATE",
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
	p.addProducePolicyConfigReferences(&change)

	p.logger.Debug(
		"Enqueuing produce policy UPDATE",
		"policy_ref", policy.Ref,
		"policy_name", policy.GetMoniker(),
		"policy_id", policyID,
	)
	plan.AddChange(change)
}

func (p *Planner) addProducePolicyConfigReferences(change *PlannedChange) {
	if change == nil {
		return
	}
	p.addProducePolicyConfigReference(
		change,
		FieldConfig+".schema_registry."+FieldID,
		ResourceTypeEventGatewaySchemaRegistry,
	)
	p.addProducePolicyConfigReference(
		change,
		FieldConfig+".encryption_key.key."+FieldID,
		ResourceTypeEventGatewayStaticKey,
	)
	p.addProducePolicyEncryptFieldsConfigReferences(change)
	p.addProducePolicyParentPolicyReference(change)
}

func (p *Planner) addProducePolicyConfigReference(
	change *PlannedChange,
	fieldPath string,
	resourceType string,
) {
	ref, ok := stringValueAtFieldPath(change.Fields, fieldPath)
	if !ok || !tags.IsRefPlaceholder(ref) {
		return
	}

	refInfo := ReferenceInfo{
		Ref: ref,
		ID:  resources.UnknownReferenceID,
	}
	targetRef := referenceTargetRef(ref)
	if p.resolver != nil {
		if resource, exists := p.resolver.getResourceByTypeAndRef(resourceType, targetRef); exists {
			if moniker := resource.GetMoniker(); moniker != "" {
				refInfo.LookupFields = map[string]string{FieldName: moniker}
			}
		}
	}

	if change.References == nil {
		change.References = make(map[string]ReferenceInfo)
	}
	change.References[fieldPath] = refInfo
}

func (p *Planner) addProducePolicyEncryptFieldsConfigReferences(change *PlannedChange) {
	if change == nil || change.Fields == nil {
		return
	}

	config, ok := change.Fields[FieldConfig].(map[string]any)
	if !ok {
		return
	}
	encryptFields, ok := config["encrypt_fields"].([]any)
	if !ok {
		return
	}

	for i, field := range encryptFields {
		fieldMap, ok := field.(map[string]any)
		if !ok {
			continue
		}
		ref, ok := stringValueAtFieldPath(fieldMap, "encryption_key.key."+FieldID)
		if !ok || !tags.IsRefPlaceholder(ref) {
			continue
		}

		fieldPath := fmt.Sprintf("%s.encrypt_fields.%d.encryption_key.key.%s", FieldConfig, i, FieldID)
		p.addProducePolicyConfigReference(change, fieldPath, ResourceTypeEventGatewayStaticKey)
	}
}

func (p *Planner) addProducePolicyParentPolicyReference(change *PlannedChange) {
	if change == nil || change.Fields == nil {
		return
	}

	parentPolicyID, ok := change.Fields[FieldParentPolicyID].(string)
	if !ok || !tags.IsRefPlaceholder(parentPolicyID) {
		return
	}

	refInfo := ReferenceInfo{
		Ref: parentPolicyID,
		ID:  resources.UnknownReferenceID,
	}
	targetRef := referenceTargetRef(parentPolicyID)
	if p.resolver != nil {
		if resource, exists := p.resolver.getResourceByTypeAndRef(ResourceTypeEventGatewayProducePolicy, targetRef); exists {
			if moniker := resource.GetMoniker(); moniker != "" {
				refInfo.LookupFields = map[string]string{FieldName: moniker}
			}
		}
	}

	if change.References == nil {
		change.References = make(map[string]ReferenceInfo)
	}
	change.References[FieldParentPolicyID] = refInfo
}

func stringValueAtFieldPath(fields map[string]any, fieldPath string) (string, bool) {
	if fields == nil || fieldPath == "" {
		return "", false
	}
	if value, ok := fields[fieldPath].(string); ok {
		return value, true
	}

	var current any = fields
	for segment := range strings.SplitSeq(fieldPath, ".") {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return "", false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) {
				return "", false
			}
			current = typed[index]
		default:
			return "", false
		}
	}

	value, ok := current.(string)
	return value, ok
}

func (p *Planner) planProducePolicyDelete(
	gatewayID string,
	gatewayRef string,
	virtualClusterID string,
	virtualClusterRef string,
	policyID string,
	policyName string,
	plan *Plan,
) string {
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

	p.logger.Debug(
		"Enqueuing produce policy DELETE",
		"policy_name", policyName,
		"policy_id", policyID,
	)
	plan.AddChange(change)
	return change.ID
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
	} else if policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate != nil {
		variantData, err = json.Marshal(policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate)
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
		fields[FieldLabels] = lbl
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
	if policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate != nil &&
		policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate.Labels != nil {
		return policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate.Labels
	}
	return nil
}

func producePolicyType(policy resources.EventGatewayProducePolicyResource) string {
	if policy.EventGatewayModifyHeadersPolicyCreate != nil {
		return policy.EventGatewayModifyHeadersPolicyCreate.GetType()
	}
	if policy.EventGatewayProduceSchemaValidationPolicy != nil {
		return policy.EventGatewayProduceSchemaValidationPolicy.GetType()
	}
	if policy.EventGatewayEncryptPolicy != nil {
		return policy.EventGatewayEncryptPolicy.GetType()
	}
	if policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate != nil {
		return policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate.GetType()
	}
	return ""
}

func producePolicyIsSchemaValidation(policy resources.EventGatewayProducePolicyResource) bool {
	return policy.EventGatewayProduceSchemaValidationPolicy != nil
}

func producePolicyParentPolicyID(policy resources.EventGatewayProducePolicyResource) string {
	if policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate == nil {
		return ""
	}
	return policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate.ParentPolicyID
}

func producePolicyWithParentPolicyID(
	policy resources.EventGatewayProducePolicyResource,
	parentPolicyID string,
) resources.EventGatewayProducePolicyResource {
	if policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate == nil {
		return policy
	}
	variant := *policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate
	variant.ParentPolicyID = parentPolicyID
	policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate = &variant
	return policy
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
	} else if desired.EventGatewayParsedRecordEncryptFieldsPolicyCreate != nil {
		desiredType = desired.EventGatewayParsedRecordEncryptFieldsPolicyCreate.GetType()
	}
	if currentType != desiredType {
		needsUpdate = true
		changes[FieldType] = FieldChange{Old: currentType, New: desiredType}
	}

	desiredParentPolicyID := producePolicyParentPolicyID(desired)
	if desiredParentPolicyID != "" {
		currentParentPolicyID := ""
		if current.ParentPolicyID != nil {
			currentParentPolicyID = *current.ParentPolicyID
		}
		if currentParentPolicyID != desiredParentPolicyID {
			needsUpdate = true
			changes[FieldParentPolicyID] = FieldChange{Old: currentParentPolicyID, New: desiredParentPolicyID}
		}
	}

	desiredName := desired.GetMoniker()
	currentName := ""
	if current.Name != nil && *current.Name != "" {
		currentName = *current.Name
	}
	if currentName != desiredName {
		needsUpdate = true
		changes[FieldName] = FieldChange{Old: currentName, New: desiredName}
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
	} else if desired.EventGatewayParsedRecordEncryptFieldsPolicyCreate != nil &&
		desired.EventGatewayParsedRecordEncryptFieldsPolicyCreate.Description != nil {
		desiredDesc = *desired.EventGatewayParsedRecordEncryptFieldsPolicyCreate.Description
	}
	if currentDesc != desiredDesc {
		needsUpdate = true
		changes[FieldDescription] = FieldChange{Old: currentDesc, New: desiredDesc}
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
	} else if desired.EventGatewayParsedRecordEncryptFieldsPolicyCreate != nil &&
		desired.EventGatewayParsedRecordEncryptFieldsPolicyCreate.Enabled != nil {
		desiredEnabled = *desired.EventGatewayParsedRecordEncryptFieldsPolicyCreate.Enabled
	}
	if currentEnabled != desiredEnabled {
		needsUpdate = true
		changes[FieldEnabled] = FieldChange{Old: currentEnabled, New: desiredEnabled}
	}

	// Config comparison
	desiredConfig := p.extractProducePolicyConfig(desired)
	if !configFieldsMatch(current.RawConfig, desiredConfig) {
		needsUpdate = true
		changes[FieldConfig] = FieldChange{Old: current.RawConfig, New: desiredConfig}
	}

	// Labels comparison
	desiredLabels := extractProducePolicyVariantLabels(desired)
	if desiredLabels != nil {
		if !compareMaps(current.Labels, desiredLabels) {
			needsUpdate = true
			changes[FieldLabels] = FieldChange{Old: current.Labels, New: desiredLabels}
		}
	} else if len(current.Labels) > 0 {
		needsUpdate = true
		changes[FieldLabels] = FieldChange{Old: current.Labels, New: map[string]string{}}
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
	} else if policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate != nil {
		cfg = policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate.Config
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
