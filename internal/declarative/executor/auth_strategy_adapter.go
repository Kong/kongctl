package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// AuthStrategyAdapter implements ResourceOperations for application auth strategies
// This adapter handles the complexity of union types in the SDK
type AuthStrategyAdapter struct {
	client *state.Client
}

// NewAuthStrategyAdapter creates a new auth strategy adapter
func NewAuthStrategyAdapter(client *state.Client) *AuthStrategyAdapter {
	return &AuthStrategyAdapter{client: client}
}

// MapCreateFields maps fields to the appropriate auth strategy request type
// Note: This returns any because the SDK uses union types
func (a *AuthStrategyAdapter) MapCreateFields(
	_ context.Context, execCtx *ExecutionContext, fields map[string]any, 
	create *kkComps.CreateAppAuthStrategyRequest) error {
	// Extract namespace and protection from execution context
	namespace := execCtx.Namespace
	protection := execCtx.Protection

	// Validate required fields
	strategyType, ok := fields["strategy_type"].(string)
	if !ok {
		return fmt.Errorf("strategy_type is required")
	}

	name := common.ExtractResourceName(fields)
	
	var displayName string
	common.MapOptionalStringField(&displayName, fields, "display_name")

	// Handle labels using centralized helper
	userLabels := labels.ExtractLabelsFromField(fields["labels"])
	authLabels := labels.BuildCreateLabels(userLabels, namespace, protection)

	// Build the request based on strategy type
	switch strategyType {
	case "key_auth":
		req, err := a.buildKeyAuthRequest(name, displayName, authLabels, fields)
		if err != nil {
			return err
		}
		create.AppAuthStrategyKeyAuthRequest = req
		return nil

	case "openid_connect":
		req, err := a.buildOpenIDConnectRequest(name, displayName, authLabels, fields)
		if err != nil {
			return err
		}
		create.AppAuthStrategyOpenIDConnectRequest = req
		return nil

	default:
		return fmt.Errorf("unsupported strategy_type: %s", strategyType)
	}
}

// MapUpdateFields maps fields to UpdateAppAuthStrategyRequest
func (a *AuthStrategyAdapter) MapUpdateFields(
	_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	update *kkComps.UpdateAppAuthStrategyRequest, currentLabels map[string]string) error {
	// Extract namespace and protection from execution context
	namespace := execCtx.Namespace
	protection := execCtx.Protection

	// Update display name if present
	if displayName, ok := fields["display_name"].(string); ok {
		update.DisplayName = &displayName
	}

	// Handle labels using centralized helper
	desiredLabels := labels.ExtractLabelsFromField(fields["labels"])
	if desiredLabels != nil {
		// Get current labels if passed from planner
		plannerCurrentLabels := labels.ExtractLabelsFromField(fields[planner.FieldCurrentLabels])
		if plannerCurrentLabels != nil {
			currentLabels = plannerCurrentLabels
		}

		// Build update labels with removal support
		update.Labels = labels.BuildUpdateLabels(desiredLabels, currentLabels, namespace, protection)
	}

	// Handle config updates if present
	if configs, ok := fields["configs"].(map[string]any); ok {
		// Get strategy type from fields (passed by planner)
		strategyType, _ := fields[planner.FieldStrategyType].(string)
		
		if strategyType == "" {
			return fmt.Errorf("strategy type not provided for config update")
		}
		
		updateConfigs, err := a.buildUpdateConfigs(strategyType, configs)
		if err != nil {
			return fmt.Errorf("failed to build configs: %w", err)
		}
		update.Configs = updateConfigs
	}

	return nil
}

// Create creates a new auth strategy
func (a *AuthStrategyAdapter) Create(ctx context.Context, req kkComps.CreateAppAuthStrategyRequest,
	namespace string, _ *ExecutionContext) (string, error) {
	result, err := a.client.CreateApplicationAuthStrategy(ctx, req, namespace)
	if err != nil {
		return "", err
	}

	// Extract ID from the response based on type
	if result.CreateAppAuthStrategyResponse == nil {
		return "", fmt.Errorf("create response missing data")
	}

	// Try key auth response
	if keyAuthResp := result.GetCreateAppAuthStrategyResponseKeyAuth(); keyAuthResp != nil {
		return keyAuthResp.ID, nil
	}

	// Try openid connect response
	if oidcResp := result.GetCreateAppAuthStrategyResponseOpenidConnect(); oidcResp != nil {
		return oidcResp.ID, nil
	}

	return "", fmt.Errorf("unexpected response type")
}

