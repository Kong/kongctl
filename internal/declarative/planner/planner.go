package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// Options configures plan generation behavior
type Options struct {
	Mode PlanMode
}

// Planner generates execution plans
type Planner struct {
	client      *state.Client
	resolver    *ReferenceResolver
	depResolver *DependencyResolver
	changeCount int
}

// NewPlanner creates a new planner
func NewPlanner(client *state.Client) *Planner {
	return &Planner{
		client:      client,
		resolver:    NewReferenceResolver(client),
		depResolver: NewDependencyResolver(),
		changeCount: 0,
	}
}

// GeneratePlan creates a plan from declarative configuration
func (p *Planner) GeneratePlan(ctx context.Context, rs *resources.ResourceSet, opts Options) (*Plan, error) {
	plan := NewPlan("1.0", "kongctl/dev", opts.Mode)

	// Generate changes for each resource type
	p.planAuthStrategyChanges(ctx, rs.ApplicationAuthStrategies, plan)

	if err := p.planPortalChanges(ctx, rs.Portals, plan); err != nil {
		return nil, fmt.Errorf("failed to plan portal changes: %w", err)
	}

	// Plan API changes
	if err := p.planAPIChanges(ctx, rs.APIs, plan); err != nil {
		return nil, fmt.Errorf("failed to plan API changes: %w", err)
	}

	// Plan API child resources (extracted from nested definitions)
	if err := p.planAPIVersionsChanges(ctx, rs.APIVersions, plan); err != nil {
		return nil, fmt.Errorf("failed to plan API version changes: %w", err)
	}
	
	if err := p.planAPIPublicationsChanges(ctx, rs.APIPublications, plan); err != nil {
		return nil, fmt.Errorf("failed to plan API publication changes: %w", err)
	}
	
	if err := p.planAPIImplementationsChanges(ctx, rs.APIImplementations, plan); err != nil {
		return nil, fmt.Errorf("failed to plan API implementation changes: %w", err)
	}
	
	if err := p.planAPIDocumentsChanges(ctx, rs.APIDocuments, plan); err != nil {
		return nil, fmt.Errorf("failed to plan API document changes: %w", err)
	}

	// Future: Add other resource types

	// Resolve references for all changes
	resolveResult, err := p.resolver.ResolveReferences(ctx, plan.Changes)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve references: %w", err)
	}

	// Apply resolved references to changes
	for changeID, refs := range resolveResult.ChangeReferences {
		for i := range plan.Changes {
			if plan.Changes[i].ID == changeID {
				plan.Changes[i].References = make(map[string]ReferenceInfo)
				for field, ref := range refs {
					plan.Changes[i].References[field] = ReferenceInfo(ref)
				}
				break
			}
		}
	}

	// Resolve dependencies and calculate execution order
	executionOrder, err := p.depResolver.ResolveDependencies(plan.Changes)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}
	plan.SetExecutionOrder(executionOrder)

	// Add warnings for unresolved references
	for _, change := range plan.Changes {
		for field, ref := range change.References {
			if ref.ID == "<unknown>" {
				plan.AddWarning(change.ID, fmt.Sprintf(
					"Reference %s=%s will be resolved during execution",
					field, ref.Ref))
			}
		}
	}

	return plan, nil
}

// nextChangeID generates semantic change IDs
func (p *Planner) nextChangeID(action ActionType, ref string) string {
	p.changeCount++
	actionChar := "?"
	switch action {
	case ActionCreate:
		actionChar = "c"
	case ActionUpdate:
		actionChar = "u"
	case ActionDelete:
		actionChar = "d"
	}
	return fmt.Sprintf("%d-%s-%s", p.changeCount, actionChar, ref)
}

