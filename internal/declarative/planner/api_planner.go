package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// planAPIChanges generates changes for API resources and their child resources
func (p *Planner) planAPIChanges(ctx context.Context, desired []resources.APIResource, plan *Plan) error {
	// Skip if no API resources to plan
	if len(desired) == 0 {
		return nil
	}
	
	// Fetch current managed APIs
	currentAPIs, err := p.client.ListManagedAPIs(ctx)
	if err != nil {
		// If API client is not configured, skip API planning
		if err.Error() == "API client not configured" {
			return nil
		}
		return fmt.Errorf("failed to list current APIs: %w", err)
	}

	// Index current APIs by name
	currentByName := make(map[string]state.API)
	for _, api := range currentAPIs {
		currentByName[api.Name] = api
	}

	// Collect protection validation errors
	var protectionErrors []error

	// Compare each desired API
	for _, desiredAPI := range desired {
		current, exists := currentByName[desiredAPI.Name]

		if !exists {
			// CREATE action
			p.planAPICreate(desiredAPI, plan)
			// TODO: Plan child resources after API creation when SDK field mappings are clarified
		} else {
			// Check if update needed
			isProtected := labels.IsProtectedResource(current.NormalizedLabels)

			// Get protection status from desired configuration
			shouldProtect := false
			if desiredAPI.Kongctl != nil && desiredAPI.Kongctl.Protected {
				shouldProtect = true
			}

			// Handle protection changes
			if isProtected != shouldProtect {
				// When changing protection status, include any other field updates too
				_, updateFields := p.shouldUpdateAPI(current, desiredAPI)
				p.planAPIProtectionChangeWithFields(current, desiredAPI, isProtected, shouldProtect, updateFields, plan)
			} else {
				// Check if update needed based on configuration
				needsUpdate, updateFields := p.shouldUpdateAPI(current, desiredAPI)
				if needsUpdate {
					// Regular update - check protection
					if err := p.validateProtection("api", desiredAPI.Name, isProtected, ActionUpdate); err != nil {
						protectionErrors = append(protectionErrors, err)
					} else {
						p.planAPIUpdateWithFields(current, desiredAPI, updateFields, plan)
					}
				}
			}

			// TODO: Plan child resource changes
			// if err := p.planAPIChildResourceChanges(ctx, current, desiredAPI, plan); err != nil {
			//	return err
			// }
		}
	}

	// Check for managed resources to delete (sync mode only)
	if plan.Metadata.Mode == PlanModeSync {
		// Build set of desired API names
		desiredNames := make(map[string]bool)
		for _, api := range desired {
			desiredNames[api.Name] = true
		}

		// Find managed APIs not in desired state
		for name, current := range currentByName {
			if !desiredNames[name] {
				// Validate protection before adding DELETE
				isProtected := labels.IsProtectedResource(current.NormalizedLabels)
				if err := p.validateProtection("api", name, isProtected, ActionDelete); err != nil {
					protectionErrors = append(protectionErrors, err)
				} else {
					p.planAPIDelete(current, plan)
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

// planAPICreate creates a CREATE change for an API
func (p *Planner) planAPICreate(api resources.APIResource, plan *Plan) {
	fields := make(map[string]interface{})
	fields["name"] = api.Name
	if api.Description != nil {
		fields["description"] = *api.Description
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, api.GetRef()),
		ResourceType: "api",
		ResourceRef:  api.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	// Set protection label based on kongctl metadata
	if fields["labels"] == nil {
		fields["labels"] = make(map[string]interface{})
	}
	labelsMap := fields["labels"].(map[string]interface{})
	
	if api.Kongctl != nil && api.Kongctl.Protected {
		change.Protection = true
		labelsMap[labels.ProtectedKey] = labels.TrueValue
	} else {
		// Explicitly set to false when not protected
		labelsMap[labels.ProtectedKey] = labels.FalseValue
	}

	plan.AddChange(change)
}

// shouldUpdateAPI checks if API needs update based on configured fields only
func (p *Planner) shouldUpdateAPI(
	current state.API, 
	desired resources.APIResource,
) (bool, map[string]interface{}) {
	updates := make(map[string]interface{})
	
	// Only compare fields present in desired configuration
	if desired.Description != nil {
		currentDesc := getString(current.Description)
		if currentDesc != *desired.Description {
			updates["description"] = *desired.Description
		}
	}
	
	return len(updates) > 0, updates
}

// planAPIUpdateWithFields creates an UPDATE change with specific fields
func (p *Planner) planAPIUpdateWithFields(
	current state.API,
	desired resources.APIResource,
	updateFields map[string]interface{},
	plan *Plan,
) {
	fields := make(map[string]interface{})
	
	// Store the fields that need updating
	for field, newValue := range updateFields {
		fields[field] = newValue
	}
	
	// Always include name for identification
	fields["name"] = current.Name
	
	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, desired.GetRef()),
		ResourceType: "api",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    []string{},
	}
	
	// Check if already protected
	if labels.IsProtectedResource(current.NormalizedLabels) {
		change.Protection = true
	}
	
	plan.AddChange(change)
}

// planAPIProtectionChangeWithFields creates an UPDATE for protection status with optional field updates
func (p *Planner) planAPIProtectionChangeWithFields(
	current state.API, 
	desired resources.APIResource, 
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
	
	// Add protection label change to fields
	if fields["labels"] == nil {
		fields["labels"] = make(map[string]interface{})
	}
	labelsMap := fields["labels"].(map[string]interface{})
	if shouldProtect {
		labelsMap[labels.ProtectedKey] = labels.TrueValue
	} else {
		labelsMap[labels.ProtectedKey] = labels.FalseValue
	}
	
	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, desired.GetRef()),
		ResourceType: "api",
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

// planAPIDelete creates a DELETE change for an API
func (p *Planner) planAPIDelete(api state.API, plan *Plan) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, api.Name),
		ResourceType: "api",
		ResourceRef:  api.Name,
		ResourceID:   api.ID,
		Action:       ActionDelete,
		Fields:       map[string]interface{}{"name": api.Name},
		DependsOn:    []string{},
	}

	plan.AddChange(change)
}

