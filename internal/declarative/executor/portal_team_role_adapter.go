package executor

import (
	"context"
	"fmt"
	"log/slog"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/log"
)

// PortalTeamRoleAdapter implements ResourceOperations for portal team roles
type PortalTeamRoleAdapter struct {
	client *state.Client
}

// NewPortalTeamRoleAdapter creates a new portal team role adapter
func NewPortalTeamRoleAdapter(client *state.Client) *PortalTeamRoleAdapter {
	return &PortalTeamRoleAdapter{client: client}
}

// MapCreateFields maps fields to PortalAssignRoleRequest
func (p *PortalTeamRoleAdapter) MapCreateFields(
	_ context.Context, execCtx *ExecutionContext, fields map[string]any, create *kkComps.PortalAssignRoleRequest,
) error {
	roleName, ok := fields["role_name"].(string)
	if !ok || roleName == "" {
		return fmt.Errorf("role_name is required")
	}
	create.RoleName = &roleName

	entityID := ""
	if execCtx != nil && execCtx.PlannedChange != nil {
		if refInfo, ok := execCtx.PlannedChange.References["entity_id"]; ok && refInfo.ID != "" &&
			refInfo.ID != "[unknown]" {
			entityID = refInfo.ID
		}
	}
	if entityID == "" {
		value, ok := fields["entity_id"].(string)
		if ok && value != "" {
			entityID = value
		}
	}
	if entityID == "" {
		return fmt.Errorf("entity_id is required")
	}
	create.EntityID = &entityID

	entityTypeName, ok := fields["entity_type_name"].(string)
	if !ok || entityTypeName == "" {
		return fmt.Errorf("entity_type_name is required")
	}
	create.EntityTypeName = &entityTypeName

	entityRegion, ok := fields["entity_region"].(string)
	if !ok || entityRegion == "" {
		return fmt.Errorf("entity_region is required")
	}
	region := kkComps.PortalAssignRoleRequestEntityRegion(entityRegion)
	create.EntityRegion = &region

	return nil
}

// MapUpdateFields is not supported for portal team roles
func (p *PortalTeamRoleAdapter) MapUpdateFields(
	_ context.Context, _ *ExecutionContext, _ map[string]any, _ *kkComps.PortalAssignRoleRequest, _ map[string]string,
) error {
	return nil
}

// Create assigns a role to a portal team
func (p *PortalTeamRoleAdapter) Create(
	ctx context.Context,
	req kkComps.PortalAssignRoleRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	portalID, teamID, err := p.getPortalAndTeamIDs(execCtx)
	if err != nil {
		return "", err
	}

	logger := portalTeamRoleLogger(ctx)
	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Assigning portal team role",
			slog.String("portal_id", portalID),
			slog.String("team_id", teamID),
			slog.String("role_name", ptrToString(req.RoleName)),
			slog.String("entity_id", ptrToString(req.EntityID)),
			slog.String("entity_type_name", ptrToString(req.EntityTypeName)),
			slog.String("entity_region", string(getAssignEntityRegion(req.EntityRegion))),
		)
	}

	id, err := p.client.AssignPortalTeamRole(ctx, portalID, teamID, req, namespace)
	if err != nil {
		return "", err
	}

	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Assigned portal team role",
			slog.String("portal_id", portalID),
			slog.String("team_id", teamID),
			slog.String("role_assignment_id", id))
	}

	return id, nil
}

// Update is not supported for portal team roles
func (p *PortalTeamRoleAdapter) Update(
	_ context.Context, _ string, _ kkComps.PortalAssignRoleRequest, _ string, _ *ExecutionContext,
) (string, error) {
	return "", fmt.Errorf("portal team roles do not support update operations")
}

// Delete removes an assigned role from a portal team
func (p *PortalTeamRoleAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	portalID, teamID, err := p.getPortalAndTeamIDs(execCtx)
	if err != nil {
		return err
	}

	logger := portalTeamRoleLogger(ctx)
	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Removing portal team role",
			slog.String("portal_id", portalID),
			slog.String("team_id", teamID),
			slog.String("role_assignment_id", id))
	}

	if err := p.client.RemovePortalTeamRole(ctx, portalID, teamID, id); err != nil {
		return err
	}

	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Removed portal team role",
			slog.String("portal_id", portalID),
			slog.String("team_id", teamID),
			slog.String("role_assignment_id", id))
	}

	return nil
}

// GetByName is not supported for portal team roles
func (p *PortalTeamRoleAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

// GetByID fetches a role assignment by ID
func (p *PortalTeamRoleAdapter) GetByID(
	ctx context.Context, id string, execCtx *ExecutionContext,
) (ResourceInfo, error) {
	portalID, teamID, err := p.getPortalAndTeamIDs(execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get portal/team for role lookup: %w", err)
	}

	roles, err := p.client.ListPortalTeamRoles(ctx, portalID, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to list portal team roles: %w", err)
	}

	for _, role := range roles {
		if role.ID == id {
			return &PortalTeamRoleResourceInfo{role: &role}, nil
		}
	}

	return nil, nil
}

// ResourceType returns the resource type name
func (p *PortalTeamRoleAdapter) ResourceType() string {
	return "portal_team_role"
}

// RequiredFields returns the required fields for creation
func (p *PortalTeamRoleAdapter) RequiredFields() []string {
	return []string{"role_name", "entity_id", "entity_type_name", "entity_region"}
}

// SupportsUpdate indicates update is not supported
func (p *PortalTeamRoleAdapter) SupportsUpdate() bool {
	return false
}

// getPortalAndTeamIDs extracts portal and team IDs from ExecutionContext
func (p *PortalTeamRoleAdapter) getPortalAndTeamIDs(execCtx *ExecutionContext) (string, string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", "", fmt.Errorf("execution context is required for portal team role operations")
	}

	change := *execCtx.PlannedChange

	portalID := ""
	if portalRef, ok := change.References["portal_id"]; ok {
		portalID = portalRef.ID
	}
	if portalID == "" && change.Parent != nil {
		portalID = change.Parent.ID
	}

	teamID := ""
	if teamRef, ok := change.References["team_id"]; ok {
		teamID = teamRef.ID
	}
	if teamID == "" && change.Parent != nil {
		teamID = change.Parent.ID
	}

	if portalID == "" || teamID == "" {
		return "", "", fmt.Errorf("portal ID and team ID are required for portal team role operations")
	}

	return portalID, teamID, nil
}

// PortalTeamRoleResourceInfo implements ResourceInfo for portal team roles
type PortalTeamRoleResourceInfo struct {
	role *state.PortalTeamRole
}

func (p *PortalTeamRoleResourceInfo) GetID() string {
	return p.role.ID
}

func (p *PortalTeamRoleResourceInfo) GetName() string {
	return fmt.Sprintf("%s:%s", p.role.RoleName, p.role.EntityID)
}

func (p *PortalTeamRoleResourceInfo) GetLabels() map[string]string {
	return make(map[string]string)
}

func (p *PortalTeamRoleResourceInfo) GetNormalizedLabels() map[string]string {
	return make(map[string]string)
}

func portalTeamRoleLogger(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return nil
	}
	logger, _ := ctx.Value(log.LoggerKey).(*slog.Logger)
	return logger
}

func getAssignEntityRegion(
	value *kkComps.PortalAssignRoleRequestEntityRegion,
) kkComps.PortalAssignRoleRequestEntityRegion {
	if value == nil {
		return ""
	}
	return *value
}

func ptrToString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
