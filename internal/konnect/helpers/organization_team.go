package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// OrganizationTeamAPI defines the interface for operations on Teams
type OrganizationTeamAPI interface {
	ListOrganizationTeams(ctx context.Context, request kkOps.ListTeamsRequest) (*kkOps.ListTeamsResponse, error)
	GetOrganizationTeam(ctx context.Context, id string) (*kkOps.GetTeamResponse, error)
	CreateOrganizationTeam(ctx context.Context, team *kkComps.CreateTeam) (*kkOps.CreateTeamResponse, error)
	UpdateOrganizationTeam(ctx context.Context, id string,
		team *kkComps.UpdateTeam) (*kkOps.UpdateTeamResponse, error)
	DeleteOrganizationTeam(ctx context.Context, id string) (*kkOps.DeleteTeamResponse, error)
}

// OrganizationTeamAPIImpl provides an implementation of the OrganizationTeamAPI interface
type OrganizationTeamAPIImpl struct {
	SDK *kkSDK.SDK
}

func (t *OrganizationTeamAPIImpl) ListOrganizationTeams(
	ctx context.Context, request kkOps.ListTeamsRequest,
) (*kkOps.ListTeamsResponse, error) {
	return t.SDK.Teams.ListTeams(ctx, request)
}

func (t *OrganizationTeamAPIImpl) GetOrganizationTeam(ctx context.Context, id string) (*kkOps.GetTeamResponse, error) {
	return t.SDK.Teams.GetTeam(ctx, id)
}

func (t *OrganizationTeamAPIImpl) CreateOrganizationTeam(
	ctx context.Context,
	team *kkComps.CreateTeam,
) (*kkOps.CreateTeamResponse, error) {
	return t.SDK.Teams.CreateTeam(ctx, team)
}

func (t *OrganizationTeamAPIImpl) UpdateOrganizationTeam(
	ctx context.Context,
	id string,
	team *kkComps.UpdateTeam,
) (*kkOps.UpdateTeamResponse, error) {
	return t.SDK.Teams.UpdateTeam(ctx, id, team)
}

func (t *OrganizationTeamAPIImpl) DeleteOrganizationTeam(
	ctx context.Context,
	id string,
) (*kkOps.DeleteTeamResponse, error) {
	return t.SDK.Teams.DeleteTeam(ctx, id)
}
