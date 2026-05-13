package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// OrganizationUserRoleAdapter implements ResourceOperations for organization user roles.
type OrganizationUserRoleAdapter struct {
	client *state.Client
}

func NewOrganizationUserRoleAdapter(client *state.Client) *OrganizationUserRoleAdapter {
	return &OrganizationUserRoleAdapter{client: client}
}

func (o *OrganizationUserRoleAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.AssignRole,
) error {
	return mapAssignRoleFields(execCtx, fields, create)
}

func (o *OrganizationUserRoleAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	_ map[string]any,
	_ *kkComps.AssignRole,
	_ map[string]string,
) error {
	return nil
}

func (o *OrganizationUserRoleAdapter) Create(
	ctx context.Context,
	req kkComps.AssignRole,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	userID, err := o.getUserID(execCtx)
	if err != nil {
		return "", err
	}
	return o.client.AssignOrganizationUserRole(ctx, userID, req, namespace)
}

func (o *OrganizationUserRoleAdapter) Update(
	_ context.Context,
	_ string,
	_ kkComps.AssignRole,
	_ string,
	_ *ExecutionContext,
) (string, error) {
	return "", fmt.Errorf("organization user roles do not support update operations")
}

func (o *OrganizationUserRoleAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	userID, err := o.getUserID(execCtx)
	if err != nil {
		return err
	}
	return o.client.RemoveOrganizationUserRole(ctx, userID, id)
}

func (o *OrganizationUserRoleAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

func (o *OrganizationUserRoleAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	userID, err := o.getUserID(execCtx)
	if err != nil {
		return nil, err
	}
	roles, err := o.client.ListOrganizationUserRoles(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list organization user roles: %w", err)
	}
	for _, role := range roles {
		if role.ID == id {
			return &OrganizationUserRoleResourceInfo{role: role}, nil
		}
	}
	return nil, nil
}

func (o *OrganizationUserRoleAdapter) ResourceType() string {
	return planner.ResourceTypeOrganizationUserRole
}

func (o *OrganizationUserRoleAdapter) RequiredFields() []string {
	return []string{
		planner.FieldRoleName,
		planner.FieldEntityID,
		planner.FieldEntityTypeName,
		planner.FieldEntityRegion,
	}
}

func (o *OrganizationUserRoleAdapter) SupportsUpdate() bool {
	return false
}

func (o *OrganizationUserRoleAdapter) getUserID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for organization user role operations")
	}
	if userRef, ok := execCtx.PlannedChange.References[planner.FieldUserID]; ok && userRef.ID != "" {
		return userRef.ID, nil
	}
	if execCtx.PlannedChange.Parent != nil && execCtx.PlannedChange.Parent.ID != "" {
		return execCtx.PlannedChange.Parent.ID, nil
	}
	return "", fmt.Errorf("user ID is required for organization user role operations")
}

type OrganizationUserRoleResourceInfo struct {
	role state.OrganizationUserRole
}

func (o *OrganizationUserRoleResourceInfo) GetID() string {
	return o.role.ID
}

func (o *OrganizationUserRoleResourceInfo) GetName() string {
	return fmt.Sprintf("%s:%s", o.role.RoleName, o.role.EntityID)
}

func (o *OrganizationUserRoleResourceInfo) GetLabels() map[string]string {
	return make(map[string]string)
}

func (o *OrganizationUserRoleResourceInfo) GetNormalizedLabels() map[string]string {
	return make(map[string]string)
}
