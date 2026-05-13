package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// OrganizationTeamAPI defines the interface for operations on Teams
type OrganizationTeamAPI interface {
	ListOrganizationTeams(ctx context.Context, request kkOps.ListTeamsRequest,
		opts ...kkOps.Option) (*kkOps.ListTeamsResponse, error)
	GetOrganizationTeam(ctx context.Context, id string,
		opts ...kkOps.Option) (*kkOps.GetTeamResponse, error)
	CreateOrganizationTeam(ctx context.Context, team *kkComps.CreateTeam,
		opts ...kkOps.Option) (*kkOps.CreateTeamResponse, error)
	UpdateOrganizationTeam(ctx context.Context, id string,
		team *kkComps.UpdateTeam, opts ...kkOps.Option) (*kkOps.UpdateTeamResponse, error)
	DeleteOrganizationTeam(ctx context.Context, id string,
		opts ...kkOps.Option) (*kkOps.DeleteTeamResponse, error)
}

// OrganizationTeamAPIImpl provides an implementation of the OrganizationTeamAPI interface
type OrganizationTeamAPIImpl struct {
	SDK *kkSDK.SDK
}

func (t *OrganizationTeamAPIImpl) ListOrganizationTeams(
	ctx context.Context, request kkOps.ListTeamsRequest, opts ...kkOps.Option,
) (*kkOps.ListTeamsResponse, error) {
	return t.SDK.Teams.ListTeams(ctx, request, opts...)
}

func (t *OrganizationTeamAPIImpl) GetOrganizationTeam(ctx context.Context, id string,
	opts ...kkOps.Option,
) (*kkOps.GetTeamResponse, error) {
	return t.SDK.Teams.GetTeam(ctx, id, opts...)
}

func (t *OrganizationTeamAPIImpl) CreateOrganizationTeam(
	ctx context.Context,
	team *kkComps.CreateTeam, opts ...kkOps.Option,
) (*kkOps.CreateTeamResponse, error) {
	return t.SDK.Teams.CreateTeam(ctx, team, opts...)
}

func (t *OrganizationTeamAPIImpl) UpdateOrganizationTeam(
	ctx context.Context,
	id string,
	team *kkComps.UpdateTeam, opts ...kkOps.Option,
) (*kkOps.UpdateTeamResponse, error) {
	return t.SDK.Teams.UpdateTeam(ctx, id, team, opts...)
}

func (t *OrganizationTeamAPIImpl) DeleteOrganizationTeam(
	ctx context.Context,
	id string, opts ...kkOps.Option,
) (*kkOps.DeleteTeamResponse, error) {
	return t.SDK.Teams.DeleteTeam(ctx, id, opts...)
}
