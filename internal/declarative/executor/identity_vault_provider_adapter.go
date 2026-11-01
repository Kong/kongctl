package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/identityproviders"
)

type identityVaultProviderAdapter struct{ client *state.Client }

func (a *identityVaultProviderAdapter) ResourceType() string {
	return planner.ResourceTypeIdentityVaultProvider
}

func (a *identityVaultProviderAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldTemplateName}
}
func (a *identityVaultProviderAdapter) SupportsUpdate() bool { return true }

func vaultProviderPayload(fields map[string]any) (resources.IdentityVaultProviderResource, error) {
	var value resources.IdentityVaultProviderResource
	for key := range fields {
		switch key {
		case planner.FieldName, planner.FieldTemplateName, planner.FieldClientID, planner.FieldClientSecret,
			planner.FieldCredentialType, planner.FieldDCREnabled, planner.FieldPKCEEnabled, planner.FieldEnabled,
			planner.FieldScopes, planner.FieldTokenEndpoint, planner.FieldAuthorizationEndpoint,
			planner.FieldRegistrationEndpoint, planner.FieldRefreshBufferSeconds:
		default:
			return value, fmt.Errorf("unsupported Vault provider field %q", key)
		}
		if fields[key] == nil {
			return value, fmt.Errorf(
				"vault provider field %s cannot be null; omit it to preserve existing settings",
				key,
			)
		}
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return value, fmt.Errorf("invalid Vault provider payload")
	}
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("invalid Vault provider field types")
	}
	return value, identityproviders.ValidateCreate(value.CreateBody())
}

func (a *identityVaultProviderAdapter) MapCreateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.CreateVaultProviderBody,
) error {
	value, err := vaultProviderPayload(fields)
	if err != nil {
		return err
	}
	*request = value.CreateBody()
	return nil
}

func (a *identityVaultProviderAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.PatchVaultProviderBody, _ map[string]string,
) error {
	value, err := vaultProviderPayload(fields)
	if err != nil {
		return err
	}
	*request = kkComps.PatchVaultProviderBody{
		Name: &value.Name, Enabled: value.Enabled, ClientSecret: value.ClientSecret, Scopes: value.Scopes,
		TokenEndpoint: value.TokenEndpoint, AuthorizationEndpoint: value.AuthorizationEndpoint,
		PkceEnabled: value.PKCEEnabled, RefreshBufferSeconds: value.RefreshBufferSeconds,
	}
	return nil
}

func (a *identityVaultProviderAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, nil
}

func (a *identityVaultProviderAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	directory, err := identityVaultDirectoryID(execCtx)
	if err != nil {
		return nil, err
	}
	value, err := a.client.GetIdentityVaultProvider(ctx, directory, id)
	if err != nil || value == nil {
		return nil, err
	}
	return value, nil
}

func (a *identityVaultProviderAdapter) checkIdentity(
	ctx context.Context,
	directory, id, name, template string,
) (*state.IdentityVaultProvider, error) {
	values, err := a.client.ListIdentityVaultProviders(ctx, directory)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if value.Name != name {
			continue
		}
		if id == "" {
			return nil, fmt.Errorf("vault provider name already exists; regenerate the plan")
		}
		if value.ID != id || value.TemplateName != template {
			return nil, fmt.Errorf("vault provider identity changed; regenerate the plan")
		}
		return &value, nil
	}
	if id != "" {
		return nil, fmt.Errorf("vault provider identity changed; regenerate the plan")
	}
	return nil, nil
}

func (a *identityVaultProviderAdapter) Create(ctx context.Context, request kkComps.CreateVaultProviderBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("vault provider namespace mismatch")
	}
	directory, err := identityVaultWriteDirectory(ctx, a.client, execCtx, "vault provider")
	if err != nil {
		return "", err
	}
	if _, err := a.checkIdentity(ctx, directory, "", request.Name, string(request.TemplateName)); err != nil {
		return "", err
	}
	value, err := a.client.CreateIdentityVaultProvider(ctx, directory, request)
	if err != nil {
		return "", err
	}
	// The API has no enabled create field. A requested initial disabled state
	// needs this explicit second write; creation is not atomically disabled.
	if enabled, configured := execCtx.PlannedChange.Fields[planner.FieldEnabled].(bool); configured &&
		enabled != *value.Enabled {
		if _, err := identityVaultWriteDirectory(ctx, a.client, execCtx, "vault provider"); err != nil {
			return value.ID, err
		}
		if err := a.client.UpdateIdentityVaultProvider(ctx, directory, value.ID, value.Name,
			kkComps.PatchVaultProviderBody{Enabled: &enabled}); err != nil {
			return value.ID, fmt.Errorf(
				"provider created but initial enabled state could not be set; inspect and replan: %w",
				err,
			)
		}
	}
	return value.ID, nil
}

func (a *identityVaultProviderAdapter) Update(ctx context.Context, id string, request kkComps.PatchVaultProviderBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("vault provider namespace mismatch")
	}
	directory, err := identityVaultWriteDirectory(ctx, a.client, execCtx, "vault provider")
	if err != nil {
		return "", err
	}
	payload, err := vaultProviderPayload(execCtx.PlannedChange.Fields)
	if err != nil {
		return "", err
	}
	current, err := a.checkIdentity(ctx, directory, id, payload.Name, payload.TemplateName)
	if err != nil {
		return "", err
	}
	for field, desired := range execCtx.PlannedChange.Fields {
		var actual any
		switch field {
		case planner.FieldTemplateName:
			actual = current.TemplateName
		case planner.FieldCredentialType:
			actual = current.CredentialType
		case planner.FieldClientID:
			if current.ClientID != nil {
				actual = *current.ClientID
			}
		case planner.FieldDCREnabled:
			actual = *current.DCREnabled
		case planner.FieldRegistrationEndpoint:
			if current.RegistrationEndpointEffective != nil {
				actual = *current.RegistrationEndpointEffective
			}
		default:
			continue
		}
		if actual != desired {
			return "", fmt.Errorf(
				"vault provider field %s changed; verified PATCH cannot replace it; regenerate the plan",
				field,
			)
		}
	}
	if err := a.client.UpdateIdentityVaultProvider(ctx, directory, id, payload.Name, request); err != nil {
		return "", err
	}
	return id, nil
}

func (a *identityVaultProviderAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	directory, err := identityVaultWriteDirectory(ctx, a.client, execCtx, "vault provider")
	if err != nil {
		return err
	}
	fields := execCtx.PlannedChange.Fields
	name, nameOK := fields[planner.FieldName].(string)
	template, templateOK := fields[planner.FieldTemplateName].(string)
	if len(fields) != 2 || !nameOK || !templateOK || template == "" {
		return fmt.Errorf("vault provider deletion requires only name and template_name")
	}
	if err := identityproviders.ValidateName(name); err != nil {
		return err
	}
	if _, err := a.checkIdentity(ctx, directory, id, name, template); err != nil {
		return err
	}
	return a.client.DeleteIdentityVaultProvider(ctx, directory, id, name)
}

func (e *Executor) registerIdentityVaultProviderExecutor() {
	e.registerResourceExecutor(
		crudResourceExecutor(NewBaseExecutor(&identityVaultProviderAdapter{client: e.client}, e.client, e.dryRun)),
	)
}
