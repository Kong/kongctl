package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// SystemAccountTeamMembershipAPI defines system account organization team membership operations.
type SystemAccountTeamMembershipAPI interface {
	ListSystemAccountTeams(
		ctx context.Context,
		request kkOps.GetSystemAccountsAccountIDTeamsRequest,
		opts ...kkOps.Option,
	) (*kkOps.GetSystemAccountsAccountIDTeamsResponse, error)
	AddSystemAccountToTeam(
		ctx context.Context,
		teamID string,
		addSystemAccountToTeam *kkComps.AddSystemAccountToTeam,
		opts ...kkOps.Option,
	) (*kkOps.PostTeamsTeamIDSystemAccountsResponse, error)
	RemoveSystemAccountFromTeam(
		ctx context.Context,
		teamID string,
		accountID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteTeamsTeamIDSystemAccountsAccountIDResponse, error)
}

// SystemAccountTeamMembershipAPIImpl provides an SDK-backed implementation.
type SystemAccountTeamMembershipAPIImpl struct {
	SDK *kkSDK.SDK
}

func (s *SystemAccountTeamMembershipAPIImpl) ListSystemAccountTeams(
	ctx context.Context,
	request kkOps.GetSystemAccountsAccountIDTeamsRequest,
	opts ...kkOps.Option,
) (*kkOps.GetSystemAccountsAccountIDTeamsResponse, error) {
	return s.SDK.SystemAccountsTeamMembership.GetSystemAccountsAccountIDTeams(ctx, request, opts...)
}

func (s *SystemAccountTeamMembershipAPIImpl) AddSystemAccountToTeam(
	ctx context.Context,
	teamID string,
	addSystemAccountToTeam *kkComps.AddSystemAccountToTeam,
	opts ...kkOps.Option,
) (*kkOps.PostTeamsTeamIDSystemAccountsResponse, error) {
	return s.SDK.SystemAccountsTeamMembership.PostTeamsTeamIDSystemAccounts(ctx, teamID, addSystemAccountToTeam, opts...)
}

func (s *SystemAccountTeamMembershipAPIImpl) RemoveSystemAccountFromTeam(
	ctx context.Context,
	teamID string,
	accountID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteTeamsTeamIDSystemAccountsAccountIDResponse, error) {
	return s.SDK.SystemAccountsTeamMembership.DeleteTeamsTeamIDSystemAccountsAccountID(ctx, teamID, accountID, opts...)
}

var _ SystemAccountTeamMembershipAPI = (*SystemAccountTeamMembershipAPIImpl)(nil)
