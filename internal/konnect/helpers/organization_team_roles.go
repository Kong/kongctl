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
	ListUserRoles(
		ctx context.Context,
		userID string,
		filter *kkOps.ListUserRolesQueryParamFilter,
		opts ...kkOps.Option,
	) (*kkOps.ListUserRolesResponse, error)
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
	UsersAssignRole(
		ctx context.Context,
		userID string,
		assignRole *kkComps.AssignRole,
		opts ...kkOps.Option,
	) (*kkOps.UsersAssignRoleResponse, error)
	UsersRemoveRole(
		ctx context.Context,
		userID string,
		roleID string,
		opts ...kkOps.Option,
	) (*kkOps.UsersRemoveRoleResponse, error)
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

func (o *OrganizationTeamRolesAPIImpl) ListUserRoles(
	ctx context.Context,
	userID string,
	filter *kkOps.ListUserRolesQueryParamFilter,
	opts ...kkOps.Option,
) (*kkOps.ListUserRolesResponse, error) {
	return o.SDK.Roles.ListUserRoles(ctx, userID, filter, opts...)
}

func (o *OrganizationTeamRolesAPIImpl) TeamsRemoveRole(
	ctx context.Context,
	teamID string,
	roleID string,
	opts ...kkOps.Option,
) (*kkOps.TeamsRemoveRoleResponse, error) {
	return o.SDK.Roles.TeamsRemoveRole(ctx, teamID, roleID, opts...)
}

func (o *OrganizationTeamRolesAPIImpl) UsersAssignRole(
	ctx context.Context,
	userID string,
	assignRole *kkComps.AssignRole,
	opts ...kkOps.Option,
) (*kkOps.UsersAssignRoleResponse, error) {
	return o.SDK.Roles.UsersAssignRole(ctx, userID, assignRole, opts...)
}

func (o *OrganizationTeamRolesAPIImpl) UsersRemoveRole(
	ctx context.Context,
	userID string,
	roleID string,
	opts ...kkOps.Option,
) (*kkOps.UsersRemoveRoleResponse, error) {
	return o.SDK.Roles.UsersRemoveRole(ctx, userID, roleID, opts...)
}

var _ OrganizationTeamRolesAPI = (*OrganizationTeamRolesAPIImpl)(nil)
