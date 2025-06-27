package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/hash"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

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
func (p *Planner) GeneratePlan(ctx context.Context, rs *resources.ResourceSet) (*Plan, error) {
	plan := NewPlan("1.0", "kongctl/dev")

	// Generate changes for each resource type
	if err := p.planAuthStrategyChanges(ctx, rs.ApplicationAuthStrategies, plan); err != nil {
		return nil, fmt.Errorf("failed to plan auth strategy changes: %w", err)
	}

	if err := p.planPortalChanges(ctx, rs.Portals, plan); err != nil {
		return nil, fmt.Errorf("failed to plan portal changes: %w", err)
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
					plan.Changes[i].References[field] = ReferenceInfo{
						Ref: ref.Ref,
						ID:  ref.ID,
					}
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

	// Compare each desired portal
	for _, desiredPortal := range desired {
		// Calculate config hash for desired state
		configHash, err := hash.CalculatePortalHash(desiredPortal.CreatePortal)
		if err != nil {
			return fmt.Errorf("failed to calculate hash for portal %q: %w", desiredPortal.GetRef(), err)
		}

		current, exists := currentByName[desiredPortal.Name]

		if !exists {
			// CREATE action
			p.planPortalCreate(desiredPortal, configHash, plan)
		} else {
			// Check if update needed
			currentHash := current.NormalizedLabels[labels.ConfigHashKey]
			isProtected := current.NormalizedLabels[labels.ProtectedKey] == "true"

			// Get protection status from desired labels
			shouldProtect := false
			if desiredPortal.Labels != nil {
				if protVal, ok := desiredPortal.Labels[labels.ProtectedKey]; ok && protVal != nil && *protVal == "true" {
					shouldProtect = true
				}
			}

			// Handle protection changes separately
			if isProtected != shouldProtect {
				p.planProtectionChange(current, isProtected, shouldProtect, plan)
				// If unprotecting, we can then update
				if isProtected && !shouldProtect {
					if currentHash != configHash {
						p.planPortalUpdate(current, desiredPortal, configHash, plan)
					}
				}
			} else if currentHash != configHash {
				// Regular update (no protection change)
				p.planPortalUpdate(current, desiredPortal, configHash, plan)
			}
		}
	}

	return nil
}

// planPortalCreate creates a CREATE change for a portal
func (p *Planner) planPortalCreate(portal resources.PortalResource, configHash string, plan *Plan) {
	fields := make(map[string]interface{})
	fields["name"] = portal.Name
	if portal.DisplayName != nil {
		fields["display_name"] = *portal.DisplayName
	}
	if portal.Description != nil {
		fields["description"] = *portal.Description
	}
	if portal.DefaultApplicationAuthStrategyID != nil {
		fields["default_application_auth_strategy_id"] = *portal.DefaultApplicationAuthStrategyID
	}
	// Add other fields...

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, portal.GetRef()),
		ResourceType: "portal",
		ResourceRef:  portal.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		ConfigHash:   configHash,
		DependsOn:    []string{},
	}

	// Check if protected
	if portal.Labels != nil {
		if protVal, ok := portal.Labels[labels.ProtectedKey]; ok && protVal != nil && *protVal == "true" {
			change.Protection = true
		}
	}

	plan.AddChange(change)
}

