package planner

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

type EGWControlPlanePlannerImpl struct {
	*BasePlanner
	resources *resources.ResourceSet
}

func NewEGWControlPlanePlanner(planner *BasePlanner, resources *resources.ResourceSet) *EGWControlPlanePlannerImpl {
	return &EGWControlPlanePlannerImpl{
		BasePlanner: planner,
		resources:   resources,
	}
}

func (p *EGWControlPlanePlannerImpl) PlannerComponent() string {
	return ResourceTypeEventGatewayControlPlane
}

func (p *EGWControlPlanePlannerImpl) GetDesiredEGWControlPlanes(
	namespace string,
) []resources.EventGatewayControlPlaneResource {
	return p.GetDesiredEventGatewayControlPlanes(namespace)
}

func (p *EGWControlPlanePlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	namespace := plannerCtx.Namespace
	desired := p.GetDesiredEGWControlPlanes(namespace)

	// Skip if no desired Event Gateway Control Planes and not in sync mode
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	return p.planner.planEGWControlPlaneChanges(ctx, plannerCtx, desired, plan)
}

func (p *Planner) planEGWControlPlaneChanges(
	ctx context.Context,
	plannerCtx *Config,
	desired []resources.EventGatewayControlPlaneResource,
	plan *Plan,
) error {
	// Skip if no Event Gateway Control Plane resources to plan and not in sync mode
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		p.logger.Debug("Skipping Event Gateway Control Plane planning - no desired Event Gateway Control Planes")
		return nil
	}

	// Get namespace from planner context
	namespace := plannerCtx.Namespace

	// Fetch current managed Event Gateway Control Planes from the specific namespace
	var currentEGWControlPlanes []state.EventGatewayControlPlane
	if namespace != resources.NamespaceExternal {
		namespaceFilter := []string{namespace}
		var err error
		currentEGWControlPlanes, err = p.listManagedEventGatewayControlPlanes(ctx, namespaceFilter)
		if err != nil {
			// If API client is not configured, skip Event Gateway Control Plane planning
			if state.IsAPIClientError(err) {
				return nil
			}
			return fmt.Errorf("failed to list current Event Gateway Control Planes: %w", err)
		}
	}

	// Index current Event Gateway Control Planes by name
	currentByName := make(map[string]state.EventGatewayControlPlane)
	for _, cp := range currentEGWControlPlanes {
		currentByName[cp.Name] = cp
	}

	// Collect protection validation errors
	protectionErrors := &ProtectionErrorCollector{}

	// Handle delete mode - plan DELETE for desired resources that exist in Konnect
	if plan.Metadata.Mode == PlanModeDelete {
		for _, desiredEGWCP := range desired {
			if desiredEGWCP.IsExternal() {
				plan.AddWarning("", fmt.Sprintf(
					"event_gateway_control_plane %q is external, skipping delete",
					desiredEGWCP.GetRef(),
				))
				continue
			}

			current, exists := currentByName[desiredEGWCP.Name]
			if !exists {
				plan.AddWarning("", fmt.Sprintf(
					"event_gateway_control_plane %q not found in Konnect, skipping delete",
					desiredEGWCP.Name,
				))
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			if err := p.validateProtection(
				ResourceTypeEventGatewayControlPlane, desiredEGWCP.Name, isProtected, ActionDelete,
			); err != nil {
				protectionErrors.Add(err)
			} else {
				p.planEGWControlPlaneDelete(current, plan)
			}
		}

		if protectionErrors.HasErrors() {
			return protectionErrors.Error()
		}
		return nil
	}

	// Compare each desired Event Gateway Control Plane
	for _, desiredEGWCP := range desired {
		// Track the gateway change ID for dependency resolution
		var gatewayChangeID string
		var gatewayID string

		if desiredEGWCP.IsExternal() {
			gatewayID = desiredEGWCP.GetKonnectID()
			if gatewayID == "" {
				plan.AddWarning("", fmt.Sprintf(
					"external event_gateway %q has no resolved ID; child diffs may be incomplete",
					desiredEGWCP.GetRef(),
				))
			}
		} else {
			current, exists := currentByName[desiredEGWCP.Name]

			if !exists {
				// CREATE action
				gatewayChangeID = p.planEGWControlPlaneCreate(desiredEGWCP, plan)
			} else {
				gatewayID = current.ID

				// Check if update needed
				isProtected := labels.IsProtectedResource(current.NormalizedLabels)

				// Get protection status from desired configuration
				shouldProtect := false
				if desiredEGWCP.Kongctl != nil &&
					desiredEGWCP.Kongctl.Protected != nil &&
					*desiredEGWCP.Kongctl.Protected {
					shouldProtect = true
				}

				// Handle protection changes
				if isProtected != shouldProtect {
					// When changing protection status, include any other field updates too
					needsUpdate, updateFields, changedFields := p.shouldUpdateEGWControlPlaneResource(current, desiredEGWCP)

					// Create protection change object
					protectionChange := &ProtectionChange{
						Old: isProtected,
						New: shouldProtect,
					}

					// Validate protection change
					err := p.validateProtectionWithChange(
						ResourceTypeEventGatewayControlPlane,
						desiredEGWCP.Name,
						isProtected,
						ActionUpdate,
						protectionChange,
						needsUpdate,
					)
					if err != nil {
						protectionErrors.Add(err)
					} else {
						p.planEGWControlPlaneProtectionChangeWithFields(
							current,
							desiredEGWCP,
							isProtected,
							shouldProtect,
							updateFields,
							changedFields,
							plan,
						)
					}
				} else {
					// Check if update needed based on configuration
					needsUpdate, updateFields, changedFields := p.shouldUpdateEGWControlPlaneResource(current, desiredEGWCP)
					if needsUpdate {
						// Regular update - check protection
						if err := p.validateProtection(
							ResourceTypeEventGatewayControlPlane, desiredEGWCP.Name, isProtected, ActionUpdate,
						); err != nil {
							protectionErrors.Add(err)
						} else {
							p.planEGWControlPlaneUpdateWithFields(current, desiredEGWCP, updateFields, changedFields, plan)
						}
					}
				}
			}
		}

		// Plan backend clusters for this gateway (whether it exists or is being created)
		backendClusters := p.resources.GetBackendClustersForGateway(desiredEGWCP.Ref)

		if p.shouldPlanChild(
			plan,
			resources.ResourceTypeEventGatewayControlPlane,
			desiredEGWCP.Ref,
			resources.ResourceTypeEventGatewayBackendCluster,
		) && (len(backendClusters) > 0 || plan.Metadata.Mode == PlanModeSync) {
			if err := p.planEventGatewayBackendClusterChanges(
				ctx, plannerCtx, namespace, desiredEGWCP.Name, gatewayID, desiredEGWCP.Ref,
				gatewayChangeID, backendClusters, plan,
			); err != nil {
				return err
			}
		}

		// Plan virtual clusters for this gateway (whether it exists or is being created)
		virtualClusters := p.resources.GetVirtualClustersForGateway(desiredEGWCP.Ref)

		if p.shouldPlanChild(
			plan,
			resources.ResourceTypeEventGatewayControlPlane,
			desiredEGWCP.Ref,
			resources.ResourceTypeEventGatewayVirtualCluster,
		) && (len(virtualClusters) > 0 || plan.Metadata.Mode == PlanModeSync) {
			if err := p.planEventGatewayVirtualClusterChanges(
				ctx, plannerCtx, namespace, desiredEGWCP.Name, gatewayID, desiredEGWCP.Ref,
				gatewayChangeID, virtualClusters, plan,
			); err != nil {
				return err
			}
		}

		// Plan listeners for this gateway (whether it exists or is being created)
		listeners := p.resources.GetListenersForEventGateway(desiredEGWCP.Ref)

		if p.shouldPlanChild(
			plan,
			resources.ResourceTypeEventGatewayControlPlane,
			desiredEGWCP.Ref,
			resources.ResourceTypeEventGatewayListener,
		) && (len(listeners) > 0 || plan.Metadata.Mode == PlanModeSync) {
			if err := p.planEventGatewayListenerChanges(
				ctx, plannerCtx, namespace, desiredEGWCP.Name, gatewayID, desiredEGWCP.Ref,
				gatewayChangeID, listeners, plan,
			); err != nil {
				return err
			}
		}

		// Plan data plane certificates for this gateway (whether it exists or is being created)
		dataPlaneCerts := p.resources.GetDataPlaneCertificatesForGateway(desiredEGWCP.Ref)

		if p.shouldPlanChild(
			plan,
			resources.ResourceTypeEventGatewayControlPlane,
			desiredEGWCP.Ref,
			resources.ResourceTypeEventGatewayDataPlaneCertificate,
		) && (len(dataPlaneCerts) > 0 || plan.Metadata.Mode == PlanModeSync) {
			if err := p.planEventGatewayDataPlaneCertificateChanges(
				ctx, plannerCtx, namespace, desiredEGWCP.Name, gatewayID, desiredEGWCP.Ref,
				gatewayChangeID, dataPlaneCerts, plan,
			); err != nil {
				return err
			}
		}

		// Plan schema registries for this gateway (whether it exists or is being created)
		schemaRegistries := p.resources.GetSchemaRegistriesForGateway(desiredEGWCP.Ref)

		if p.shouldPlanChild(
			plan,
			resources.ResourceTypeEventGatewayControlPlane,
			desiredEGWCP.Ref,
			resources.ResourceTypeEventGatewaySchemaRegistry,
		) && (len(schemaRegistries) > 0 || plan.Metadata.Mode == PlanModeSync) {
			if err := p.planEventGatewaySchemaRegistryChanges(
				ctx, plannerCtx, namespace, desiredEGWCP.Name, gatewayID, desiredEGWCP.Ref,
				gatewayChangeID, schemaRegistries, plan,
			); err != nil {
				return err
			}
		}

		// Plan static keys for this gateway (whether it exists or is being created)
		staticKeys := p.resources.GetStaticKeysForGateway(desiredEGWCP.Ref)

		if p.shouldPlanChild(
			plan,
			resources.ResourceTypeEventGatewayControlPlane,
			desiredEGWCP.Ref,
			resources.ResourceTypeEventGatewayStaticKey,
		) && (len(staticKeys) > 0 || plan.Metadata.Mode == PlanModeSync) {
			if err := p.planEventGatewayStaticKeyChanges(
				ctx, plannerCtx, namespace, desiredEGWCP.Name, gatewayID, desiredEGWCP.Ref,
				gatewayChangeID, staticKeys, plan,
			); err != nil {
				return err
			}
		}

		// Plan TLS trust bundles for this gateway (whether it exists or is being created)
		trustBundles := p.resources.GetTrustBundlesForGateway(desiredEGWCP.Ref)

		if p.shouldPlanChild(
			plan,
			resources.ResourceTypeEventGatewayControlPlane,
			desiredEGWCP.Ref,
			resources.ResourceTypeEventGatewayTLSTrustBundle,
		) && (len(trustBundles) > 0 || plan.Metadata.Mode == PlanModeSync) {
			if err := p.planEventGatewayTLSTrustBundleChanges(
				ctx, plannerCtx, namespace, desiredEGWCP.Name, gatewayID, desiredEGWCP.Ref,
				gatewayChangeID, trustBundles, plan,
			); err != nil {
				return err
			}
		}
	}

	// Check for managed resources to delete (sync mode only)
	if plan.Metadata.Mode == PlanModeSync {
		// Build set of desired Event gateway names
		desiredNames := make(map[string]bool)
		for _, eventGateway := range desired {
			desiredNames[eventGateway.Name] = true
		}

		// Find managed Event Gateway Control Planes not in desired state
		for name, current := range currentByName {
			if !desiredNames[name] {
				// Validate protection before adding DELETE
				isProtected := labels.IsProtectedResource(current.NormalizedLabels)
				if err := p.validateProtection(ResourceTypeEventGatewayControlPlane, name, isProtected, ActionDelete); err != nil {
					protectionErrors.Add(err)
				} else {
					p.planEGWControlPlaneDelete(current, plan)
				}
			}
		}
	}

	// Fail fast if any protected resources would be modified
	if protectionErrors.HasErrors() {
		return protectionErrors.Error()
	}

	return nil
}

