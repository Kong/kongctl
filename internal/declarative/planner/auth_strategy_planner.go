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

	// Get namespace from context
	namespace, ok := ctx.Value(NamespaceContextKey).(string)
	if !ok {
		// Default to all namespaces for backward compatibility
		namespace = "*"
	}
	
	// Fetch current managed auth strategies from the specific namespace
	namespaceFilter := []string{namespace}
	currentStrategies, err := p.GetClient().ListManagedAuthStrategies(ctx, namespaceFilter)
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
			if desiredStrategy.Kongctl != nil && desiredStrategy.Kongctl.Protected != nil && *desiredStrategy.Kongctl.Protected {
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
					// Check for strategy type change error
					if errMsg, hasError := updateFields[FieldError].(string); hasError {
						protectionErrors.Add(fmt.Errorf("%s", errMsg))
					} else {
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
			keyAuthConfig := make(map[string]interface{})
			if strategy.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames != nil {
				keyAuthConfig["key_names"] = strategy.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames
			}
			
			fields["configs"] = map[string]interface{}{
				"key-auth": keyAuthConfig,
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
			
			fields["configs"] = map[string]interface{}{
				"openid-connect": oidcConfig,
			}
		}
	}
	
	kongctl = strategy.Kongctl
	
	fields["name"] = name
	if displayName != "" {
		fields["display_name"] = displayName
	}

	change := PlannedChange{
		ID:           p.NextChangeID(ActionCreate, "application_auth_strategy", strategy.GetRef()),
		ResourceType: "application_auth_strategy",
		ResourceRef:  strategy.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	// Only set protection if explicitly specified
	if kongctl != nil && kongctl.Protected != nil {
		change.Protection = *kongctl.Protected
	}

	// Extract namespace
	if kongctl != nil && kongctl.Namespace != nil {
		change.Namespace = *kongctl.Namespace
	} else {
		// This should not happen as loader should have set default namespace
		change.Namespace = DefaultNamespace
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

	// First, check if strategy type is changing - this is not supported
	var desiredStrategyType string
	switch desired.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		desiredStrategyType = "key_auth"
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		desiredStrategyType = "openid_connect"
	}

	if current.StrategyType != desiredStrategyType {
		// Return error via updateFields to be handled by caller
		updateFields[FieldError] = fmt.Sprintf(
			"changing strategy_type from %s to %s is not supported. Please delete and recreate the auth strategy",
			current.StrategyType, desiredStrategyType)
		return true, updateFields
	}

	// Extract fields based on strategy type
	var displayName string
	var desiredLabels map[string]string
	
	switch desired.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		if desired.AppAuthStrategyKeyAuthRequest != nil {
			displayName = desired.AppAuthStrategyKeyAuthRequest.DisplayName
			desiredLabels = desired.AppAuthStrategyKeyAuthRequest.Labels
		}
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		if desired.AppAuthStrategyOpenIDConnectRequest != nil {
			displayName = desired.AppAuthStrategyOpenIDConnectRequest.DisplayName
			desiredLabels = desired.AppAuthStrategyOpenIDConnectRequest.Labels
		}
	}

	// Only compare fields present in desired configuration
	if displayName != "" {
		if current.DisplayName != displayName {
			updateFields["display_name"] = displayName
		}
	}

	// Check config updates based on strategy type
	switch desired.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		if desired.AppAuthStrategyKeyAuthRequest != nil {
			// Check key_names updates
			if desired.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames != nil {
				desiredKeyNames := desired.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames
				
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
				if len(currentKeyNames) != len(desiredKeyNames) {
					updateFields["configs"] = map[string]interface{}{
						"key-auth": map[string]interface{}{
							"key_names": desiredKeyNames,
						},
					}
				} else {
					// Compare values
					for i, desiredName := range desiredKeyNames {
						if i < len(currentKeyNames) && currentKeyNames[i] != desiredName {
							updateFields["configs"] = map[string]interface{}{
								"key-auth": map[string]interface{}{
									"key_names": desiredKeyNames,
								},
							}
							break
						}
					}
				}
			}
		}
		
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		if desired.AppAuthStrategyOpenIDConnectRequest != nil {
			oidcConfig := &desired.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect
			
			// Get current OIDC config
			var currentOIDC map[string]interface{}
			if current.Configs != nil {
				if oidc, ok := current.Configs["openid-connect"].(map[string]interface{}); ok {
					currentOIDC = oidc
				}
			}
			
			oidcUpdates := make(map[string]interface{})
			hasUpdates := false
			
			// Check issuer
			if oidcConfig.Issuer != "" {
				currentIssuer, _ := currentOIDC["issuer"].(string)
				if currentIssuer != oidcConfig.Issuer {
					oidcUpdates["issuer"] = oidcConfig.Issuer
					hasUpdates = true
				}
			}
			
			// Check credential_claim
			if oidcConfig.CredentialClaim != nil {
				currentClaims := extractStringSlice(currentOIDC["credential_claim"])
				if !stringSlicesEqual(currentClaims, oidcConfig.CredentialClaim) {
					oidcUpdates["credential_claim"] = oidcConfig.CredentialClaim
					hasUpdates = true
				}
			}
			
			// Check scopes
			if oidcConfig.Scopes != nil {
				currentScopes := extractStringSlice(currentOIDC["scopes"])
				if !stringSlicesEqual(currentScopes, oidcConfig.Scopes) {
					oidcUpdates["scopes"] = oidcConfig.Scopes
					hasUpdates = true
				}
			}
			
			// Check auth_methods
			if oidcConfig.AuthMethods != nil {
				currentMethods := extractStringSlice(currentOIDC["auth_methods"])
				if !stringSlicesEqual(currentMethods, oidcConfig.AuthMethods) {
					oidcUpdates["auth_methods"] = oidcConfig.AuthMethods
					hasUpdates = true
				}
			}
			
			if hasUpdates {
				updateFields["configs"] = map[string]interface{}{
					"openid-connect": oidcUpdates,
				}
			}
		}
	}

	// Check if labels are defined in the desired state
	if desiredLabels != nil {
		// Compare only user labels to determine if update is needed
		if labels.CompareUserLabels(current.NormalizedLabels, desiredLabels) {
			// User labels differ, include all labels in update
			updateFields["labels"] = desiredLabels
		}
	}

	return len(updateFields) > 0, updateFields
}

