package executor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/log"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// createApplicationAuthStrategy handles CREATE operations for application auth strategies
// Deprecated: Use AuthStrategyAdapter with BaseExecutor instead
//nolint:unused // kept for test compatibility, will be removed in Phase 2 cleanup
func (e *Executor) createApplicationAuthStrategy(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	
	logger.Debug("Creating application auth strategy",
		slog.Any("fields", change.Fields))
	
	// Validate required fields
	if err := common.ValidateRequiredFields(change.Fields, []string{"strategy_type"}); err != nil {
		return "", common.WrapWithResourceContext(err, "auth_strategy", "")
	}
	
	strategyType, _ := change.Fields["strategy_type"].(string)
	name := common.ExtractResourceName(change.Fields)
	
	var displayName string
	common.MapOptionalStringField(&displayName, change.Fields, "display_name")
	
	// Handle labels using centralized helper
	userLabels := labels.ExtractLabelsFromField(change.Fields["labels"])
	authLabels := labels.BuildCreateLabels(userLabels, change.Namespace, change.Protection)
	
	logger.Debug("Created labels for auth strategy",
		slog.Any("labels", authLabels))
	
	// Build the request based on strategy type
	switch strategyType {
	case "key_auth":
		// Extract key auth config
		configs, ok := change.Fields["configs"].(map[string]any)
		if !ok {
			return "", fmt.Errorf("configs is required for key_auth strategy")
		}
		
		keyAuthConfig, ok := configs["key-auth"].(map[string]any)
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
		
		// Extract key names - handle both []string and []any
		switch keyNames := keyAuthConfig["key_names"].(type) {
		case []string:
			req.Configs.KeyAuth.KeyNames = keyNames
		case []any:
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
		
		logger.Debug("Creating key_auth strategy",
			slog.Any("request", req))
		
		// Call API
		if e.dryRun {
			return "dry-run-auth-strategy-id", nil
		}
		
		createReq := kkComps.CreateAppAuthStrategyRequest{
			AppAuthStrategyKeyAuthRequest: req,
		}
		
		result, err := e.client.CreateApplicationAuthStrategy(ctx, createReq, change.Namespace)
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
		configs, ok := change.Fields["configs"].(map[string]any)
		if !ok {
			return "", fmt.Errorf("configs is required for openid_connect strategy")
		}
		
		oidcConfig, ok := configs["openid-connect"].(map[string]any)
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
		
		// Extract credential_claim (required by API) - handle both []string and []any
		switch credentialClaim := oidcConfig["credential_claim"].(type) {
		case []string:
			req.Configs.OpenidConnect.CredentialClaim = credentialClaim
		case []any:
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
		
		// Extract scopes - handle both []string and []any
		switch scopes := oidcConfig["scopes"].(type) {
		case []string:
			req.Configs.OpenidConnect.Scopes = scopes
		case []any:
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
		
		// Extract auth methods - handle both []string and []any
		switch authMethods := oidcConfig["auth_methods"].(type) {
		case []string:
			req.Configs.OpenidConnect.AuthMethods = authMethods
		case []any:
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
		
		logger.Debug("Creating openid_connect strategy",
			slog.Any("request", req))
		logger.Debug("OpenID config details",
			slog.String("issuer", req.Configs.OpenidConnect.Issuer),
			slog.Any("scopes", req.Configs.OpenidConnect.Scopes),
			slog.Any("auth_methods", req.Configs.OpenidConnect.AuthMethods),
			slog.Any("credential_claim", req.Configs.OpenidConnect.CredentialClaim))
		
		// Call API
		if e.dryRun {
			return "dry-run-auth-strategy-id", nil
		}
		
		createReq := kkComps.CreateAppAuthStrategyRequest{
			AppAuthStrategyOpenIDConnectRequest: req,
		}
		
		result, err := e.client.CreateApplicationAuthStrategy(ctx, createReq, change.Namespace)
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
// Deprecated: Use AuthStrategyAdapter with BaseExecutor instead
//nolint:unused // kept for test compatibility, will be removed in Phase 2 cleanup
func (e *Executor) updateApplicationAuthStrategy(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Get logger from context
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	
	logger.Debug("Updating auth strategy",
		slog.Any("change", change))
	
	if e.client == nil {
		return "", fmt.Errorf("client not configured")
	}

	// Build update request
	updateReq := kkComps.UpdateAppAuthStrategyRequest{}
	
	// Update display name if present
	if displayName, ok := change.Fields["display_name"].(string); ok {
		updateReq.DisplayName = &displayName
	}
	
	// Handle labels using centralized helper
	desiredLabels := labels.ExtractLabelsFromField(change.Fields["labels"])
	if desiredLabels != nil {
		// Get current labels if passed from planner
		currentLabels := labels.ExtractLabelsFromField(change.Fields[planner.FieldCurrentLabels])
		
		// Build update labels with removal support
		updateReq.Labels = labels.BuildUpdateLabels(desiredLabels, currentLabels, change.Namespace, change.Protection)
		
		logger.Debug("Update request labels (with removal support)",
			slog.Any("labels", updateReq.Labels))
	}
	
	// Handle config updates if present
	if configs, ok := change.Fields["configs"].(map[string]any); ok {
		// Get strategy type from fields (passed by planner)
		strategyType, _ := change.Fields[planner.FieldStrategyType].(string)
		
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
		
		logger.Debug("Config update",
			slog.String("strategy_type", strategyType),
			slog.Any("configs", updateConfigs))
	}
	
	// Call update API
	_, err := e.client.UpdateApplicationAuthStrategy(ctx, change.ResourceID, updateReq, change.Namespace)
	if err != nil {
		return "", fmt.Errorf("failed to update application auth strategy: %w", err)
	}
	
	return change.ResourceID, nil
}

// deleteApplicationAuthStrategy handles DELETE operations for application auth strategies
// Deprecated: Use AuthStrategyAdapter with BaseExecutor instead
//nolint:unused // kept for test compatibility, will be removed in Phase 2 cleanup
func (e *Executor) deleteApplicationAuthStrategy(ctx context.Context, change planner.PlannedChange) error {
	if e.client == nil {
		return fmt.Errorf("client not configured")
	}

	return e.client.DeleteApplicationAuthStrategy(ctx, change.ResourceID)
}

// buildAuthStrategyConfigs builds the SDK Configs union type from planner data
//nolint:unused // deprecated, will be removed in Phase 2 cleanup
func buildAuthStrategyConfigs(strategyType string, configs map[string]any) (*kkComps.Configs, error) {
	switch strategyType {
	case "key_auth":
		keyAuthConfig, ok := configs["key-auth"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("key-auth config missing for key_auth strategy")
		}
		
		return buildKeyAuthConfigs(keyAuthConfig)
		
	case "openid_connect":
		oidcConfig, ok := configs["openid-connect"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("openid-connect config missing for openid_connect strategy")
		}
		
		return buildOpenIDConnectConfigs(oidcConfig)
		
	default:
		return nil, fmt.Errorf("unsupported strategy type: %s", strategyType)
	}
}

// buildKeyAuthConfigs builds key auth configs for the SDK
//nolint:unused // deprecated, will be removed in Phase 2 cleanup
func buildKeyAuthConfigs(keyAuthConfig map[string]any) (*kkComps.Configs, error) {
	keyAuth := kkComps.AppAuthStrategyConfigKeyAuth{}
	
	// Extract key names - handle both []string and []any
	switch keyNames := keyAuthConfig["key_names"].(type) {
	case []string:
		keyAuth.KeyNames = keyNames
	case []any:
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
//nolint:unused // deprecated, will be removed in Phase 2 cleanup
func buildOpenIDConnectConfigs(oidcConfig map[string]any) (*kkComps.Configs, error) {
	// Use partial config for updates
	oidc := kkComps.PartialAppAuthStrategyConfigOpenIDConnect{}
	
	// Extract issuer
	if issuer, ok := oidcConfig["issuer"].(string); ok {
		oidc.Issuer = &issuer
	}
	
	// Extract credential_claim - handle both []string and []any
	switch credentialClaim := oidcConfig["credential_claim"].(type) {
	case []string:
		oidc.CredentialClaim = credentialClaim
	case []any:
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
	
	// Extract scopes - handle both []string and []any
	switch scopes := oidcConfig["scopes"].(type) {
	case []string:
		oidc.Scopes = scopes
	case []any:
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
	
	// Extract auth methods - handle both []string and []any
	switch authMethods := oidcConfig["auth_methods"].(type) {
	case []string:
		oidc.AuthMethods = authMethods
	case []any:
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
