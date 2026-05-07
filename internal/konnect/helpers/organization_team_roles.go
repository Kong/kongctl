package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// OrganizationTeamRolesAPI defines the interface for organization team role operations.
type OrganizationTeamRolesAPI interface {
	ListTeamRoles(
		ctx context.Context,
		teamID string,
		filter *kkOps.ListTeamRolesQueryParamFilter,
		opts ...kkOps.Option,
	) (*kkOps.ListTeamRolesResponse, error)
	TeamsAssignRole(
		ctx context.Context,
		teamID string,
		assignRole *kkComps.AssignRole,
		opts ...kkOps.Option,
	) (*kkOps.TeamsAssignRoleResponse, error)
	TeamsRemoveRole(
		ctx context.Context,
		teamID string,
		roleID string,
		opts ...kkOps.Option,
	) (*kkOps.TeamsRemoveRoleResponse, error)
}

// OrganizationTeamRolesAPIImpl provides an SDK-backed implementation of OrganizationTeamRolesAPI.
type OrganizationTeamRolesAPIImpl struct {
	SDK *kkSDK.SDK
}

func (o *OrganizationTeamRolesAPIImpl) ListTeamRoles(
	ctx context.Context,
	teamID string,
	filter *kkOps.ListTeamRolesQueryParamFilter,
	opts ...kkOps.Option,
) (*kkOps.ListTeamRolesResponse, error) {
	return o.SDK.Roles.ListTeamRoles(ctx, teamID, filter, opts...)
}

func (o *OrganizationTeamRolesAPIImpl) TeamsAssignRole(
	ctx context.Context,
	teamID string,
	assignRole *kkComps.AssignRole,
	opts ...kkOps.Option,
) (*kkOps.TeamsAssignRoleResponse, error) {
	return o.SDK.Roles.TeamsAssignRole(ctx, teamID, assignRole, opts...)
}

func (o *OrganizationTeamRolesAPIImpl) TeamsRemoveRole(
	ctx context.Context,
	teamID string,
	roleID string,
	opts ...kkOps.Option,
) (*kkOps.TeamsRemoveRoleResponse, error) {
	return o.SDK.Roles.TeamsRemoveRole(ctx, teamID, roleID, opts...)
}

var _ OrganizationTeamRolesAPI = (*OrganizationTeamRolesAPIImpl)(nil)
