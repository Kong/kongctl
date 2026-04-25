package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

type DCRProviderAdapter struct {
	client *state.Client
}

func NewDCRProviderAdapter(client *state.Client) *DCRProviderAdapter {
	return &DCRProviderAdapter{client: client}
}

func (a *DCRProviderAdapter) MapCreateFields(
	_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	create *kkComps.CreateDcrProviderRequest,
) error {
	payload := map[string]any{
		planner.FieldName:                    common.ExtractResourceName(fields),
		planner.FieldDCRProviderProviderType: fields[planner.FieldDCRProviderProviderType],
		planner.FieldDCRProviderIssuer:       fields[planner.FieldDCRProviderIssuer],
		planner.FieldDCRProviderConfig:       fields[planner.FieldDCRProviderConfig],
	}
	if displayName, ok := fields[planner.FieldDisplayName].(string); ok && displayName != "" {
		payload[planner.FieldDisplayName] = displayName
	}

	userLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	managedLabels := labels.BuildCreateLabels(userLabels, execCtx.Namespace, execCtx.Protection)
	if managedLabels != nil {
		payload[planner.FieldLabels] = managedLabels
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal DCR provider create payload: %w", err)
	}
	if err := json.Unmarshal(payloadBytes, create); err != nil {
		return fmt.Errorf("failed to unmarshal DCR provider create payload: %w", err)
	}
	return nil
}

func (a *DCRProviderAdapter) MapUpdateFields(
	_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	update *kkComps.UpdateDcrProviderRequest, currentLabels map[string]string,
) error {
	payload := map[string]any{}
	if displayName, ok := fields[planner.FieldDisplayName].(string); ok {
		payload[planner.FieldDisplayName] = displayName
	}
	if issuer, ok := fields[planner.FieldDCRProviderIssuer].(string); ok {
		payload[planner.FieldDCRProviderIssuer] = issuer
	}

	desiredLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	if desiredLabels != nil {
		plannerCurrentLabels := labels.ExtractLabelsFromField(fields[planner.FieldCurrentLabels])
		if plannerCurrentLabels != nil {
			currentLabels = plannerCurrentLabels
		}
		payload[planner.FieldLabels] = labels.BuildUpdateLabels(
			desiredLabels,
			currentLabels,
			execCtx.Namespace,
			execCtx.Protection,
		)
	}

	if displayName, ok := payload[planner.FieldDisplayName].(string); ok {
		update.DisplayName = &displayName
	}
	if issuer, ok := payload[planner.FieldDCRProviderIssuer].(string); ok {
		update.Issuer = &issuer
	}
	if updateLabels, ok := payload[planner.FieldLabels].(map[string]*string); ok {
		update.Labels = updateLabels
	}

	if dcrConfig, ok := fields[planner.FieldDCRProviderConfig]; ok {
		providerType, _ := fields[planner.FieldDCRProviderUpdateType].(string)
		if providerType == "" {
			return fmt.Errorf("provider type not provided for DCR config update")
		}

		config, err := buildDCRProviderUpdateConfig(providerType, dcrConfig)
		if err != nil {
			return fmt.Errorf("failed to build DCR provider update config: %w", err)
		}
		update.DcrConfig = config
	}

	return nil
}

func (a *DCRProviderAdapter) Create(ctx context.Context, req kkComps.CreateDcrProviderRequest,
	namespace string, _ *ExecutionContext,
) (string, error) {
	provider, err := a.client.CreateDCRProvider(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return provider.ID, nil
}

func (a *DCRProviderAdapter) Update(ctx context.Context, id string, req kkComps.UpdateDcrProviderRequest,
	namespace string, _ *ExecutionContext,
) (string, error) {
	if err := a.client.UpdateDCRProvider(ctx, id, req, namespace); err != nil {
		return "", err
	}
	return id, nil
}

func (a *DCRProviderAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteDCRProvider(ctx, id)
}

func (a *DCRProviderAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	provider, err := a.client.GetDCRProviderByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, nil
	}
	return &DCRProviderResourceInfo{provider: provider}, nil
}

