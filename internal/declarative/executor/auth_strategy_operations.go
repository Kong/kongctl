package executor

import (
	"context"
	"fmt"
	"os"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// createApplicationAuthStrategy handles CREATE operations for application auth strategies
func (e *Executor) createApplicationAuthStrategy(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Debug logging
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == "true"
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG [auth_strategy_operations]: "+format+"\n", args...)
		}
	}
	
	debugLog("Creating application auth strategy with fields: %+v", change.Fields)
	
	// Get strategy type
	strategyType, ok := change.Fields["strategy_type"].(string)
	if !ok {
		return "", fmt.Errorf("strategy_type is required")
	}
	
	// Get name (required for all types)
	name, ok := change.Fields["name"].(string)
	if !ok {
		return "", fmt.Errorf("name is required")
	}
	
	// Get display name (required for all types)
	displayName, ok := change.Fields["display_name"].(string)
	if !ok {
		return "", fmt.Errorf("display_name is required")
	}
	
	// Handle labels - preserve user labels (auth strategies use map[string]string)
	authLabels := make(map[string]string)
	
	// Copy user-defined labels from the change
	if labelsField, ok := change.Fields["labels"].(map[string]interface{}); ok {
		debugLog("Found user labels in fields: %+v", labelsField)
		for k, v := range labelsField {
			if strVal, ok := v.(string); ok {
				// Only copy user labels (non-KONGCTL labels)
				if !labels.IsKongctlLabel(k) {
					authLabels[k] = strVal
					debugLog("Adding user label: %s=%s", k, strVal)
				}
			}
		}
	}
	
	// Add protection label based on change.Protection field
	protectionValue := labels.FalseValue
	if prot, ok := change.Protection.(bool); ok && prot {
		protectionValue = labels.TrueValue
		debugLog("Setting protection label to true")
	} else {
		debugLog("Setting protection label to false")
	}
	authLabels[labels.ProtectedKey] = protectionValue
	
	// Always add managed label
	managedValue := labels.TrueValue
	authLabels[labels.ManagedKey] = managedValue
	
	// Build the request based on strategy type
	switch strategyType {
	case "key_auth":
		// Extract key auth config
		configs, ok := change.Fields["configs"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("configs is required for key_auth strategy")
		}
		
		keyAuthConfig, ok := configs["key_auth"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("configs.key_auth is required for key_auth strategy")
		}
		
		// Build key auth request
		req := &kkComps.AppAuthStrategyKeyAuthRequest{
			Name:         name,
			DisplayName:  displayName,
			StrategyType: kkComps.StrategyTypeKeyAuth,
			Configs: kkComps.AppAuthStrategyKeyAuthRequestConfigs{
				KeyAuth: kkComps.AppAuthStrategyConfigKeyAuth{},
			},
			Labels: authLabels,
		}
		
		// Extract key names - handle both []string and []interface{}
		switch keyNames := keyAuthConfig["key_names"].(type) {
		case []string:
			req.Configs.KeyAuth.KeyNames = keyNames
		case []interface{}:
			names := make([]string, 0, len(keyNames))
			for _, kn := range keyNames {
				if name, ok := kn.(string); ok {
					names = append(names, name)
				}
			}
			if len(names) > 0 {
				req.Configs.KeyAuth.KeyNames = names
			}
		}
		
		debugLog("Creating key_auth strategy: %+v", req)
		
		// Call API
		if e.dryRun {
			return "dry-run-auth-strategy-id", nil
		}
		
		createReq := kkComps.CreateAppAuthStrategyRequest{
			AppAuthStrategyKeyAuthRequest: req,
		}
		
		result, err := e.client.CreateApplicationAuthStrategy(ctx, createReq)
		if err != nil {
			return "", fmt.Errorf("failed to create key_auth strategy: %w", err)
		}
		
		// Extract ID from the response
		if result.CreateAppAuthStrategyResponse == nil {
			return "", fmt.Errorf("create response missing data")
		}
		
		if keyAuthResp := result.GetCreateAppAuthStrategyResponseKeyAuth(); keyAuthResp != nil {
			return keyAuthResp.ID, nil
		}
		
		return "", fmt.Errorf("unexpected response type for key_auth strategy")
		
	case "openid_connect":
		// Extract openid connect config
		configs, ok := change.Fields["configs"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("configs is required for openid_connect strategy")
		}
		
		oidcConfig, ok := configs["openid_connect"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("configs.openid_connect is required for openid_connect strategy")
		}
		
		// Build openid connect request
		req := &kkComps.AppAuthStrategyOpenIDConnectRequest{
			Name:         name,
			DisplayName:  displayName,
			StrategyType: kkComps.AppAuthStrategyOpenIDConnectRequestStrategyTypeOpenidConnect,
			Configs: kkComps.AppAuthStrategyOpenIDConnectRequestConfigs{
				OpenidConnect: kkComps.AppAuthStrategyConfigOpenIDConnect{},
			},
			Labels: authLabels,
		}
		
		// Extract issuer (required)
		if issuer, ok := oidcConfig["issuer"].(string); ok {
			req.Configs.OpenidConnect.Issuer = issuer
		} else {
			return "", fmt.Errorf("issuer is required for openid_connect strategy")
		}
		
		// Extract credential_claim (required by API) - handle both []string and []interface{}
		switch credentialClaim := oidcConfig["credential_claim"].(type) {
		case []string:
			req.Configs.OpenidConnect.CredentialClaim = credentialClaim
		case []interface{}:
			claims := make([]string, 0, len(credentialClaim))
			for _, c := range credentialClaim {
				if claim, ok := c.(string); ok {
					claims = append(claims, claim)
				}
			}
			req.Configs.OpenidConnect.CredentialClaim = claims
		default:
			// Default to "sub" if not provided
			req.Configs.OpenidConnect.CredentialClaim = []string{"sub"}
		}
		
		// Extract scopes - handle both []string and []interface{}
		switch scopes := oidcConfig["scopes"].(type) {
		case []string:
			req.Configs.OpenidConnect.Scopes = scopes
		case []interface{}:
			scopeStrs := make([]string, 0, len(scopes))
			for _, s := range scopes {
				if scope, ok := s.(string); ok {
					scopeStrs = append(scopeStrs, scope)
				}
			}
			if len(scopeStrs) > 0 {
				req.Configs.OpenidConnect.Scopes = scopeStrs
			}
		}
		
		// Extract auth methods - handle both []string and []interface{}
		switch authMethods := oidcConfig["auth_methods"].(type) {
		case []string:
			req.Configs.OpenidConnect.AuthMethods = authMethods
		case []interface{}:
			methods := make([]string, 0, len(authMethods))
			for _, m := range authMethods {
				if method, ok := m.(string); ok {
					methods = append(methods, method)
				}
			}
			if len(methods) > 0 {
				req.Configs.OpenidConnect.AuthMethods = methods
			}
		}
		
		debugLog("Creating openid_connect strategy: %+v", req)
		debugLog("OpenID config details - Issuer: %s, Scopes: %v, AuthMethods: %v, CredentialClaim: %v", 
			req.Configs.OpenidConnect.Issuer,
			req.Configs.OpenidConnect.Scopes,
			req.Configs.OpenidConnect.AuthMethods,
			req.Configs.OpenidConnect.CredentialClaim)
		
		// Call API
		if e.dryRun {
			return "dry-run-auth-strategy-id", nil
		}
		
		createReq := kkComps.CreateAppAuthStrategyRequest{
			AppAuthStrategyOpenIDConnectRequest: req,
		}
		
		result, err := e.client.CreateApplicationAuthStrategy(ctx, createReq)
		if err != nil {
			return "", fmt.Errorf("failed to create openid_connect strategy: %w", err)
		}
		
		// Extract ID from the response
		if result.CreateAppAuthStrategyResponse == nil {
			return "", fmt.Errorf("create response missing data")
		}
		
		if oidcResp := result.GetCreateAppAuthStrategyResponseOpenidConnect(); oidcResp != nil {
			return oidcResp.ID, nil
		}
		
		return "", fmt.Errorf("unexpected response type for openid_connect strategy")
		
	default:
		return "", fmt.Errorf("unsupported strategy_type: %s", strategyType)
	}
}

