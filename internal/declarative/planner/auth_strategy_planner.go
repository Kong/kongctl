package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// authStrategyPlannerImpl implements planning logic for auth strategy resources
type authStrategyPlannerImpl struct {
	*BasePlanner
}

// NewAuthStrategyPlanner creates a new auth strategy planner
func NewAuthStrategyPlanner(base *BasePlanner) AuthStrategyPlanner {
	return &authStrategyPlannerImpl{
		BasePlanner: base,
	}
}

// PlanChanges generates changes for auth strategy resources
func (p *authStrategyPlannerImpl) PlanChanges(ctx context.Context, plan *Plan) error {
	desired := p.GetDesiredAuthStrategies()
	
	// Skip if no auth strategies to plan and not in sync mode
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	// Fetch current managed auth strategies
	currentStrategies, err := p.GetClient().ListManagedAuthStrategies(ctx)
	if err != nil {
		// If app auth client is not configured, skip auth strategy planning
		if err.Error() == "AppAuth client not configured" {
			return nil
		}
		return fmt.Errorf("failed to list current auth strategies: %w", err)
	}

	// Index current strategies by name
	currentByName := make(map[string]state.ApplicationAuthStrategy)
	for _, strategy := range currentStrategies {
		currentByName[strategy.Name] = strategy
	}

	// Collect protection validation errors
	protectionErrors := &ProtectionErrorCollector{}

	// Compare each desired auth strategy
	for _, desiredStrategy := range desired {
		// Extract name based on strategy type
		var name string
		switch desiredStrategy.Type {
		case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
			if desiredStrategy.AppAuthStrategyKeyAuthRequest != nil {
				name = desiredStrategy.AppAuthStrategyKeyAuthRequest.Name
			}
		case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
			if desiredStrategy.AppAuthStrategyOpenIDConnectRequest != nil {
				name = desiredStrategy.AppAuthStrategyOpenIDConnectRequest.Name
			}
		}
		
		if name == "" {
			continue
		}
		
		current, exists := currentByName[name]

		if !exists {
			// CREATE action
			p.planAuthStrategyCreate(desiredStrategy, plan)
		} else {
			// Check if update needed
			isProtected := labels.IsProtectedResource(current.NormalizedLabels)

			// Get protection status from desired configuration
			shouldProtect := false
			if desiredStrategy.Kongctl != nil && desiredStrategy.Kongctl.Protected {
				shouldProtect = true
			}

			// Handle protection changes
			if isProtected != shouldProtect {
				// When changing protection status, include any other field updates too
				_, updateFields := p.shouldUpdateAuthStrategy(current, desiredStrategy)
				p.planAuthStrategyProtectionChangeWithFields(
					current, desiredStrategy, isProtected, shouldProtect, updateFields, plan)
			} else {
				// Check if update needed based on configuration
				needsUpdate, updateFields := p.shouldUpdateAuthStrategy(current, desiredStrategy)
				if needsUpdate {
					// Regular update - check protection
					err := p.ValidateProtection("auth_strategy", name, isProtected, ActionUpdate)
					protectionErrors.Add(err)
					if err == nil {
						p.planAuthStrategyUpdateWithFields(current, desiredStrategy, updateFields, plan)
					}
				}
			}
		}
	}

	// Check for managed resources to delete (sync mode only)
	if plan.Metadata.Mode == PlanModeSync {
		// Build set of desired strategy names
		desiredNames := make(map[string]bool)
		for _, strategy := range desired {
			var name string
			switch strategy.Type {
			case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
				if strategy.AppAuthStrategyKeyAuthRequest != nil {
					name = strategy.AppAuthStrategyKeyAuthRequest.Name
				}
			case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
				if strategy.AppAuthStrategyOpenIDConnectRequest != nil {
					name = strategy.AppAuthStrategyOpenIDConnectRequest.Name
				}
			}
			if name != "" {
				desiredNames[name] = true
			}
		}

		// Find managed strategies not in desired state
		for name, current := range currentByName {
			if !desiredNames[name] {
				// Validate protection before adding DELETE
				isProtected := labels.IsProtectedResource(current.NormalizedLabels)
				err := p.ValidateProtection("auth_strategy", name, isProtected, ActionDelete)
				protectionErrors.Add(err)
				if err == nil {
					p.planAuthStrategyDelete(current, plan)
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

// planAuthStrategyCreate creates a CREATE change for an auth strategy
func (p *authStrategyPlannerImpl) planAuthStrategyCreate(
	strategy resources.ApplicationAuthStrategyResource, plan *Plan) {
	fields := make(map[string]interface{})
	
	// Extract fields based on strategy type
	var name string
	var displayName string
	var labels map[string]string
	var kongctl *resources.KongctlMeta
	
	switch strategy.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		if strategy.AppAuthStrategyKeyAuthRequest != nil {
			name = strategy.AppAuthStrategyKeyAuthRequest.Name
			displayName = strategy.AppAuthStrategyKeyAuthRequest.DisplayName
			labels = strategy.AppAuthStrategyKeyAuthRequest.Labels
			
			// Set strategy type
			fields["strategy_type"] = "key_auth"
			
			// Set config under configs map
			if strategy.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames != nil {
				fields["configs"] = map[string]interface{}{
					"key-auth": map[string]interface{}{
						"key_names": strategy.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames,
					},
				}
			}
		}
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		if strategy.AppAuthStrategyOpenIDConnectRequest != nil {
			name = strategy.AppAuthStrategyOpenIDConnectRequest.Name
			displayName = strategy.AppAuthStrategyOpenIDConnectRequest.DisplayName
			labels = strategy.AppAuthStrategyOpenIDConnectRequest.Labels
			
			// Set strategy type
			fields["strategy_type"] = "openid_connect"
			
			// Set config under configs map
			oidcConfig := make(map[string]interface{})
			if strategy.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect.Issuer != "" {
				oidcConfig["issuer"] = strategy.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect.Issuer
			}
			if strategy.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect.CredentialClaim != nil {
				oidcConfig["credential_claim"] = strategy.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect.CredentialClaim
			}
			if strategy.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect.Scopes != nil {
				oidcConfig["scopes"] = strategy.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect.Scopes
			}
			if strategy.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect.AuthMethods != nil {
				oidcConfig["auth_methods"] = strategy.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect.AuthMethods
			}
			
			if len(oidcConfig) > 0 {
				fields["configs"] = map[string]interface{}{
					"openid-connect": oidcConfig,
				}
			}
		}
	}
	
	kongctl = strategy.Kongctl
	
	fields["name"] = name
	if displayName != "" {
		fields["display_name"] = displayName
	}

	change := PlannedChange{
		ID:           p.NextChangeID(ActionCreate, strategy.GetRef()),
		ResourceType: "application_auth_strategy",
		ResourceRef:  strategy.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	// Always set protection status explicitly
	if kongctl != nil && kongctl.Protected {
		change.Protection = true
	} else {
		change.Protection = false
	}

	// Copy user-defined labels only (protection label will be added during execution)
	if len(labels) > 0 {
		labelsMap := make(map[string]interface{})
		for k, v := range labels {
			labelsMap[k] = v
		}
		fields["labels"] = labelsMap
	}

	plan.AddChange(change)
}

// shouldUpdateAuthStrategy checks if auth strategy needs update based on configured fields only
func (p *authStrategyPlannerImpl) shouldUpdateAuthStrategy(
	current state.ApplicationAuthStrategy,
	desired resources.ApplicationAuthStrategyResource,
) (bool, map[string]interface{}) {
	updateFields := make(map[string]interface{})

	// Extract fields based on strategy type
	var displayName string
	var labels map[string]string
	var keyNames []string
	
	switch desired.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		if desired.AppAuthStrategyKeyAuthRequest != nil {
			displayName = desired.AppAuthStrategyKeyAuthRequest.DisplayName
			labels = desired.AppAuthStrategyKeyAuthRequest.Labels
			
			// Extract key names if present
			if desired.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames != nil {
				keyNames = desired.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames
			}
		}
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		if desired.AppAuthStrategyOpenIDConnectRequest != nil {
			displayName = desired.AppAuthStrategyOpenIDConnectRequest.DisplayName
			labels = desired.AppAuthStrategyOpenIDConnectRequest.Labels
		}
	}

	// Only compare fields present in desired configuration
	if displayName != "" {
		if current.DisplayName != displayName {
			updateFields["display_name"] = displayName
		}
	}

	// Check key_names updates
	if len(keyNames) > 0 {
		// Convert current key names to a comparable format
		currentKeyNames := make([]string, 0)
		if current.Configs != nil {
			if keyAuthConfig, ok := current.Configs["key-auth"].(map[string]interface{}); ok {
				// Try different type assertions for key_names
				switch kn := keyAuthConfig["key_names"].(type) {
				case []interface{}:
					for _, name := range kn {
						if str, ok := name.(string); ok {
							currentKeyNames = append(currentKeyNames, str)
						}
					}
				case []string:
					currentKeyNames = kn
				}
			}
		}
		
		// Compare lengths first
		if len(currentKeyNames) != len(keyNames) {
			updateFields["configs"] = map[string]interface{}{
				"key-auth": map[string]interface{}{
					"key_names": keyNames,
				},
			}
		} else {
			// Compare values
			for i, desiredName := range keyNames {
				if i < len(currentKeyNames) && currentKeyNames[i] != desiredName {
					updateFields["configs"] = map[string]interface{}{
						"key-auth": map[string]interface{}{
							"key_names": keyNames,
						},
					}
					break
				}
			}
		}
	}

	// Compare user labels if any are specified
	if len(labels) > 0 {
		if compareUserLabels(current.NormalizedLabels, labels) {
			updateFields["labels"] = labels
		}
	}

	return len(updateFields) > 0, updateFields
}

// planAuthStrategyUpdateWithFields creates an UPDATE change with specific fields
func (p *authStrategyPlannerImpl) planAuthStrategyUpdateWithFields(
	current state.ApplicationAuthStrategy,
	desired resources.ApplicationAuthStrategyResource,
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
		ID:           p.NextChangeID(ActionUpdate, desired.GetRef()),
		ResourceType: "application_auth_strategy",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	// Set protection status based on current state
	change.Protection = labels.IsProtectedResource(current.NormalizedLabels)

	plan.AddChange(change)
}

// planAuthStrategyProtectionChangeWithFields creates an UPDATE for protection status with optional field updates
func (p *authStrategyPlannerImpl) planAuthStrategyProtectionChangeWithFields(
	current state.ApplicationAuthStrategy,
	desired resources.ApplicationAuthStrategyResource,
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
		ID:           p.NextChangeID(ActionUpdate, desired.GetRef()),
		ResourceType: "application_auth_strategy",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		Protection: ProtectionChange{
			Old: wasProtected,
			New: shouldProtect,
		},
		DependsOn: []string{},
	}

	plan.AddChange(change)
}

// planAuthStrategyDelete creates a DELETE change for an auth strategy
func (p *authStrategyPlannerImpl) planAuthStrategyDelete(strategy state.ApplicationAuthStrategy, plan *Plan) {
	change := PlannedChange{
		ID:           p.NextChangeID(ActionDelete, strategy.Name),
		ResourceType: "application_auth_strategy",
		ResourceRef:  strategy.Name,
		ResourceID:   strategy.ID,
		Action:       ActionDelete,
		Fields:       map[string]interface{}{"name": strategy.Name},
		DependsOn:    []string{},
	}

	plan.AddChange(change)
}