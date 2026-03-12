package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// TeamMembershipAPI defines the interface for organization team membership operations
// covering both user and system-account membership, plus identity lookup helpers.
type TeamMembershipAPI interface {
	// User identity lookup
	ListUsers(
		ctx context.Context,
		request kkOps.ListUsersRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListUsersResponse, error)

	// System account identity lookup
	ListSystemAccounts(
		ctx context.Context,
		request kkOps.GetSystemAccountsRequest,
		opts ...kkOps.Option,
	) (*kkOps.GetSystemAccountsResponse, error)

	// User membership
	ListTeamUsers(
		ctx context.Context,
		request kkOps.ListTeamUsersRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListTeamUsersResponse, error)

	AddUserToTeam(
		ctx context.Context,
		teamID string,
		body *kkComps.AddUserToTeam,
		opts ...kkOps.Option,
	) (*kkOps.AddUserToTeamResponse, error)

	RemoveUserFromTeam(
		ctx context.Context,
		userID string,
		teamID string,
		opts ...kkOps.Option,
	) (*kkOps.RemoveUserFromTeamResponse, error)

	// System-account membership
	ListTeamSystemAccounts(
		ctx context.Context,
		request kkOps.GetTeamsTeamIDSystemAccountsRequest,
		opts ...kkOps.Option,
	) (*kkOps.GetTeamsTeamIDSystemAccountsResponse, error)

	AddSystemAccountToTeam(
		ctx context.Context,
		teamID string,
		body *kkComps.AddSystemAccountToTeam,
		opts ...kkOps.Option,
	) (*kkOps.PostTeamsTeamIDSystemAccountsResponse, error)

	RemoveSystemAccountFromTeam(
		ctx context.Context,
		teamID string,
		accountID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteTeamsTeamIDSystemAccountsAccountIDResponse, error)
}

// TeamMembershipAPIImpl provides an implementation backed by the SDK
type TeamMembershipAPIImpl struct {
	SDK *kkSDK.SDK
}

// ListUsers lists users in the organization, allowing filtering by id, email, or full_name
func (t *TeamMembershipAPIImpl) ListUsers(
	ctx context.Context,
	request kkOps.ListUsersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListUsersResponse, error) {
	if t.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return t.SDK.Users.ListUsers(ctx, request, opts...)
}

// ListSystemAccounts lists system accounts in the organization, allowing filtering by name
func (t *TeamMembershipAPIImpl) ListSystemAccounts(
	ctx context.Context,
	request kkOps.GetSystemAccountsRequest,
	opts ...kkOps.Option,
) (*kkOps.GetSystemAccountsResponse, error) {
	if t.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return t.SDK.SystemAccounts.GetSystemAccounts(ctx, request, opts...)
}

// ListTeamUsers lists users that belong to the given team
func (t *TeamMembershipAPIImpl) ListTeamUsers(
	ctx context.Context,
	request kkOps.ListTeamUsersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListTeamUsersResponse, error) {
	if t.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return t.SDK.TeamMembership.ListTeamUsers(ctx, request, opts...)
}

// AddUserToTeam adds a user to a team
func (t *TeamMembershipAPIImpl) AddUserToTeam(
	ctx context.Context,
	teamID string,
	body *kkComps.AddUserToTeam,
	opts ...kkOps.Option,
) (*kkOps.AddUserToTeamResponse, error) {
	if t.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return t.SDK.TeamMembership.AddUserToTeam(ctx, teamID, body, opts...)
}

// RemoveUserFromTeam removes a user from a team
func (t *TeamMembershipAPIImpl) RemoveUserFromTeam(
	ctx context.Context,
	userID string,
	teamID string,
	opts ...kkOps.Option,
) (*kkOps.RemoveUserFromTeamResponse, error) {
	if t.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return t.SDK.TeamMembership.RemoveUserFromTeam(ctx, userID, teamID, opts...)
}

// ListTeamSystemAccounts lists system accounts that belong to the given team
func (t *TeamMembershipAPIImpl) ListTeamSystemAccounts(
	ctx context.Context,
	request kkOps.GetTeamsTeamIDSystemAccountsRequest,
	opts ...kkOps.Option,
) (*kkOps.GetTeamsTeamIDSystemAccountsResponse, error) {
	if t.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return t.SDK.SystemAccountsTeamMembership.GetTeamsTeamIDSystemAccounts(ctx, request, opts...)
}

// AddSystemAccountToTeam adds a system account to a team
func (t *TeamMembershipAPIImpl) AddSystemAccountToTeam(
	ctx context.Context,
	teamID string,
	body *kkComps.AddSystemAccountToTeam,
	opts ...kkOps.Option,
) (*kkOps.PostTeamsTeamIDSystemAccountsResponse, error) {
	if t.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return t.SDK.SystemAccountsTeamMembership.PostTeamsTeamIDSystemAccounts(ctx, teamID, body, opts...)
}

// RemoveSystemAccountFromTeam removes a system account from a team
func (t *TeamMembershipAPIImpl) RemoveSystemAccountFromTeam(
	ctx context.Context,
	teamID string,
	accountID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteTeamsTeamIDSystemAccountsAccountIDResponse, error) {
	if t.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return t.SDK.SystemAccountsTeamMembership.DeleteTeamsTeamIDSystemAccountsAccountID(ctx, teamID, accountID, opts...)
}

var _ TeamMembershipAPI = (*TeamMembershipAPIImpl)(nil)
