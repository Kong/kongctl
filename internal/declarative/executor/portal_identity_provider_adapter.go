package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalIdentityProviderAdapter implements ResourceOperations for portal identity providers.
type PortalIdentityProviderAdapter struct {
	client *state.Client
}

// NewPortalIdentityProviderAdapter creates a new portal identity provider adapter.
func NewPortalIdentityProviderAdapter(client *state.Client) *PortalIdentityProviderAdapter {
	return &PortalIdentityProviderAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateIdentityProvider.
func (p *PortalIdentityProviderAdapter) MapCreateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any, create *kkComps.CreateIdentityProvider,
) error {
	typeName, ok := fields[planner.FieldType].(string)
	if !ok || typeName == "" {
		return fmt.Errorf("type is required")
	}
	providerType := kkComps.IdentityProviderType(typeName)
	create.Type = providerType.ToPointer()

	if enabled, ok := fields[planner.FieldEnabled].(bool); ok {
		create.Enabled = &enabled
	}
	if loginPath, ok := fields[planner.FieldLoginPath].(string); ok {
		create.LoginPath = &loginPath
	}

	config, err := createIdentityProviderConfigFromField(fields[planner.FieldConfig])
	if err != nil {
		return err
	}
	create.Config = config
	return nil
}

// MapUpdateFields maps planner fields to UpdateIdentityProvider.
func (p *PortalIdentityProviderAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateIdentityProvider,
	_ map[string]string,
) error {
	if enabled, ok := fields[planner.FieldEnabled].(bool); ok {
		update.Enabled = &enabled
	}
	if loginPath, ok := fields[planner.FieldLoginPath].(string); ok {
		update.LoginPath = &loginPath
	}
	if rawConfig, ok := fields[planner.FieldConfig]; ok {
		config, err := createIdentityProviderConfigFromField(rawConfig)
		if err != nil {
			return err
		}
		updateConfig, err := updateIdentityProviderConfigFromCreate(config)
		if err != nil {
			return err
		}
		update.Config = updateConfig
	}
	return nil
}

// Create creates a new portal identity provider.
func (p *PortalIdentityProviderAdapter) Create(
	ctx context.Context, req kkComps.CreateIdentityProvider, namespace string, execCtx *ExecutionContext,
) (string, error) {
	portalID, err := p.getPortalID(execCtx)
	if err != nil {
		return "", err
	}

	enabled := req.Enabled

	id, err := p.client.CreatePortalIdentityProvider(ctx, portalID, req, namespace)
	if err != nil {
		return "", err
	}

	if enabled != nil {
		update := kkComps.UpdateIdentityProvider{Enabled: enabled}
		if err := p.client.UpdatePortalIdentityProvider(
			ctx,
			portalID,
			id,
			update,
			namespace,
		); err != nil {
			return "", err
		}
	}

	return id, nil
}

// Update updates an existing portal identity provider.
func (p *PortalIdentityProviderAdapter) Update(
	ctx context.Context, id string, req kkComps.UpdateIdentityProvider, namespace string, execCtx *ExecutionContext,
) (string, error) {
	portalID, err := p.getPortalID(execCtx)
	if err != nil {
		return "", err
	}

	if err := p.client.UpdatePortalIdentityProvider(ctx, portalID, id, req, namespace); err != nil {
		return "", err
	}

	return id, nil
}

// Delete deletes a portal identity provider.
func (p *PortalIdentityProviderAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	portalID, err := p.getPortalID(execCtx)
	if err != nil {
		return err
	}
	return p.client.DeletePortalIdentityProvider(ctx, portalID, id)
}

// GetByName returns nil because portal identity providers are looked up by ID.
func (p *PortalIdentityProviderAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

// GetByID gets a portal identity provider by ID.
func (p *PortalIdentityProviderAdapter) GetByID(
	ctx context.Context, id string, execCtx *ExecutionContext,
) (ResourceInfo, error) {
	portalID, err := p.getPortalID(execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get portal ID for identity provider lookup: %w", err)
	}

	provider, err := p.client.GetPortalIdentityProvider(ctx, portalID, id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, nil
	}

	return &PortalIdentityProviderResourceInfo{provider: provider}, nil
}

// ResourceType returns the resource type name.
func (p *PortalIdentityProviderAdapter) ResourceType() string {
	return planner.ResourceTypePortalIdentityProvider
}

// RequiredFields returns the required fields for creation.
func (p *PortalIdentityProviderAdapter) RequiredFields() []string {
	return []string{planner.FieldType, planner.FieldConfig}
}

// SupportsUpdate returns true.
func (p *PortalIdentityProviderAdapter) SupportsUpdate() bool {
	return true
}

func (p *PortalIdentityProviderAdapter) getPortalID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for portal identity provider operations")
	}

	change := *execCtx.PlannedChange
	if portalRef, ok := change.References[planner.FieldPortalID]; ok && portalRef.ID != "" {
		return portalRef.ID, nil
	}
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}
	return "", fmt.Errorf("portal ID is required for portal identity provider operations")
}

func createIdentityProviderConfigFromField(raw any) (*kkComps.CreateIdentityProviderConfig, error) {
	switch config := raw.(type) {
	case kkComps.CreateIdentityProviderConfig:
		return &config, nil
	case *kkComps.CreateIdentityProviderConfig:
		return config, nil
	case map[string]any:
		encoded, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("failed to encode identity provider config: %w", err)
		}

		var decoded kkComps.CreateIdentityProviderConfig
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			return nil, fmt.Errorf("failed to decode identity provider config: %w", err)
		}
		return &decoded, nil
	default:
		return nil, fmt.Errorf("config is required")
	}
}

func updateIdentityProviderConfigFromCreate(
	config *kkComps.CreateIdentityProviderConfig,
) (*kkComps.UpdateIdentityProviderConfig, error) {
	if config == nil {
		return nil, nil
	}

	switch config.Type {
	case kkComps.CreateIdentityProviderConfigTypeOIDCIdentityProviderConfig:
		if config.OIDCIdentityProviderConfig == nil {
			return nil, fmt.Errorf("oidc identity provider config is required")
		}
		converted := kkComps.CreateUpdateIdentityProviderConfigOIDCIdentityProviderConfig(
			*config.OIDCIdentityProviderConfig,
		)
		return &converted, nil
	case kkComps.CreateIdentityProviderConfigTypeSAMLIdentityProviderConfigInput:
		if config.SAMLIdentityProviderConfigInput == nil {
			return nil, fmt.Errorf("saml identity provider config is required")
		}
		converted := kkComps.CreateUpdateIdentityProviderConfigSAMLIdentityProviderConfigInput(
			*config.SAMLIdentityProviderConfigInput,
		)
		return &converted, nil
	default:
		return nil, fmt.Errorf("identity provider config type is required")
	}
}

// PortalIdentityProviderResourceInfo implements ResourceInfo for portal identity providers.
type PortalIdentityProviderResourceInfo struct {
	provider *state.PortalIdentityProvider
}

// GetID returns the resource ID.
func (p *PortalIdentityProviderResourceInfo) GetID() string {
	if p.provider == nil {
		return ""
	}
	return p.provider.ID
}

// GetName returns the resource name.
func (p *PortalIdentityProviderResourceInfo) GetName() string {
	if p.provider == nil {
		return ""
	}
	return string(p.provider.Type)
}

// GetLabels returns the resource labels.
func (p *PortalIdentityProviderResourceInfo) GetLabels() map[string]string {
	return nil
}

// GetNormalizedLabels returns the normalized labels.
func (p *PortalIdentityProviderResourceInfo) GetNormalizedLabels() map[string]string {
	return map[string]string{}
}
