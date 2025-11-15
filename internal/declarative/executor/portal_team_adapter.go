package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalTeamAdapter implements ResourceOperations for portal teams
type PortalTeamAdapter struct {
	client *state.Client
}

// NewPortalTeamAdapter creates a new portal team adapter
func NewPortalTeamAdapter(client *state.Client) *PortalTeamAdapter {
	return &PortalTeamAdapter{client: client}
}

// MapCreateFields maps fields to PortalCreateTeamRequest
func (p *PortalTeamAdapter) MapCreateFields(
	_ context.Context, _ *ExecutionContext, fields map[string]any,
	create *kkComps.PortalCreateTeamRequest,
) error {
	// Required fields
	name, ok := fields["name"].(string)
	if !ok {
		return fmt.Errorf("name is required")
	}
	create.Name = name

	// Optional fields
	if description, ok := fields["description"].(string); ok {
		create.Description = &description
	}

	return nil
}

// MapUpdateFields maps fields to PortalUpdateTeamRequest
func (p *PortalTeamAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	update *kkComps.PortalUpdateTeamRequest, _ map[string]string,
) error {
	// Only description can be updated (name is the identifier)
	if description, ok := fields["description"].(string); ok {
		update.Description = &description
	}

	return nil
}

// Create creates a new portal team
func (p *PortalTeamAdapter) Create(ctx context.Context, req kkComps.PortalCreateTeamRequest,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	// Get portal ID from execution context
	portalID, err := p.getPortalID(execCtx)
	if err != nil {
		return "", err
	}

	return p.client.CreatePortalTeam(ctx, portalID, req, namespace)
}

// Update updates an existing portal team
func (p *PortalTeamAdapter) Update(ctx context.Context, id string, req kkComps.PortalUpdateTeamRequest,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	// Get portal ID from execution context
	portalID, err := p.getPortalID(execCtx)
	if err != nil {
		return "", err
	}

	err = p.client.UpdatePortalTeam(ctx, portalID, id, req, namespace)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Delete deletes a portal team
func (p *PortalTeamAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	// Get portal ID from execution context
	portalID, err := p.getPortalID(execCtx)
	if err != nil {
		return err
	}

	return p.client.DeletePortalTeam(ctx, portalID, id)
}

// GetByName gets a portal team by name
func (p *PortalTeamAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	// Portal teams are looked up by the planner from the list
	// No direct "get by name" API available
	return nil, nil
}

// GetByID gets a portal team by ID
func (p *PortalTeamAdapter) GetByID(
	ctx context.Context, id string, execCtx *ExecutionContext,
) (ResourceInfo, error) {
	portalID, err := p.getPortalID(execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get portal ID for team lookup: %w", err)
	}

	// List all teams and find the one with matching ID
	teams, err := p.client.ListPortalTeams(ctx, portalID)
	if err != nil {
		return nil, fmt.Errorf("failed to list portal teams: %w", err)
	}

	for _, team := range teams {
		if team.ID == id {
			return &PortalTeamResourceInfo{team: &team}, nil
		}
	}

	return nil, nil
}

// ResourceType returns the resource type name
func (p *PortalTeamAdapter) ResourceType() string {
	return "portal_team"
}

// RequiredFields returns the required fields for creation
func (p *PortalTeamAdapter) RequiredFields() []string {
	return []string{"name"}
}

// SupportsUpdate returns true as teams support updates (description only)
func (p *PortalTeamAdapter) SupportsUpdate() bool {
	return true
}

// getPortalID extracts the portal ID from ExecutionContext
func (p *PortalTeamAdapter) getPortalID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for team operations")
	}

	change := *execCtx.PlannedChange

	// Priority 1: Check References (for Create operations)
	if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID != "" {
		return portalRef.ID, nil
	}

	// Priority 2: Check Parent field (for Delete operations)
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("portal ID is required for team operations")
}

// PortalTeamResourceInfo implements ResourceInfo for portal teams
type PortalTeamResourceInfo struct {
	team *state.PortalTeam
}

func (p *PortalTeamResourceInfo) GetID() string {
	return p.team.ID
}

func (p *PortalTeamResourceInfo) GetName() string {
	return p.team.Name
}

func (p *PortalTeamResourceInfo) GetLabels() map[string]string {
	// Portal teams don't support labels (child resources don't have labels)
	return make(map[string]string)
}

func (p *PortalTeamResourceInfo) GetNormalizedLabels() map[string]string {
	return make(map[string]string)
}
