package executor

import (
	"context"
	"fmt"
	"log/slog"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/log"
)

// OrganizationTeamRoleAdapter implements ResourceOperations for organization team roles.
type OrganizationTeamRoleAdapter struct {
	client *state.Client
}

func NewOrganizationTeamRoleAdapter(client *state.Client) *OrganizationTeamRoleAdapter {
	return &OrganizationTeamRoleAdapter{client: client}
}

func (o *OrganizationTeamRoleAdapter) MapCreateFields(
	_ context.Context, execCtx *ExecutionContext, fields map[string]any, create *kkComps.AssignRole,
) error {
	roleName, ok := fields[planner.FieldRoleName].(string)
	if !ok || roleName == "" {
		return fmt.Errorf("role_name is required")
	}
	role := kkComps.RoleName(roleName)
	create.RoleName = &role

	entityID := ""
	if execCtx != nil && execCtx.PlannedChange != nil {
		if refInfo, ok := execCtx.PlannedChange.References[planner.FieldEntityID]; ok && refInfo.ID != "" &&
			refInfo.ID != "[unknown]" {
			entityID = refInfo.ID
		}
	}
	if entityID == "" {
		value, ok := fields[planner.FieldEntityID].(string)
		if ok && value != "" {
			entityID = value
		}
	}
	if entityID == "" {
		return fmt.Errorf("entity_id is required")
	}
	create.EntityID = &entityID

	entityTypeName, ok := fields[planner.FieldEntityTypeName].(string)
	if !ok || entityTypeName == "" {
		return fmt.Errorf("entity_type_name is required")
	}
	entityType := kkComps.EntityTypeName(entityTypeName)
	create.EntityTypeName = &entityType

	entityRegion, ok := fields[planner.FieldEntityRegion].(string)
	if !ok || entityRegion == "" {
		return fmt.Errorf("entity_region is required")
	}
	region := kkComps.AssignRoleEntityRegion(entityRegion)
	create.EntityRegion = &region

	return nil
}

func (o *OrganizationTeamRoleAdapter) MapUpdateFields(
	_ context.Context, _ *ExecutionContext, _ map[string]any, _ *kkComps.AssignRole, _ map[string]string,
) error {
	return nil
}

func (o *OrganizationTeamRoleAdapter) Create(
	ctx context.Context,
	req kkComps.AssignRole,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	teamID, err := o.getTeamID(execCtx)
	if err != nil {
		return "", err
	}

	if logger := organizationTeamRoleLogger(ctx); logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Assigning organization team role",
			slog.String("team_id", teamID),
			slog.String("role_name", getAssignRoleName(req)),
			slog.String("entity_id", getAssignRoleEntityID(req)),
		)
	}

	return o.client.AssignOrganizationTeamRole(ctx, teamID, req, namespace)
}

func (o *OrganizationTeamRoleAdapter) Update(
	_ context.Context, _ string, _ kkComps.AssignRole, _ string, _ *ExecutionContext,
) (string, error) {
	return "", fmt.Errorf("organization team roles do not support update operations")
}

func (o *OrganizationTeamRoleAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	teamID, err := o.getTeamID(execCtx)
	if err != nil {
		return err
	}

	return o.client.RemoveOrganizationTeamRole(ctx, teamID, id)
}

func (o *OrganizationTeamRoleAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

func (o *OrganizationTeamRoleAdapter) GetByID(
	ctx context.Context, id string, execCtx *ExecutionContext,
) (ResourceInfo, error) {
	teamID, err := o.getTeamID(execCtx)
	if err != nil {
		return nil, err
	}

	roles, err := o.client.ListOrganizationTeamRoles(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to list organization team roles: %w", err)
	}
	for _, role := range roles {
		if role.ID == id {
			return &OrganizationTeamRoleResourceInfo{role: &role}, nil
		}
	}
	return nil, nil
}

func (o *OrganizationTeamRoleAdapter) ResourceType() string {
	return planner.ResourceTypeOrganizationTeamRole
}

func (o *OrganizationTeamRoleAdapter) RequiredFields() []string {
	return []string{planner.FieldRoleName, planner.FieldEntityID, planner.FieldEntityTypeName, planner.FieldEntityRegion}
}

func (o *OrganizationTeamRoleAdapter) SupportsUpdate() bool {
	return false
}

func (o *OrganizationTeamRoleAdapter) getTeamID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for organization team role operations")
	}

	change := *execCtx.PlannedChange
	if teamRef, ok := change.References[planner.FieldTeamID]; ok && teamRef.ID != "" {
		return teamRef.ID, nil
	}
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("team ID is required for organization team role operations")
}

// OrganizationTeamRoleResourceInfo implements ResourceInfo for organization team roles.
type OrganizationTeamRoleResourceInfo struct {
	role *state.OrganizationTeamRole
}

func (o *OrganizationTeamRoleResourceInfo) GetID() string {
	return o.role.ID
}

func (o *OrganizationTeamRoleResourceInfo) GetName() string {
	return fmt.Sprintf("%s:%s", o.role.RoleName, o.role.EntityID)
}

func (o *OrganizationTeamRoleResourceInfo) GetLabels() map[string]string {
	return make(map[string]string)
}

func (o *OrganizationTeamRoleResourceInfo) GetNormalizedLabels() map[string]string {
	return make(map[string]string)
}

func getAssignRoleName(req kkComps.AssignRole) string {
	if req.RoleName == nil {
		return ""
	}
	return string(*req.RoleName)
}

func getAssignRoleEntityID(req kkComps.AssignRole) string {
	if req.EntityID == nil {
		return ""
	}
	return *req.EntityID
}

func organizationTeamRoleLogger(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return nil
	}
	logger, _ := ctx.Value(log.LoggerKey).(*slog.Logger)
	return logger
}
