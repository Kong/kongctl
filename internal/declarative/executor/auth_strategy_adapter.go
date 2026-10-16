package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/fieldcontracts"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
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
	create *kkComps.CreateAppAuthStrategyRequest,
) error {
	// Validate required fields
	strategyType, ok := fields[planner.FieldAuthStrategyType].(string)
	if !ok {
		return fmt.Errorf("strategy_type is required")
	}

	name := common.ExtractResourceName(fields)

	var displayName string
	common.MapOptionalStringField(&displayName, fields, planner.FieldDisplayName)

	// Handle labels using centralized helper
	userLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	authLabels := labels.BuildCreateLabels(userLabels, execCtx.Namespace, execCtx.Protection)

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
	_ context.Context, _ *ExecutionContext, fields map[string]any,
	update *kkComps.UpdateAppAuthStrategyRequest, _ map[string]string,
) error {
	// Update display name if present
	if displayName, ok := fields[planner.FieldDisplayName].(string); ok {
		update.DisplayName = &displayName
	}

	// Handle config updates if present
	if configs, ok := fields[planner.FieldConfigs].(map[string]any); ok {
		// Get strategy type from fields (passed by planner)
		strategyType, _ := fields[planner.FieldStrategyType].(string)

		if strategyType == "" {
			return fmt.Errorf("strategy type not provided for config update")
		}

		updateConfigs, err := fieldcontracts.MapAuthStrategyConfigUpdate(strategyType, configs)
		if err != nil {
			return fmt.Errorf("failed to build configs: %w", err)
		}
		update.Configs = updateConfigs
	}

	if providerID, ok := fields[planner.FieldDCRProviderID].(string); ok && providerID != "" {
		update.DcrProviderID = &providerID
	}

	return nil
}

func (a *AuthStrategyAdapter) MapUpdateLabels(
	execCtx *ExecutionContext,
	update *kkComps.UpdateAppAuthStrategyRequest,
	desiredLabels map[string]string,
	currentLabels map[string]string,
) {
	mapPointerUpdateLabels(&update.Labels, execCtx, desiredLabels, currentLabels)
}

// Create creates a new auth strategy
func (a *AuthStrategyAdapter) Create(ctx context.Context, req kkComps.CreateAppAuthStrategyRequest,
	namespace string, _ *ExecutionContext,
) (string, error) {
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
	namespace string, _ *ExecutionContext,
) (string, error) {
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
func (a *AuthStrategyAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	strategy, err := a.client.GetAuthStrategyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if strategy == nil {
		return nil, nil
	}
	return &AuthStrategyResourceInfo{strategy: strategy}, nil
}

// ResourceType returns the resource type name
func (a *AuthStrategyAdapter) ResourceType() string {
	return planner.ResourceTypeApplicationAuthStrategy
}

// RequiredFields returns the required fields for creation
func (a *AuthStrategyAdapter) RequiredFields() []string {
	return []string{planner.FieldAuthStrategyType}
}

// SupportsUpdate returns true as auth strategies support updates
func (a *AuthStrategyAdapter) SupportsUpdate() bool {
	return true
}

// buildKeyAuthRequest builds a key auth strategy request
func (a *AuthStrategyAdapter) buildKeyAuthRequest(name, displayName string, labels map[string]string,
	fields map[string]any,
) (*kkComps.AppAuthStrategyKeyAuthRequest, error) {
	config, err := fieldcontracts.MapKeyAuthConfigCreate(fields[planner.FieldConfigs])
	if err != nil {
		return nil, err
	}

	// Build key auth request
	req := &kkComps.AppAuthStrategyKeyAuthRequest{
		Name:         name,
		DisplayName:  displayName,
		StrategyType: kkComps.StrategyTypeKeyAuth,
		Configs: kkComps.AppAuthStrategyKeyAuthRequestConfigs{
			KeyAuth: config,
		},
		Labels: labels,
	}

	return req, nil
}

// buildOpenIDConnectRequest builds an OpenID Connect strategy request
func (a *AuthStrategyAdapter) buildOpenIDConnectRequest(name, displayName string, labels map[string]string,
	fields map[string]any,
) (*kkComps.AppAuthStrategyOpenIDConnectRequest, error) {
	config, err := fieldcontracts.MapOIDCConfigCreate(fields[planner.FieldConfigs])
	if err != nil {
		return nil, err
	}

	// Build openid connect request
	req := &kkComps.AppAuthStrategyOpenIDConnectRequest{
		Name:         name,
		DisplayName:  displayName,
		StrategyType: kkComps.AppAuthStrategyOpenIDConnectRequestStrategyTypeOpenidConnect,
		Configs: kkComps.AppAuthStrategyOpenIDConnectRequestConfigs{
			OpenidConnect: config,
		},
		Labels: labels,
	}

	if providerID, ok := fields[planner.FieldDCRProviderID].(string); ok && providerID != "" {
		req.DcrProviderID = &providerID
	}

	return req, nil
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