func (p *Planner) planEGWControlPlaneProtectionChangeWithFields(
	current state.EventGatewayControlPlane,
	desired resources.EventGatewayControlPlaneResource,
	wasProtected, shouldProtect bool,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	// Extract namespace
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}

	// Use generic protection change planner
	config := ProtectionChangeConfig{
		ResourceType: ResourceTypeEventGatewayControlPlane,
		ResourceName: desired.Name,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		OldProtected: wasProtected,
		NewProtected: shouldProtect,
		Namespace:    namespace,
	}

	change := p.genericPlanner.PlanProtectionChange(context.Background(), config)

	// Always include essential fields for protection changes
	fields := make(map[string]any)

	// Include any field updates if present
	maps.Copy(fields, updateFields)

	// ALWAYS include essential identification fields for protection changes
	fields[FieldName] = current.Name
	fields[FieldID] = current.ID

	// Preserve namespace context for execution phase
	if current.Labels != nil {
		if namespace, exists := current.Labels[labels.NamespaceKey]; exists {
			fields[FieldNamespace] = namespace
		}

		// Preserve other critical labels that identify managed resources
		preservedLabels := make(map[string]string)
		for key, value := range current.Labels {
			// Preserve all KONGCTL- prefixed labels except protected (which will be updated)
			if strings.HasPrefix(key, "KONGCTL-") && key != labels.ProtectedKey {
				preservedLabels[key] = value
			}
		}
		if len(preservedLabels) > 0 {
			fields[FieldPreservedLabels] = preservedLabels
		}
	}

	change.Fields = fields
	if len(changedFields) > 0 {
		change.ChangedFields = changedFields
	}

	plan.AddChange(change)
}