// planPortalUpdate creates an UPDATE change for a portal
func (p *Planner) planPortalUpdate(current state.Portal, desired resources.PortalResource, configHash string, plan *Plan) {
	fields := make(map[string]interface{})
	dependencies := []string{}

	// Compare each field and store only changes
	currentDesc := getString(current.Description)
	desiredDesc := getString(desired.Description)
	if currentDesc != desiredDesc {
		fields["description"] = FieldChange{
			Old: currentDesc,
			New: desiredDesc,
		}
	}

	if current.DisplayName != getString(desired.DisplayName) {
		fields["display_name"] = FieldChange{
			Old: current.DisplayName,
			New: getString(desired.DisplayName),
		}
	}

	// Handle auth strategy reference
	desiredAuthID := getString(desired.DefaultApplicationAuthStrategyID)
	currentAuthID := getString(current.DefaultApplicationAuthStrategyID)
	if currentAuthID != desiredAuthID {
		fields["default_application_auth_strategy_id"] = FieldChange{
			Old: currentAuthID,
			New: desiredAuthID,
		}
	}

	// Add other field comparisons...

	// Only create change if there are actual field changes
	if len(fields) > 0 {
		change := PlannedChange{
			ID:           p.nextChangeID(ActionUpdate, desired.GetRef()),
			ResourceType: "portal",
			ResourceRef:  desired.GetRef(),
			ResourceID:   current.ID,
			Action:       ActionUpdate,
			Fields:       fields,
			ConfigHash:   configHash,
			DependsOn:    dependencies,
		}

		// Check if already protected
		if current.NormalizedLabels[labels.ProtectedKey] == "true" {
			change.Protection = true
		}

		plan.AddChange(change)
	}
}

// planProtectionChange creates a separate UPDATE for protection status
func (p *Planner) planProtectionChange(portal state.Portal, wasProtected, shouldProtect bool, plan *Plan) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, portal.Name+"-protection"),
		ResourceType: "portal",
		ResourceRef:  portal.Name,
		ResourceID:   portal.ID,
		Action:       ActionUpdate,
		Fields:       map[string]interface{}{}, // No field changes allowed
		Protection: ProtectionChange{
			Old: wasProtected,
			New: shouldProtect,
		},
		ConfigHash: portal.NormalizedLabels[labels.ConfigHashKey],
		DependsOn:  []string{},
	}

	plan.AddChange(change)
}

// planAuthStrategyChanges generates changes for auth strategies
func (p *Planner) planAuthStrategyChanges(_ context.Context, desired []resources.ApplicationAuthStrategyResource, plan *Plan) error {
	// Similar logic to portals but for auth strategies
	// TODO: Implement when auth strategy state client is available

	// For now, just create all as new
	for _, strategy := range desired {
		configHash, err := hash.CalculateAuthStrategyHash(strategy.CreateAppAuthStrategyRequest)
		if err != nil {
			return fmt.Errorf("failed to calculate hash for auth strategy %q: %w", strategy.GetRef(), err)
		}

		// Extract fields based on strategy type
		fields := make(map[string]interface{})
		var strategyType string
		var configs map[string]interface{}

		switch strategy.CreateAppAuthStrategyRequest.Type {
		case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
			if strategy.CreateAppAuthStrategyRequest.AppAuthStrategyKeyAuthRequest != nil {
				fields["name"] = strategy.CreateAppAuthStrategyRequest.AppAuthStrategyKeyAuthRequest.Name
				fields["display_name"] = strategy.CreateAppAuthStrategyRequest.AppAuthStrategyKeyAuthRequest.DisplayName
				strategyType = "key_auth"
				configs = map[string]interface{}{
					"key_auth": strategy.CreateAppAuthStrategyRequest.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth,
				}
			}
		case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
			if strategy.CreateAppAuthStrategyRequest.AppAuthStrategyOpenIDConnectRequest != nil {
				fields["name"] = strategy.CreateAppAuthStrategyRequest.AppAuthStrategyOpenIDConnectRequest.Name
				fields["display_name"] = strategy.CreateAppAuthStrategyRequest.AppAuthStrategyOpenIDConnectRequest.DisplayName
				strategyType = "openid_connect"
				configs = map[string]interface{}{
					"openid_connect": strategy.CreateAppAuthStrategyRequest.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect,
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
			ConfigHash:   configHash,
			DependsOn:    []string{},
		}

		plan.AddChange(change)
	}

	return nil
}

// getString dereferences string pointer or returns empty
func getString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}