// extractStringSlice converts interface{} to []string
func extractStringSlice(val interface{}) []string {
	result := make([]string, 0)
	switch v := val.(type) {
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
	case []string:
		result = v
	}
	return result
}

// stringSlicesEqual compares two string slices
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
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
		// Skip internal error field
		if field != FieldError {
			fields[field] = newValue
		}
	}
	
	// Pass strategy type to executor
	fields[FieldStrategyType] = current.StrategyType
	
	// Pass current labels so executor can properly handle removals
	if _, hasLabels := updateFields["labels"]; hasLabels {
		fields[FieldCurrentLabels] = current.NormalizedLabels
	}

	change := PlannedChange{
		ID:           p.NextChangeID(ActionUpdate, "application_auth_strategy", desired.GetRef()),
		ResourceType: "application_auth_strategy",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	// Set protection status based on current state
	change.Protection = labels.IsProtectedResource(current.NormalizedLabels)

	// Extract namespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		change.Namespace = *desired.Kongctl.Namespace
	} else {
		// This should not happen as loader should have set default namespace
		change.Namespace = DefaultNamespace
	}

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
		ID:           p.NextChangeID(ActionUpdate, "application_auth_strategy", desired.GetRef()),
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

	// Extract namespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		change.Namespace = *desired.Kongctl.Namespace
	} else {
		// This should not happen as loader should have set default namespace
		change.Namespace = DefaultNamespace
	}

	plan.AddChange(change)
}

// planAuthStrategyDelete creates a DELETE change for an auth strategy
func (p *authStrategyPlannerImpl) planAuthStrategyDelete(strategy state.ApplicationAuthStrategy, plan *Plan) {
	change := PlannedChange{
		ID:           p.NextChangeID(ActionDelete, "application_auth_strategy", strategy.Name),
		ResourceType: "application_auth_strategy",
		ResourceRef:  strategy.Name,
		ResourceID:   strategy.ID,
		Action:       ActionDelete,
		Fields:       map[string]interface{}{"name": strategy.Name},
		DependsOn:    []string{},
	}

	// Extract namespace from labels (for existing resources being deleted)
	if ns, ok := strategy.NormalizedLabels[labels.NamespaceKey]; ok {
		change.Namespace = ns
	} else {
		// Fallback to default
		change.Namespace = DefaultNamespace
	}

	plan.AddChange(change)
}