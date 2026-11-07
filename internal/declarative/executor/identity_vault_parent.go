package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
)

func identityVaultDirectoryID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil || execCtx.PlannedChange.Parent == nil ||
		!util.IsValidUUID(execCtx.PlannedChange.Parent.ID) {
		return "", fmt.Errorf("vault resource requires a resolved directory UUID")
	}
	return execCtx.PlannedChange.Parent.ID, nil
}

func identityVaultWriteDirectory(ctx context.Context, client *state.Client, execCtx *ExecutionContext,
	resource string,
) (string, error) {
	id, err := identityVaultDirectoryID(execCtx)
	if err != nil {
		return "", err
	}
	return identityVaultWriteDirectoryByID(ctx, client, execCtx, resource, id)
}

func identityVaultWriteDirectoryByID(
	ctx context.Context, client *state.Client, execCtx *ExecutionContext, resource, id string,
) (string, error) {
	if !util.IsValidUUID(id) {
		return "", fmt.Errorf("vault resource requires a resolved directory UUID")
	}
	parent, err := client.GetIdentityDirectoryByID(ctx, id)
	if err != nil {
		return "", err
	}
	if parent == nil || parent.ID != id {
		return "", fmt.Errorf("%s directory no longer exists", resource)
	}
	if err := resources.ValidateChildOwnership(resource+" directory", parent.ManagedBy, execCtx.Namespace); err != nil {
		return "", err
	}
	if parent.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool {
		return "", fmt.Errorf("%s requires an explicitly managed directory", resource)
	}
	protecting := execCtx.PlannedChange.ProtectingParent
	if protecting == nil || protecting.ResourceType != planner.ResourceTypeIdentityDirectory ||
		protecting.ResourceName != parent.Name || protecting.ResourceID != "" && protecting.ResourceID != id {
		return "", fmt.Errorf("%s directory identity changed; regenerate the plan", resource)
	}
	if execCtx.PlannedChange.Action != planner.ActionCreate && resources.IsChildProtected(parent.ManagedBy) {
		return "", fmt.Errorf("directory %q is protected", parent.Name)
	}
	if parent.VaultEnabled == nil || !*parent.VaultEnabled {
		return "", fmt.Errorf("%s directory Vault is disabled or unknown", resource)
	}
	return id, nil
}