func (a *DCRProviderAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	provider, err := a.client.GetDCRProviderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, nil
	}
	return &DCRProviderResourceInfo{provider: provider}, nil
}

func (a *DCRProviderAdapter) ResourceType() string {
	return planner.ResourceTypeDCRProvider
}

func (a *DCRProviderAdapter) RequiredFields() []string {
	return []string{planner.FieldDCRProviderProviderType, planner.FieldDCRProviderIssuer, planner.FieldDCRProviderConfig}
}

func (a *DCRProviderAdapter) SupportsUpdate() bool {
	return true
}

func buildDCRProviderUpdateConfig(providerType string, raw any) (*kkComps.DcrConfig, error) {
	config, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("DCR config must be an object")
	}

	switch providerType {
	case "auth0":
		req := kkComps.UpdateDcrConfigAuth0InRequest{}
		mapOptionalString(&req.InitialClientID, config, "initial_client_id")
		mapOptionalString(&req.InitialClientSecret, config, "initial_client_secret")
		mapOptionalString(&req.InitialClientAudience, config, "initial_client_audience")
		mapOptionalBool(&req.UseDeveloperManagedScopes, config, "use_developer_managed_scopes")
		result := kkComps.CreateDcrConfigUpdateDcrConfigAuth0InRequest(req)
		return &result, nil

	case "azureAd":
		req := kkComps.UpdateDcrConfigAzureAdInRequest{}
		mapOptionalString(&req.InitialClientID, config, "initial_client_id")
		mapOptionalString(&req.InitialClientSecret, config, "initial_client_secret")
		result := kkComps.CreateDcrConfigUpdateDcrConfigAzureAdInRequest(req)
		return &result, nil

	case "curity":
		req := kkComps.UpdateDcrConfigCurityInRequest{}
		mapOptionalString(&req.InitialClientID, config, "initial_client_id")
		mapOptionalString(&req.InitialClientSecret, config, "initial_client_secret")
		result := kkComps.CreateDcrConfigUpdateDcrConfigCurityInRequest(req)
		return &result, nil

	case "okta":
		req := kkComps.UpdateDcrConfigOktaInRequest{}
		mapOptionalString(&req.DcrToken, config, "dcr_token")
		result := kkComps.CreateDcrConfigUpdateDcrConfigOktaInRequest(req)
		return &result, nil

	case "http":
		req := kkComps.UpdateDcrConfigHTTPInRequest{}
		mapOptionalString(&req.DcrBaseURL, config, "dcr_base_url")
		mapOptionalString(&req.APIKey, config, "api_key")
		mapOptionalBool(&req.DisableEventHooks, config, "disable_event_hooks")
		mapOptionalBool(&req.DisableRefreshSecret, config, "disable_refresh_secret")
		mapOptionalBool(&req.AllowMultipleCredentials, config, "allow_multiple_credentials")
		result := kkComps.CreateDcrConfigUpdateDcrConfigHTTPInRequest(req)
		return &result, nil

	default:
		return nil, fmt.Errorf("unsupported DCR provider type: %s", providerType)
	}
}

func mapOptionalString(target **string, fields map[string]any, key string) {
	if value, ok := fields[key].(string); ok {
		*target = &value
	}
}

func mapOptionalBool(target **bool, fields map[string]any, key string) {
	if value, ok := fields[key].(bool); ok {
		*target = &value
	}
}

type DCRProviderResourceInfo struct {
	provider *state.DCRProvider
}

func (d *DCRProviderResourceInfo) GetID() string {
	return d.provider.ID
}

func (d *DCRProviderResourceInfo) GetName() string {
	return d.provider.Name
}

func (d *DCRProviderResourceInfo) GetLabels() map[string]string {
	return d.provider.NormalizedLabels
}

func (d *DCRProviderResourceInfo) GetNormalizedLabels() map[string]string {
	return d.provider.NormalizedLabels
}
