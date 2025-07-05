package executor

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// createApplicationAuthStrategy handles CREATE operations for application auth strategies
func (e *Executor) createApplicationAuthStrategy(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Debug logging
	debugEnabled := os.Getenv(labels.DebugEnvVar) == "true"
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
	
	// Add last updated timestamp
	authLabels[labels.LastUpdatedKey] = time.Now().UTC().Format("20060102-150405Z")
	
	// Build the request based on strategy type
	switch strategyType {
	case "key_auth":
		// Extract key auth config
		configs, ok := change.Fields["configs"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("configs is required for key_auth strategy")
		}
		
		keyAuthConfig, ok := configs["key-auth"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("configs.key-auth is required for key_auth strategy")
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
		
		oidcConfig, ok := configs["openid-connect"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("configs.openid-connect is required for openid_connect strategy")
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
	// Debug logging
	debugEnabled := os.Getenv(labels.DebugEnvVar) == "true"
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG [auth_strategy_operations/update]: "+format+"\n", args...)
		}
	}
	
	debugLog("Updating auth strategy with change: %+v", change)
	
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
	// Try both type assertions since planner might send either type
	var labelsToProcess map[string]string
	if labelsField, ok := change.Fields["labels"].(map[string]interface{}); ok {
		// Handle map[string]interface{} case
		debugLog("Labels are type map[string]interface{}")
		labelsToProcess = make(map[string]string)
		for k, v := range labelsField {
			if strVal, ok := v.(string); ok {
				labelsToProcess[k] = strVal
			}
		}
	} else if labelsField, ok := change.Fields["labels"].(map[string]string); ok {
		// Handle map[string]string case (what planner actually sends)
		debugLog("Labels are type map[string]string: %+v", labelsField)
		labelsToProcess = labelsField
	} else if change.Fields["labels"] != nil {
		debugLog("Labels field exists but has unexpected type: %T", change.Fields["labels"])
	}
	
	if labelsToProcess != nil {
		authLabels := make(map[string]string)
		
		// Get current labels if passed from planner
		currentLabels := make(map[string]string)
		if currentLabelsField, ok := change.Fields["_current_labels"].(map[string]string); ok {
			currentLabels = currentLabelsField
		}
		
		// Copy user-defined labels
		for k, v := range labelsToProcess {
			// Only copy user labels (non-KONGCTL labels)
			if !labels.IsKongctlLabel(k) {
				authLabels[k] = v
			}
		}
		
		// Add protection label based on change.Protection field
		if protChange, ok := change.Protection.(planner.ProtectionChange); ok {
			if protChange.New {
				authLabels[labels.ProtectedKey] = labels.TrueValue
			} else {
				authLabels[labels.ProtectedKey] = labels.FalseValue
			}
		} else if prot, ok := change.Protection.(bool); ok {
			// For regular updates, preserve the protection status
			if prot {
				authLabels[labels.ProtectedKey] = labels.TrueValue
			} else {
				authLabels[labels.ProtectedKey] = labels.FalseValue
			}
		} else {
			// If no protection info provided, default to false
			authLabels[labels.ProtectedKey] = labels.FalseValue
		}
		
		// Always add managed label
		authLabels[labels.ManagedKey] = labels.TrueValue
		
		// Add last updated timestamp
		authLabels[labels.LastUpdatedKey] = time.Now().UTC().Format("20060102-150405Z")
		
		// Convert to pointer map for SDK
		pointerLabels := make(map[string]*string)
		
		// First, add all labels we want to keep/update
		for k, v := range authLabels {
			val := v
			pointerLabels[k] = &val
		}
		
		// Then, add nil values for current user labels that should be removed
		for k := range currentLabels {
			// If it's a user label in current state but not in desired state, remove it
			if !labels.IsKongctlLabel(k) {
				if _, exists := authLabels[k]; !exists {
					pointerLabels[k] = nil
					debugLog("Marking label for removal: %s", k)
				}
			}
		}
		
		updateReq.Labels = pointerLabels
		
		debugLog("Final labels for update: %+v", authLabels)
		debugLog("Update request labels (pointers): %+v", pointerLabels)
	}
	
	// Handle config updates if present
	if configs, ok := change.Fields["configs"].(map[string]interface{}); ok {
		// Get strategy type from fields (passed by planner)
		strategyType, _ := change.Fields["_strategy_type"].(string)
		
		if strategyType == "" {
			// Try to determine from current state
			// This shouldn't happen if planner is working correctly
			return "", fmt.Errorf("strategy type not provided for config update")
		}
		
		updateConfigs, err := buildAuthStrategyConfigs(strategyType, configs)
		if err != nil {
			return "", fmt.Errorf("failed to build configs: %w", err)
		}
		updateReq.Configs = updateConfigs
		
		debugLog("Config update - strategy type: %s, configs: %+v", strategyType, updateConfigs)
	}
	
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

// buildAuthStrategyConfigs builds the SDK Configs union type from planner data
func buildAuthStrategyConfigs(strategyType string, configs map[string]interface{}) (*kkComps.Configs, error) {
	switch strategyType {
	case "key_auth":
		keyAuthConfig, ok := configs["key-auth"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("key-auth config missing for key_auth strategy")
		}
		
		return buildKeyAuthConfigs(keyAuthConfig)
		
	case "openid_connect":
		oidcConfig, ok := configs["openid-connect"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("openid-connect config missing for openid_connect strategy")
		}
		
		return buildOpenIDConnectConfigs(oidcConfig)
		
	default:
		return nil, fmt.Errorf("unsupported strategy type: %s", strategyType)
	}
}

// buildKeyAuthConfigs builds key auth configs for the SDK
func buildKeyAuthConfigs(keyAuthConfig map[string]interface{}) (*kkComps.Configs, error) {
	keyAuth := kkComps.AppAuthStrategyConfigKeyAuth{}
	
	// Extract key names - handle both []string and []interface{}
	switch keyNames := keyAuthConfig["key_names"].(type) {
	case []string:
		keyAuth.KeyNames = keyNames
	case []interface{}:
		names := make([]string, 0, len(keyNames))
		for _, kn := range keyNames {
			if name, ok := kn.(string); ok {
				names = append(names, name)
			}
		}
		keyAuth.KeyNames = names
	}
	
	two := kkComps.Two{
		KeyAuth: keyAuth,
	}
	
	configs := kkComps.CreateConfigsTwo(two)
	return &configs, nil
}

// buildOpenIDConnectConfigs builds OpenID Connect configs for the SDK
func buildOpenIDConnectConfigs(oidcConfig map[string]interface{}) (*kkComps.Configs, error) {
	// Use partial config for updates
	oidc := kkComps.PartialAppAuthStrategyConfigOpenIDConnect{}
	
	// Extract issuer
	if issuer, ok := oidcConfig["issuer"].(string); ok {
		oidc.Issuer = &issuer
	}
	
	// Extract credential_claim - handle both []string and []interface{}
	switch credentialClaim := oidcConfig["credential_claim"].(type) {
	case []string:
		oidc.CredentialClaim = credentialClaim
	case []interface{}:
		claims := make([]string, 0, len(credentialClaim))
		for _, c := range credentialClaim {
			if claim, ok := c.(string); ok {
				claims = append(claims, claim)
			}
		}
		if len(claims) > 0 {
			oidc.CredentialClaim = claims
		}
	}
	
	// Extract scopes - handle both []string and []interface{}
	switch scopes := oidcConfig["scopes"].(type) {
	case []string:
		oidc.Scopes = scopes
	case []interface{}:
		scopeStrs := make([]string, 0, len(scopes))
		for _, s := range scopes {
			if scope, ok := s.(string); ok {
				scopeStrs = append(scopeStrs, scope)
			}
		}
		if len(scopeStrs) > 0 {
			oidc.Scopes = scopeStrs
		}
	}
	
	// Extract auth methods - handle both []string and []interface{}
	switch authMethods := oidcConfig["auth_methods"].(type) {
	case []string:
		oidc.AuthMethods = authMethods
	case []interface{}:
		methods := make([]string, 0, len(authMethods))
		for _, m := range authMethods {
			if method, ok := m.(string); ok {
				methods = append(methods, method)
			}
		}
		if len(methods) > 0 {
			oidc.AuthMethods = methods
		}
	}
	
	one := kkComps.One{
		OpenidConnect: oidc,
	}
	
	configs := kkComps.CreateConfigsOne(one)
	return &configs, nil
}