// Update updates an existing auth strategy
func (a *AuthStrategyAdapter) Update(ctx context.Context, id string, req kkComps.UpdateAppAuthStrategyRequest,
	namespace string, _ *ExecutionContext) (string, error) {
	_, err := a.client.UpdateApplicationAuthStrategy(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Delete deletes an auth strategy
func (a *AuthStrategyAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteApplicationAuthStrategy(ctx, id)
}

// GetByName gets an auth strategy by name
func (a *AuthStrategyAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	strategy, err := a.client.GetAuthStrategyByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if strategy == nil {
		return nil, nil
	}
	return &AuthStrategyResourceInfo{strategy: strategy}, nil
}

// GetByID gets an auth strategy by ID
func (a *AuthStrategyAdapter) GetByID(_ context.Context, _ string, _ *ExecutionContext) (ResourceInfo, error) {
	// Use the existing GetByName approach since we don't have a direct GetByID in the client
	// This is a fallback - the planner should handle ID lookups
	return nil, nil
}

// ResourceType returns the resource type name
func (a *AuthStrategyAdapter) ResourceType() string {
	return "application_auth_strategy"
}

// RequiredFields returns the required fields for creation
func (a *AuthStrategyAdapter) RequiredFields() []string {
	return []string{"strategy_type"}
}

// SupportsUpdate returns true as auth strategies support updates
func (a *AuthStrategyAdapter) SupportsUpdate() bool {
	return true
}

// buildKeyAuthRequest builds a key auth strategy request
func (a *AuthStrategyAdapter) buildKeyAuthRequest(name, displayName string, labels map[string]string,
	fields map[string]any) (*kkComps.AppAuthStrategyKeyAuthRequest, error) {
	// Extract key auth config
	configs, ok := fields["configs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("configs is required for key_auth strategy")
	}
	
	keyAuthConfig, ok := configs["key-auth"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("configs.key-auth is required for key_auth strategy")
	}
	
	// Build key auth request
	req := &kkComps.AppAuthStrategyKeyAuthRequest{
		Name:         name,
		DisplayName:  displayName,
		StrategyType: kkComps.StrategyTypeKeyAuth,
		Configs: kkComps.AppAuthStrategyKeyAuthRequestConfigs{
			KeyAuth: kkComps.AppAuthStrategyConfigKeyAuth{},
		},
		Labels: labels,
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

	return req, nil
}

// buildOpenIDConnectRequest builds an OpenID Connect strategy request
func (a *AuthStrategyAdapter) buildOpenIDConnectRequest(name, displayName string, labels map[string]string,
	fields map[string]any) (*kkComps.AppAuthStrategyOpenIDConnectRequest, error) {
	// Extract openid connect config
	configs, ok := fields["configs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("configs is required for openid_connect strategy")
	}
	
	oidcConfig, ok := configs["openid-connect"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("configs.openid-connect is required for openid_connect strategy")
	}
	
	// Build openid connect request
	req := &kkComps.AppAuthStrategyOpenIDConnectRequest{
		Name:         name,
		DisplayName:  displayName,
		StrategyType: kkComps.AppAuthStrategyOpenIDConnectRequestStrategyTypeOpenidConnect,
		Configs: kkComps.AppAuthStrategyOpenIDConnectRequestConfigs{
			OpenidConnect: kkComps.AppAuthStrategyConfigOpenIDConnect{},
		},
		Labels: labels,
	}
	
	// Extract issuer (required)
	if issuer, ok := oidcConfig["issuer"].(string); ok {
		req.Configs.OpenidConnect.Issuer = issuer
	} else {
		return nil, fmt.Errorf("issuer is required for openid_connect strategy")
	}
	
	// Extract credential_claim
	req.Configs.OpenidConnect.CredentialClaim = a.extractStringSlice(oidcConfig["credential_claim"], []string{"sub"})
	
	// Extract scopes
	req.Configs.OpenidConnect.Scopes = a.extractStringSlice(oidcConfig["scopes"], nil)
	
	// Extract auth methods
	req.Configs.OpenidConnect.AuthMethods = a.extractStringSlice(oidcConfig["auth_methods"], nil)

	return req, nil
}

// buildUpdateConfigs builds the SDK Configs union type from planner data
func (a *AuthStrategyAdapter) buildUpdateConfigs(strategyType string,
	configs map[string]any) (*kkComps.Configs, error) {
	switch strategyType {
	case "key_auth":
		keyAuthConfig, ok := configs["key-auth"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("key-auth config missing for key_auth strategy")
		}
		
		keyAuth := kkComps.AppAuthStrategyConfigKeyAuth{}
		keyAuth.KeyNames = a.extractStringSlice(keyAuthConfig["key_names"], nil)
		
		two := kkComps.Two{
			KeyAuth: keyAuth,
		}
		
		configs := kkComps.CreateConfigsTwo(two)
		return &configs, nil
		
	case "openid_connect":
		oidcConfig, ok := configs["openid-connect"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("openid-connect config missing for openid_connect strategy")
		}
		
		// Use partial config for updates
		oidc := kkComps.PartialAppAuthStrategyConfigOpenIDConnect{}
		
		// Extract issuer
		if issuer, ok := oidcConfig["issuer"].(string); ok {
			oidc.Issuer = &issuer
		}
		
		// Extract other fields
		oidc.CredentialClaim = a.extractStringSlice(oidcConfig["credential_claim"], nil)
		oidc.Scopes = a.extractStringSlice(oidcConfig["scopes"], nil)
		oidc.AuthMethods = a.extractStringSlice(oidcConfig["auth_methods"], nil)
		
		one := kkComps.One{
			OpenidConnect: oidc,
		}
		
		configs := kkComps.CreateConfigsOne(one)
		return &configs, nil
		
	default:
		return nil, fmt.Errorf("unsupported strategy type: %s", strategyType)
	}
}

// extractStringSlice extracts a string slice from various input types
func (a *AuthStrategyAdapter) extractStringSlice(input any, defaultValue []string) []string {
	switch v := input.(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

// AuthStrategyResourceInfo wraps an ApplicationAuthStrategy to implement ResourceInfo
type AuthStrategyResourceInfo struct {
	strategy *state.ApplicationAuthStrategy
}

func (a *AuthStrategyResourceInfo) GetID() string {
	return a.strategy.ID
}

func (a *AuthStrategyResourceInfo) GetName() string {
	return a.strategy.Name
}

func (a *AuthStrategyResourceInfo) GetLabels() map[string]string {
	// Auth strategies already have map[string]string labels
	return a.strategy.NormalizedLabels
}

func (a *AuthStrategyResourceInfo) GetNormalizedLabels() map[string]string {
	return a.strategy.NormalizedLabels
}