// validateProtection checks if a protected resource would be modified or deleted
func (p *Planner) validateProtection(
	resourceType, resourceName string, 
	currentProtected bool, 
	action ActionType,
) error {
	if action == ActionUpdate || action == ActionDelete {
		if currentProtected {
			var actionVerb string
			switch action { //nolint:exhaustive // ActionCreate is not possible here due to outer if condition
			case ActionDelete:
				actionVerb = "deleted"
			case ActionUpdate:
				actionVerb = "updated"
			default:
				actionVerb = "modified"
			}
			return fmt.Errorf("%s %q is protected and cannot be %s", 
				resourceType, resourceName, actionVerb)
		}
	}
	return nil
}

// planPortalChanges generates changes for portal resources
func (p *Planner) planPortalChanges(ctx context.Context, desired []resources.PortalResource, plan *Plan) error {
	// Fetch current managed portals
	currentPortals, err := p.client.ListManagedPortals(ctx)
	if err != nil {
		return fmt.Errorf("failed to list current portals: %w", err)
	}

	// Index current portals by name
	currentByName := make(map[string]state.Portal)
	for _, portal := range currentPortals {
		currentByName[portal.Name] = portal
	}

	// Collect protection validation errors
	var protectionErrors []error

	// Compare each desired portal
	for _, desiredPortal := range desired {
		current, exists := currentByName[desiredPortal.Name]

		if !exists {
			// CREATE action
			p.planPortalCreate(desiredPortal, plan)
		} else {
			// Check if update needed
			isProtected := current.NormalizedLabels[labels.ProtectedKey] == "true"

			// Get protection status from desired configuration
			shouldProtect := false
			if desiredPortal.Kongctl != nil && desiredPortal.Kongctl.Protected {
				shouldProtect = true
			}

			// Handle protection changes
			if isProtected != shouldProtect {
				// When changing protection status, include any other field updates too
				_, updateFields := p.shouldUpdatePortal(current, desiredPortal)
				p.planProtectionChangeWithFields(current, desiredPortal, isProtected, shouldProtect, updateFields, plan)
			} else {
				// Check if update needed based on configuration
				needsUpdate, updateFields := p.shouldUpdatePortal(current, desiredPortal)
				if needsUpdate {
					// Regular update - check protection
					if err := p.validateProtection("portal", desiredPortal.Name, isProtected, ActionUpdate); err != nil {
						protectionErrors = append(protectionErrors, err)
					} else {
						p.planPortalUpdateWithFields(current, desiredPortal, updateFields, plan)
					}
				}
			}
		}
	}

	// Check for managed resources to delete (sync mode only)
	if plan.Metadata.Mode == PlanModeSync {
		// Build set of desired portal names
		desiredNames := make(map[string]bool)
		for _, portal := range desired {
			desiredNames[portal.Name] = true
		}

		// Find managed portals not in desired state
		for name, current := range currentByName {
			if !desiredNames[name] {
				// Validate protection before adding DELETE
				isProtected := current.NormalizedLabels[labels.ProtectedKey] == "true"
				if err := p.validateProtection("portal", name, isProtected, ActionDelete); err != nil {
					protectionErrors = append(protectionErrors, err)
				} else {
					p.planPortalDelete(current, plan)
				}
			}
		}
	}

	// Fail fast if any protected resources would be modified
	if len(protectionErrors) > 0 {
		errMsg := "Cannot generate plan due to protected resources:\n"
		for _, err := range protectionErrors {
			errMsg += fmt.Sprintf("- %s\n", err.Error())
		}
		errMsg += "\nTo proceed, first update these resources to set protected: false"
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// planPortalCreate creates a CREATE change for a portal
func (p *Planner) planPortalCreate(portal resources.PortalResource, plan *Plan) {
	fields := make(map[string]interface{})
	fields["name"] = portal.Name
	if portal.DisplayName != nil {
		fields["display_name"] = *portal.DisplayName
	}
	if portal.Description != nil {
		fields["description"] = *portal.Description
	}
	if portal.AuthenticationEnabled != nil {
		fields["authentication_enabled"] = *portal.AuthenticationEnabled
	}
	if portal.RbacEnabled != nil {
		fields["rbac_enabled"] = *portal.RbacEnabled
	}
	if portal.DefaultAPIVisibility != nil {
		fields["default_api_visibility"] = string(*portal.DefaultAPIVisibility)
	}
	if portal.DefaultPageVisibility != nil {
		fields["default_page_visibility"] = string(*portal.DefaultPageVisibility)
	}
	if portal.DefaultApplicationAuthStrategyID != nil {
		fields["default_application_auth_strategy_id"] = *portal.DefaultApplicationAuthStrategyID
	}
	if portal.AutoApproveDevelopers != nil {
		fields["auto_approve_developers"] = *portal.AutoApproveDevelopers
	}
	if portal.AutoApproveApplications != nil {
		fields["auto_approve_applications"] = *portal.AutoApproveApplications
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, portal.GetRef()),
		ResourceType: "portal",
		ResourceRef:  portal.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	// Always set protection status explicitly
	if portal.Kongctl != nil && portal.Kongctl.Protected {
		change.Protection = true
	} else {
		change.Protection = false
	}
	
	// Copy user-defined labels only (protection label will be added during execution)
	if len(portal.Labels) > 0 {
		if fields["labels"] == nil {
			fields["labels"] = make(map[string]interface{})
		}
		labelsMap := fields["labels"].(map[string]interface{})
		for k, v := range portal.Labels {
			if v != nil {
				labelsMap[k] = *v
			}
		}
	}

	plan.AddChange(change)
}

// shouldUpdatePortal checks if portal needs update based on configured fields only
func (p *Planner) shouldUpdatePortal(
	current state.Portal, 
	desired resources.PortalResource,
) (bool, map[string]interface{}) {
	updates := make(map[string]interface{})
	
	// Only compare fields present in desired configuration
	if desired.Description != nil {
		currentDesc := getString(current.Description)
		if currentDesc != *desired.Description {
			updates["description"] = *desired.Description
		}
	}
	
	if desired.DisplayName != nil {
		if current.DisplayName != *desired.DisplayName {
			updates["display_name"] = *desired.DisplayName
		}
	}
	
	if desired.DefaultApplicationAuthStrategyID != nil {
		currentAuthID := getString(current.DefaultApplicationAuthStrategyID)
		if currentAuthID != *desired.DefaultApplicationAuthStrategyID {
			updates["default_application_auth_strategy_id"] = *desired.DefaultApplicationAuthStrategyID
		}
	}
	
	if desired.AuthenticationEnabled != nil {
		if current.AuthenticationEnabled != *desired.AuthenticationEnabled {
			updates["authentication_enabled"] = *desired.AuthenticationEnabled
		}
	}
	
	if desired.RbacEnabled != nil {
		if current.RbacEnabled != *desired.RbacEnabled {
			updates["rbac_enabled"] = *desired.RbacEnabled
		}
	}
	
	if desired.AutoApproveDevelopers != nil {
		if current.AutoApproveDevelopers != *desired.AutoApproveDevelopers {
			updates["auto_approve_developers"] = *desired.AutoApproveDevelopers
		}
	}
	
	if desired.AutoApproveApplications != nil {
		if current.AutoApproveApplications != *desired.AutoApproveApplications {
			updates["auto_approve_applications"] = *desired.AutoApproveApplications
		}
	}
	
	if desired.DefaultAPIVisibility != nil {
		currentVisibility := string(current.DefaultAPIVisibility)
		desiredVisibility := string(*desired.DefaultAPIVisibility)
		if currentVisibility != desiredVisibility {
			updates["default_api_visibility"] = desiredVisibility
		}
	}
	
	if desired.DefaultPageVisibility != nil {
		currentVisibility := string(current.DefaultPageVisibility)
		desiredVisibility := string(*desired.DefaultPageVisibility)
		if currentVisibility != desiredVisibility {
			updates["default_page_visibility"] = desiredVisibility
		}
	}
	
	return len(updates) > 0, updates
}

// planPortalUpdateWithFields creates an UPDATE change with specific fields
func (p *Planner) planPortalUpdateWithFields(
	current state.Portal,
	desired resources.PortalResource,
	updateFields map[string]interface{},
	plan *Plan,
) {
	fields := make(map[string]interface{})
	
	// Store the fields that need updating
	// Note: We store the new values directly, not FieldChange structs
	// This simplifies the executor's job
	for field, newValue := range updateFields {
		fields[field] = newValue
	}
	
	// Always include name for identification
	fields["name"] = current.Name
	
	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, desired.GetRef()),
		ResourceType: "portal",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    []string{},
	}
	
	// Check if already protected
	if current.NormalizedLabels[labels.ProtectedKey] == "true" {
		change.Protection = true
	}
	
	plan.AddChange(change)
}