func (p *Planner) shouldUpdateEGWControlPlaneResource(
	current state.EventGatewayControlPlane,
	desired resources.EventGatewayControlPlaneResource,
) (bool, map[string]any, map[string]FieldChange) {
	updates := make(map[string]any)
	changedFields := make(map[string]FieldChange)

	if desired.Name != current.Name {
		updates[FieldName] = desired.Name
		changedFields[FieldName] = FieldChange{
			Old: current.Name,
			New: desired.Name,
		}
	}

	if getString(current.Description) != getString(desired.Description) {
		updates[FieldDescription] = getString(desired.Description)
		changedFields[FieldDescription] = FieldChange{
			Old: getString(current.Description),
			New: getString(desired.Description),
		}
	}

	if desired.MinRuntimeVersion != nil && current.MinRuntimeVersion != getString(desired.MinRuntimeVersion) {
		updates[FieldMinRuntimeVersion] = getString(desired.MinRuntimeVersion)
		changedFields[FieldMinRuntimeVersion] = FieldChange{
			Old: current.MinRuntimeVersion,
			New: getString(desired.MinRuntimeVersion),
		}
	}

	if desired.Labels != nil {
		if labels.CompareUserLabels(current.NormalizedLabels, desired.GetLabels()) {
			updates[FieldLabels] = desired.GetLabels()
			changedFields[FieldLabels] = FieldChange{
				Old: labels.GetUserLabels(current.NormalizedLabels),
				New: labels.GetUserLabels(desired.GetLabels()),
			}
		}
	}

	// Add other field comparisons

	return len(updates) > 0, updates, changedFields
}

