package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/identitykeys"
	"github.com/kong/kongctl/internal/util"
)

type identityVaultKeyAdapter struct{ client *state.Client }

func (a *identityVaultKeyAdapter) ResourceType() string { return planner.ResourceTypeIdentityVaultKey }

func (a *identityVaultKeyAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldPublicKey}
}
func (a *identityVaultKeyAdapter) SupportsUpdate() bool { return true }

func vaultKeyPayload(fields map[string]any) (string, identitykeys.PublicKey, error) {
	if len(fields) != 2 {
		return "", identitykeys.PublicKey{}, fmt.Errorf("vault key payload requires only name and public_key")
	}
	name, ok := fields[planner.FieldName].(string)
	if !ok {
		return "", identitykeys.PublicKey{}, fmt.Errorf("vault key name must be a string")
	}
	if err := identitykeys.ValidateName(name); err != nil {
		return "", identitykeys.PublicKey{}, err
	}
	material, ok := fields[planner.FieldPublicKey].(string)
	if !ok {
		return "", identitykeys.PublicKey{}, fmt.Errorf("vault public_key must be a string")
	}
	key, err := identitykeys.ParsePublicKey(material)
	return name, key, err
}

func (a *identityVaultKeyAdapter) MapCreateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, request *kkComps.CreateVaultKeyBody,
) error {
	name, key, err := vaultKeyPayload(fields)
	if err != nil {
		return err
	}
	request.Name, request.PublicKey = name, key.PEM
	return nil
}

func (a *identityVaultKeyAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, request *kkComps.PatchVaultKeyBody, _ map[string]string,
) error {
	name, _, err := vaultKeyPayload(fields)
	if err != nil {
		return err
	}
	request.Name = &name
	return nil
}

func (a *identityVaultKeyAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, nil
}

func vaultKeyDirectoryID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil || execCtx.PlannedChange.Parent == nil ||
		!util.IsValidUUID(execCtx.PlannedChange.Parent.ID) {
		return "", fmt.Errorf("vault key requires a resolved directory UUID")
	}
	return execCtx.PlannedChange.Parent.ID, nil
}

func (a *identityVaultKeyAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	directory, err := vaultKeyDirectoryID(execCtx)
	if err != nil {
		return nil, err
	}
	value, err := a.client.GetIdentityVaultKey(ctx, directory, id)
	if err != nil || value == nil {
		return nil, err
	}
	return value, nil
}

func (a *identityVaultKeyAdapter) writeParent(ctx context.Context, execCtx *ExecutionContext) (string, error) {
	id, err := vaultKeyDirectoryID(execCtx)
	if err != nil {
		return "", err
	}
	parent, err := a.client.GetIdentityDirectoryByID(ctx, id)
	if err != nil {
		return "", err
	}
	if parent == nil || parent.ID != id {
		return "", fmt.Errorf("vault key directory no longer exists")
	}
	if err := resources.ValidateChildOwnership("Vault key directory", parent.ManagedBy, execCtx.Namespace); err != nil {
		return "", err
	}
	if parent.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool {
		return "", fmt.Errorf("vault keys require an explicitly managed directory")
	}
	protecting := execCtx.PlannedChange.ProtectingParent
	if protecting == nil || protecting.ResourceType != planner.ResourceTypeIdentityDirectory ||
		protecting.ResourceName != parent.Name || protecting.ResourceID != "" && protecting.ResourceID != id {
		return "", fmt.Errorf("vault key directory identity changed; regenerate the plan")
	}
	if execCtx.PlannedChange.Action != planner.ActionCreate && resources.IsChildProtected(parent.ManagedBy) {
		return "", fmt.Errorf("directory %q is protected", parent.Name)
	}
	if parent.VaultEnabled == nil || !*parent.VaultEnabled {
		return "", fmt.Errorf("vault key directory Vault is disabled or unknown")
	}
	return id, nil
}

func (a *identityVaultKeyAdapter) checkIdentity(ctx context.Context, parent, id, kid string) error {
	values, err := a.client.ListIdentityVaultKeys(ctx, parent)
	if err != nil {
		return err
	}
	for _, value := range values {
		if value.Kid == kid {
			if id == "" {
				return fmt.Errorf("vault public key already exists; regenerate the plan")
			}
			if value.ID == id {
				return nil
			}
			return fmt.Errorf("vault public key identity changed; regenerate the plan")
		}
	}
	if id != "" {
		return fmt.Errorf("vault public key identity changed; regenerate the plan")
	}
	if len(values) >= 5 {
		return fmt.Errorf("directory allows five Vault keys; explicitly retire a selected key before adding material")
	}
	return nil
}

func (a *identityVaultKeyAdapter) Create(ctx context.Context, request kkComps.CreateVaultKeyBody, namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("vault key namespace mismatch")
	}
	parent, err := a.writeParent(ctx, execCtx)
	if err != nil {
		return "", err
	}
	key, err := identitykeys.ParsePublicKey(request.PublicKey)
	if err != nil {
		return "", err
	}
	if err := a.checkIdentity(ctx, parent, "", key.Kid); err != nil {
		return "", err
	}
	value, err := a.client.CreateIdentityVaultKey(ctx, parent, request)
	if err != nil {
		return "", err
	}
	return value.ID, nil
}

func (a *identityVaultKeyAdapter) Update(ctx context.Context, id string, request kkComps.PatchVaultKeyBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("vault key namespace mismatch")
	}
	parent, err := a.writeParent(ctx, execCtx)
	if err != nil {
		return "", err
	}
	_, key, err := vaultKeyPayload(execCtx.PlannedChange.Fields)
	if err != nil {
		return "", err
	}
	if err := a.checkIdentity(ctx, parent, id, key.Kid); err != nil {
		return "", err
	}
	if request.Name == nil {
		return "", fmt.Errorf("vault key update requires an explicit name")
	}
	if err := a.client.UpdateIdentityVaultKey(ctx, parent, id, key.Kid, *request.Name); err != nil {
		return "", err
	}
	return id, nil
}

func (a *identityVaultKeyAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	parent, err := a.writeParent(ctx, execCtx)
	if err != nil {
		return err
	}
	_, key, err := vaultKeyPayload(execCtx.PlannedChange.Fields)
	if err != nil {
		return err
	}
	if err := a.checkIdentity(ctx, parent, id, key.Kid); err != nil {
		return err
	}
	return a.client.DeleteIdentityVaultKey(ctx, parent, id)
}

func (e *Executor) registerIdentityVaultKeyExecutor() {
	e.registerResourceExecutor(crudResourceExecutor(NewBaseExecutor(
		&identityVaultKeyAdapter{client: e.client}, e.client, e.dryRun,
	)))
}