// planProtectionChangeWithFields creates an UPDATE for protection status with optional field updates
func (p *Planner) planProtectionChangeWithFields(
	current state.Portal, 
	desired resources.PortalResource, 
	wasProtected, shouldProtect bool, 
	updateFields map[string]interface{},
	plan *Plan,
) {
	fields := make(map[string]interface{})
	
	// Include any field updates if unprotecting
	if wasProtected && !shouldProtect && len(updateFields) > 0 {
		for field, newValue := range updateFields {
			fields[field] = newValue
		}
	}
	
	// Always include name for identification
	fields["name"] = current.Name
	
	// Don't add protection label here - it will be added during execution
	// based on the Protection field
	
	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, desired.GetRef()),
		ResourceType: "portal",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		Protection: ProtectionChange{
			Old: wasProtected,
			New: shouldProtect,
		},
		DependsOn:  []string{},
	}

	plan.AddChange(change)
}

// planAuthStrategyChanges generates changes for auth strategies
func (p *Planner) planAuthStrategyChanges(
	_ context.Context, 
	desired []resources.ApplicationAuthStrategyResource, 
	plan *Plan,
) {
	// Similar logic to portals but for auth strategies
	// TODO: Implement when auth strategy state client is available

	// For now, just create all as new
	for _, strategy := range desired {
		// Extract fields based on strategy type
		fields := make(map[string]interface{})
		var strategyType string
		var configs map[string]interface{}

		switch strategy.Type {
		case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
			if strategy.AppAuthStrategyKeyAuthRequest != nil {
				fields["name"] = strategy.AppAuthStrategyKeyAuthRequest.Name
				fields["display_name"] = strategy.AppAuthStrategyKeyAuthRequest.DisplayName
				strategyType = "key_auth"
				configs = map[string]interface{}{
					"key_auth": strategy.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth,
				}
			}
		case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
			if strategy.AppAuthStrategyOpenIDConnectRequest != nil {
				fields["name"] = strategy.AppAuthStrategyOpenIDConnectRequest.Name
				fields["display_name"] = strategy.AppAuthStrategyOpenIDConnectRequest.DisplayName
				strategyType = "openid_connect"
				configs = map[string]interface{}{
					"openid_connect": strategy.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect,
				}
			}
		}

		fields["strategy_type"] = strategyType
		fields["configs"] = configs

		change := PlannedChange{
			ID:           p.nextChangeID(ActionCreate, strategy.GetRef()),
			ResourceType: "application_auth_strategy",
			ResourceRef:  strategy.GetRef(),
			Action:       ActionCreate,
			Fields:       fields,
			DependsOn:    []string{},
		}

		plan.AddChange(change)
	}
}

// planPortalDelete creates a DELETE change for a portal
func (p *Planner) planPortalDelete(portal state.Portal, plan *Plan) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, portal.Name),
		ResourceType: "portal",
		ResourceRef:  portal.Name,
		ResourceID:   portal.ID,
		Action:       ActionDelete,
		Fields:       map[string]interface{}{}, // No fields for DELETE
		DependsOn:    []string{},
	}

	plan.AddChange(change)
}

// getString dereferences string pointer or returns empty
func getString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}