func (p *Planner) planEGWControlPlaneCreate(
	egwControlPlane resources.EventGatewayControlPlaneResource,
	plan *Plan,
) string {
	var protection any
	if egwControlPlane.Kongctl != nil && egwControlPlane.Kongctl.Protected != nil {
		protection = *egwControlPlane.Kongctl.Protected
	}

	// Extract namespace
	namespace := DefaultNamespace
	if egwControlPlane.Kongctl != nil && egwControlPlane.Kongctl.Namespace != nil {
		namespace = *egwControlPlane.Kongctl.Namespace
	}

	config := CreateConfig{
		ResourceType:   ResourceTypeEventGatewayControlPlane,
		ResourceName:   egwControlPlane.Name,
		ResourceRef:    egwControlPlane.Ref,
		RequiredFields: []string{FieldName},
		FieldExtractor: func(_ any) map[string]any {
			return extractEGWControlPlaneFields(egwControlPlane)
		},
		Namespace: namespace,
		DependsOn: []string{},
	}

	change, err := p.genericPlanner.PlanCreate(context.Background(), config)
	if err != nil {
		return ""
	}
	change.Protection = protection
	plan.AddChange(change)
	return change.ID
}

func extractEGWControlPlaneFields(resource any) map[string]any {
	fields := make(map[string]any)
	egwControlPlane, ok := resource.(resources.EventGatewayControlPlaneResource)
	if !ok {
		return fields
	}

	fields[FieldName] = egwControlPlane.Name

	if egwControlPlane.Description != nil {
		fields[FieldDescription] = *egwControlPlane.Description
	}

	if egwControlPlane.MinRuntimeVersion != nil {
		fields[FieldMinRuntimeVersion] = *egwControlPlane.MinRuntimeVersion
	}

	if len(egwControlPlane.GetLabels()) > 0 {
		fields[FieldLabels] = egwControlPlane.GetLabels()
	}
	return fields
}

