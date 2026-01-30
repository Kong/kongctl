package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// OrganizationTeamAPI defines the interface for operations on Teams
type OrganizationTeamAPI interface {
	ListTeams(ctx context.Context, request kkOps.ListTeamsRequest) (*kkOps.ListTeamsResponse, error)
	GetTeam(ctx context.Context, id string) (*kkOps.GetTeamResponse, error)
	CreateTeam(ctx context.Context, team *kkComps.CreateTeam) (*kkOps.CreateTeamResponse, error)
	UpdateTeam(ctx context.Context, id string,
		team *kkComps.UpdateTeam) (*kkOps.UpdateTeamResponse, error)
	DeleteTeam(ctx context.Context, id string) (*kkOps.DeleteTeamResponse, error)
}

// OrganizationTeamAPIImpl provides an implementation of the OrganizationTeamAPI interface
type OrganizationTeamAPIImpl struct {
	SDK *kkSDK.SDK
}

func (t *OrganizationTeamAPIImpl) ListTeams(
	ctx context.Context, request kkOps.ListTeamsRequest,
) (*kkOps.ListTeamsResponse, error) {
	return t.SDK.Teams.ListTeams(ctx, request)
}

func (t *OrganizationTeamAPIImpl) GetTeam(ctx context.Context, id string) (*kkOps.GetTeamResponse, error) {
	return t.SDK.Teams.GetTeam(ctx, id)
}

func (t *OrganizationTeamAPIImpl) CreateTeam(
	ctx context.Context,
	team *kkComps.CreateTeam,
) (*kkOps.CreateTeamResponse, error) {
	return t.SDK.Teams.CreateTeam(ctx, team)
}

func (t *OrganizationTeamAPIImpl) UpdateTeam(
	ctx context.Context,
	id string,
	team *kkComps.UpdateTeam,
) (*kkOps.UpdateTeamResponse, error) {
	return t.SDK.Teams.UpdateTeam(ctx, id, team)
}

func (t *OrganizationTeamAPIImpl) DeleteTeam(
	ctx context.Context,
	id string,
) (*kkOps.DeleteTeamResponse, error) {
	return t.SDK.Teams.DeleteTeam(ctx, id)
}
