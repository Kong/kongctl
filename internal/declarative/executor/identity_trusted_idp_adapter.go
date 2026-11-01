package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/identitytrust"
)

type identityTrustedIDPAdapter struct{ client *state.Client }

func (a *identityTrustedIDPAdapter) ResourceType() string {
	return planner.ResourceTypeIdentityTrustedIDP
}

func (a *identityTrustedIDPAdapter) RequiredFields() []string {
	return []string{planner.FieldIssuerURL}
}
func (a *identityTrustedIDPAdapter) SupportsUpdate() bool { return true }

func trustedIDPPayload(fields map[string]any) (resources.IdentityTrustedIDPResource, error) {
	var value resources.IdentityTrustedIDPResource
	for key := range fields {
		switch key {
		case planner.FieldIssuerURL, planner.FieldDisplayName, planner.FieldJwksURI,
			planner.FieldJitProvisioningEnabled, planner.FieldVaultAccessEnabled, planner.FieldJit:
		default:
			return value, fmt.Errorf("unsupported trusted IdP field %q", key)
		}
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return value, fmt.Errorf("invalid trusted IdP payload")
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, fmt.Errorf("invalid trusted IdP field types: %w", err)
	}
	return value, identitytrust.ValidatePublic(value.IssuerURL, value.DisplayName, value.JwksURI, value.Jit)
}

func (a *identityTrustedIDPAdapter) MapCreateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.CreateTrustedIdpPayload,
) error {
	value, err := trustedIDPPayload(fields)
	if err != nil {
		return err
	}
	*request = value.CreateBody()
	return nil
}

func (a *identityTrustedIDPAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.ReplaceTrustedIdpBody, _ map[string]string,
) error {
	value, err := trustedIDPPayload(fields)
	if err != nil {
		return err
	}
	*request = kkComps.ReplaceTrustedIdpBody{
		IssuerURL: value.IssuerURL, DisplayName: value.DisplayName, JwksURI: value.JwksURI,
		JitProvisioningEnabled: value.JitProvisioningEnabled, VaultAccessEnabled: value.VaultAccessEnabled, Jit: value.Jit,
	}
	return nil
}

func (a *identityTrustedIDPAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, nil
}

func (a *identityTrustedIDPAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	directory, err := identityVaultDirectoryID(execCtx)
	if err != nil {
		return nil, err
	}
	value, err := a.client.GetIdentityTrustedIDP(ctx, directory, id)
	if err != nil || value == nil {
		return nil, err
	}
	return value, nil
}

func (a *identityTrustedIDPAdapter) checkIdentity(
	ctx context.Context,
	directory, id, issuer string,
) (*state.IdentityTrustedIDP, error) {
	values, err := a.client.ListIdentityTrustedIDPs(ctx, directory)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if value.IssuerURL != issuer {
			continue
		}
		if id == "" {
			return nil, fmt.Errorf("trusted IdP issuer already exists; regenerate the plan")
		}
		if value.ID != id {
			return nil, fmt.Errorf("trusted IdP identity changed; regenerate the plan")
		}
		current, err := a.client.GetIdentityTrustedIDP(ctx, directory, id)
		if err != nil {
			return nil, err
		}
		if current == nil || current.IssuerURL != issuer {
			return nil, fmt.Errorf("trusted IdP identity changed; regenerate the plan")
		}
		return current, nil
	}
	if id != "" {
		return nil, fmt.Errorf("trusted IdP identity changed; regenerate the plan")
	}
	return nil, nil
}

func (a *identityTrustedIDPAdapter) Create(ctx context.Context, request kkComps.CreateTrustedIdpPayload,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("trusted IdP namespace mismatch")
	}
	directory, err := identityVaultWriteDirectory(ctx, a.client, execCtx, "trusted IdP")
	if err != nil {
		return "", err
	}
	if _, err := a.checkIdentity(ctx, directory, "", request.IssuerURL); err != nil {
		return "", err
	}
	value, err := a.client.CreateIdentityTrustedIDP(ctx, directory, request)
	if err != nil {
		return "", err
	}
	return value.ID, nil
}

func (a *identityTrustedIDPAdapter) Update(ctx context.Context, id string, request kkComps.ReplaceTrustedIdpBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("trusted IdP namespace mismatch")
	}
	directory, err := identityVaultWriteDirectory(ctx, a.client, execCtx, "trusted IdP")
	if err != nil {
		return "", err
	}
	payload, err := trustedIDPPayload(execCtx.PlannedChange.Fields)
	if err != nil {
		return "", err
	}
	current, err := a.checkIdentity(ctx, directory, id, payload.IssuerURL)
	if err != nil {
		return "", err
	}
	// PUT clears omitted nullable fields and resets omitted booleans. Hydrate
	// all omitted declarations from a fresh public read before replacement.
	if !payload.DisplayNameDeclared() {
		request.DisplayName = current.DisplayName
	}
	if !payload.JwksURIDeclared() {
		request.JwksURI = current.JwksURI
	}
	if payload.Jit == nil {
		request.Jit = current.Jit
	}
	if payload.JitProvisioningEnabled == nil {
		request.JitProvisioningEnabled = current.JitProvisioningEnabled
	}
	if payload.VaultAccessEnabled == nil {
		request.VaultAccessEnabled = current.VaultAccessEnabled
	}
	if err := a.client.ReplaceIdentityTrustedIDP(ctx, directory, id, payload.IssuerURL, request); err != nil {
		return "", err
	}
	return id, nil
}

func (a *identityTrustedIDPAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	directory, err := identityVaultWriteDirectory(ctx, a.client, execCtx, "trusted IdP")
	if err != nil {
		return err
	}
	fields := execCtx.PlannedChange.Fields
	issuer, ok := fields[planner.FieldIssuerURL].(string)
	if !ok || len(fields) != 1 {
		return fmt.Errorf("trusted IdP deletion requires only issuer_url")
	}
	if err := identitytrust.ValidatePublic(issuer, nil, nil, nil); err != nil {
		return err
	}
	return a.client.DeleteIdentityTrustedIDP(ctx, directory, id, issuer)
}

func (e *Executor) registerIdentityTrustedIDPExecutor() {
	e.registerResourceExecutor(
		crudResourceExecutor(NewBaseExecutor(&identityTrustedIDPAdapter{client: e.client}, e.client, e.dryRun)),
	)
}