func (p *Planner) planEGWControlPlaneUpdateWithFields(
	current state.EventGatewayControlPlane,
	desired resources.EventGatewayControlPlaneResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	plan *Plan,
) {
	var protection any
	if desired.Kongctl != nil && desired.Kongctl.Protected != nil {
		protection = *desired.Kongctl.Protected
	}

	// Extract namespace
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}

	// Always include name for identification
	updateFields[FieldName] = current.Name

	updateFields[FieldCurrentLabels] = current.NormalizedLabels
	config := UpdateConfig{
		ResourceType:   ResourceTypeEventGatewayControlPlane,
		ResourceName:   desired.Name,
		ResourceRef:    desired.Ref,
		ResourceID:     current.ID,
		CurrentFields:  nil, // Not needed for direct update
		DesiredFields:  updateFields,
		ChangedFields:  changedFields,
		RequiredFields: []string{FieldName},
		Namespace:      namespace,
	}

	change, err := p.genericPlanner.PlanUpdate(context.Background(), config)
	if err != nil {
		// Handle error appropriately - this is example code
		// In real implementation, return the error
		return
	}
	change.Protection = protection

	plan.AddChange(change)
}

func (p *Planner) planEGWControlPlaneDelete(egwControlPlane state.EventGatewayControlPlane, plan *Plan) {
	namespace := DefaultNamespace
	if ns, ok := egwControlPlane.NormalizedLabels[labels.NamespaceKey]; ok {
		namespace = ns
	}

	config := DeleteConfig{
		ResourceType: ResourceTypeEventGatewayControlPlane,
		ResourceName: egwControlPlane.Name,
		ResourceRef:  egwControlPlane.Name,
		ResourceID:   egwControlPlane.ID,
		Namespace:    namespace,
	}

	change := p.genericPlanner.PlanDelete(context.Background(), config)
	plan.AddChange(change)
}