// updateApplicationAuthStrategy handles UPDATE operations for application auth strategies
func (e *Executor) updateApplicationAuthStrategy(ctx context.Context, change planner.PlannedChange) (string, error) {
	if e.client == nil {
		return "", fmt.Errorf("client not configured")
	}

	// Build update request
	updateReq := kkComps.UpdateAppAuthStrategyRequest{}
	
	// Update display name if present
	if displayName, ok := change.Fields["display_name"].(string); ok {
		updateReq.DisplayName = &displayName
	}
	
	// Handle labels - preserve user labels and add management labels
	if labelsField, ok := change.Fields["labels"].(map[string]interface{}); ok {
		authLabels := make(map[string]string)
		
		// Copy user-defined labels
		for k, v := range labelsField {
			if strVal, ok := v.(string); ok {
				// Only copy user labels (non-KONGCTL labels)
				if !labels.IsKongctlLabel(k) {
					authLabels[k] = strVal
				}
			}
		}
		
		// Add protection label based on change.Protection field
		if protChange, ok := change.Protection.(planner.ProtectionChange); ok {
			if protChange.New {
				authLabels[labels.ProtectedKey] = labels.TrueValue
			} else {
				authLabels[labels.ProtectedKey] = labels.FalseValue
			}
		}
		
		// Always add managed label
		authLabels[labels.ManagedKey] = labels.TrueValue
		
		// Convert to pointer map for SDK
		pointerLabels := make(map[string]*string)
		for k, v := range authLabels {
			val := v
			pointerLabels[k] = &val
		}
		updateReq.Labels = pointerLabels
	}
	
	// Note: Config updates are complex with the SDK
	// For now, we'll skip config updates and only handle display name and labels
	// TODO: Implement config updates when SDK types are clearer
	
	// Call update API
	_, err := e.client.UpdateApplicationAuthStrategy(ctx, change.ResourceID, updateReq)
	if err != nil {
		return "", fmt.Errorf("failed to update application auth strategy: %w", err)
	}
	
	return change.ResourceID, nil
}

// deleteApplicationAuthStrategy handles DELETE operations for application auth strategies
func (e *Executor) deleteApplicationAuthStrategy(ctx context.Context, change planner.PlannedChange) error {
	if e.client == nil {
		return fmt.Errorf("client not configured")
	}

	return e.client.DeleteApplicationAuthStrategy(ctx, change.ResourceID)
}
