package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalAPI defines the interface for operations on Teams
type TeamAPI interface {
	ListTeams(ctx context.Context, request kkOps.ListTeamsRequest) (*kkOps.ListTeamsResponse, error)
	GetTeam(ctx context.Context, id string) (*kkOps.GetTeamResponse, error)
	CreateTeam(ctx context.Context, team *kkComps.CreateTeam) (*kkOps.CreateTeamResponse, error)
	UpdateTeam(ctx context.Context, id string,
		team *kkComps.UpdateTeam) (*kkOps.UpdateTeamResponse, error)
	DeleteTeam(ctx context.Context, id string) (*kkOps.DeleteTeamResponse, error)
}

// TeamAPIImpl provides an implementation of the TeamAPI interface
type TeamAPIImpl struct {
	SDK *kkSDK.SDK
}

func (t *TeamAPIImpl) ListTeams(
	ctx context.Context, request kkOps.ListTeamsRequest,
) (*kkOps.ListTeamsResponse, error) {
	return t.SDK.Teams.ListTeams(ctx, request)
}

func (t *TeamAPIImpl) GetTeam(ctx context.Context, id string) (*kkOps.GetTeamResponse, error) {
	return t.SDK.Teams.GetTeam(ctx, id)
}

func (t *TeamAPIImpl) CreateTeam(
	ctx context.Context,
	team *kkComps.CreateTeam,
) (*kkOps.CreateTeamResponse, error) {
	return t.SDK.Teams.CreateTeam(ctx, team)
}

func (t *TeamAPIImpl) UpdateTeam(
	ctx context.Context,
	id string,
	team *kkComps.UpdateTeam,
) (*kkOps.UpdateTeamResponse, error) {
	return t.SDK.Teams.UpdateTeam(ctx, id, team)
}

func (t *TeamAPIImpl) DeleteTeam(
	ctx context.Context,
	id string,
) (*kkOps.DeleteTeamResponse, error) {
	return t.SDK.Teams.DeleteTeam(ctx, id)
}
