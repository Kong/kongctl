package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalTeamRolesAPI defines the interface for portal team role operations
type PortalTeamRolesAPI interface {
	ListPortalTeamRoles(
		ctx context.Context,
		request kkOps.ListPortalTeamRolesRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListPortalTeamRolesResponse, error)
	AssignRoleToPortalTeams(
		ctx context.Context,
		request kkOps.AssignRoleToPortalTeamsRequest,
		opts ...kkOps.Option,
	) (*kkOps.AssignRoleToPortalTeamsResponse, error)
	RemoveRoleFromPortalTeam(
		ctx context.Context,
		request kkOps.RemoveRoleFromPortalTeamRequest,
		opts ...kkOps.Option,
	) (*kkOps.RemoveRoleFromPortalTeamResponse, error)
}

// PortalTeamRolesAPIImpl provides an implementation of PortalTeamRolesAPI backed by the SDK
type PortalTeamRolesAPIImpl struct {
	SDK *kkSDK.SDK
}

// ListPortalTeamRoles lists assigned roles for a portal team
func (p *PortalTeamRolesAPIImpl) ListPortalTeamRoles(
	ctx context.Context, request kkOps.ListPortalTeamRolesRequest, opts ...kkOps.Option,
) (*kkOps.ListPortalTeamRolesResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalTeamRoles.ListPortalTeamRoles(ctx, request, opts...)
}

// AssignRoleToPortalTeams assigns a role to a portal team
func (p *PortalTeamRolesAPIImpl) AssignRoleToPortalTeams(
	ctx context.Context, request kkOps.AssignRoleToPortalTeamsRequest, opts ...kkOps.Option,
) (*kkOps.AssignRoleToPortalTeamsResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalTeamRoles.AssignRoleToPortalTeams(ctx, request, opts...)
}

// RemoveRoleFromPortalTeam removes an assigned role from a portal team
func (p *PortalTeamRolesAPIImpl) RemoveRoleFromPortalTeam(
	ctx context.Context, request kkOps.RemoveRoleFromPortalTeamRequest, opts ...kkOps.Option,
) (*kkOps.RemoveRoleFromPortalTeamResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalTeamRoles.RemoveRoleFromPortalTeam(ctx, request, opts...)
}

// Ensure interface compliance
var _ PortalTeamRolesAPI = (*PortalTeamRolesAPIImpl)(nil)
