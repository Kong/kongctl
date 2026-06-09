package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// OrganizationTeamMembershipAPI defines the interface for organization team membership operations.
type OrganizationTeamMembershipAPI interface {
	ListTeamUsers(
		ctx context.Context,
		request kkOps.ListTeamUsersRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListTeamUsersResponse, error)
	ListUserTeams(
		ctx context.Context,
		request kkOps.ListUserTeamsRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListUserTeamsResponse, error)
	AddUserToTeam(
		ctx context.Context,
		teamID string,
		addUserToTeam *kkComps.AddUserToTeam,
		opts ...kkOps.Option,
	) (*kkOps.AddUserToTeamResponse, error)
	RemoveUserFromTeam(
		ctx context.Context,
		userID string,
		teamID string,
		opts ...kkOps.Option,
	) (*kkOps.RemoveUserFromTeamResponse, error)
}

// OrganizationTeamMembershipAPIImpl provides an SDK-backed implementation.
type OrganizationTeamMembershipAPIImpl struct {
	SDK *kkSDK.SDK
}

func (o *OrganizationTeamMembershipAPIImpl) ListTeamUsers(
	ctx context.Context,
	request kkOps.ListTeamUsersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListTeamUsersResponse, error) {
	return o.SDK.TeamMembership.ListTeamUsers(ctx, request, opts...)
}

func (o *OrganizationTeamMembershipAPIImpl) ListUserTeams(
	ctx context.Context,
	request kkOps.ListUserTeamsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListUserTeamsResponse, error) {
	return o.SDK.TeamMembership.ListUserTeams(ctx, request, opts...)
}

func (o *OrganizationTeamMembershipAPIImpl) AddUserToTeam(
	ctx context.Context,
	teamID string,
	addUserToTeam *kkComps.AddUserToTeam,
	opts ...kkOps.Option,
) (*kkOps.AddUserToTeamResponse, error) {
	return o.SDK.TeamMembership.AddUserToTeam(ctx, teamID, addUserToTeam, opts...)
}

func (o *OrganizationTeamMembershipAPIImpl) RemoveUserFromTeam(
	ctx context.Context,
	userID string,
	teamID string,
	opts ...kkOps.Option,
) (*kkOps.RemoveUserFromTeamResponse, error) {
	return o.SDK.TeamMembership.RemoveUserFromTeam(ctx, userID, teamID, opts...)
}

var _ OrganizationTeamMembershipAPI = (*OrganizationTeamMembershipAPIImpl)(nil)
