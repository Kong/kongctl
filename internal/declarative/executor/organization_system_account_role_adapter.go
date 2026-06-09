package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// OrganizationSystemAccountRoleAdapter implements ResourceOperations for organization system account roles.
type OrganizationSystemAccountRoleAdapter struct {
	client *state.Client
}

func NewOrganizationSystemAccountRoleAdapter(client *state.Client) *OrganizationSystemAccountRoleAdapter {
	return &OrganizationSystemAccountRoleAdapter{client: client}
}

func (o *OrganizationSystemAccountRoleAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.AssignRole,
) error {
	return mapAssignRoleFields(execCtx, fields, create)
}

func (o *OrganizationSystemAccountRoleAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	_ map[string]any,
	_ *kkComps.AssignRole,
	_ map[string]string,
) error {
	return nil
}

func (o *OrganizationSystemAccountRoleAdapter) Create(
	ctx context.Context,
	req kkComps.AssignRole,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	accountID, err := o.getAccountID(execCtx)
	if err != nil {
		return "", err
	}
	return o.client.AssignOrganizationSystemAccountRole(ctx, accountID, req, namespace)
}

func (o *OrganizationSystemAccountRoleAdapter) Update(
	_ context.Context,
	_ string,
	_ kkComps.AssignRole,
	_ string,
	_ *ExecutionContext,
) (string, error) {
	return "", fmt.Errorf("organization system account roles do not support update operations")
}

func (o *OrganizationSystemAccountRoleAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	accountID, err := o.getAccountID(execCtx)
	if err != nil {
		return err
	}
	return o.client.RemoveOrganizationSystemAccountRole(ctx, accountID, id)
}

func (o *OrganizationSystemAccountRoleAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

func (o *OrganizationSystemAccountRoleAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	accountID, err := o.getAccountID(execCtx)
	if err != nil {
		return nil, err
	}
	roles, err := o.client.ListOrganizationSystemAccountRoles(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list organization system account roles: %w", err)
	}
	for _, role := range roles {
		if role.ID == id {
			return &OrganizationSystemAccountRoleResourceInfo{role: role}, nil
		}
	}
	return nil, nil
}

func (o *OrganizationSystemAccountRoleAdapter) ResourceType() string {
	return planner.ResourceTypeOrganizationSystemAccountRole
}

func (o *OrganizationSystemAccountRoleAdapter) RequiredFields() []string {
	return []string{planner.FieldRoleName, planner.FieldEntityID, planner.FieldEntityTypeName, planner.FieldEntityRegion}
}

func (o *OrganizationSystemAccountRoleAdapter) SupportsUpdate() bool {
	return false
}

func (o *OrganizationSystemAccountRoleAdapter) getAccountID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for organization system account role operations")
	}
	if accountRef, ok := execCtx.PlannedChange.References[planner.FieldSystemAccountID]; ok && accountRef.ID != "" {
		return accountRef.ID, nil
	}
	if execCtx.PlannedChange.Parent != nil && execCtx.PlannedChange.Parent.ID != "" {
		return execCtx.PlannedChange.Parent.ID, nil
	}
	return "", fmt.Errorf("system account ID is required for organization system account role operations")
}

type OrganizationSystemAccountRoleResourceInfo struct {
	role state.OrganizationSystemAccountRole
}

func (o *OrganizationSystemAccountRoleResourceInfo) GetID() string {
	return o.role.ID
}

func (o *OrganizationSystemAccountRoleResourceInfo) GetName() string {
	return fmt.Sprintf("%s:%s", o.role.RoleName, o.role.EntityID)
}

func (o *OrganizationSystemAccountRoleResourceInfo) GetLabels() map[string]string {
	return map[string]string{}
}

func (o *OrganizationSystemAccountRoleResourceInfo) GetNormalizedLabels() map[string]string {
	return map[string]string{}
}
