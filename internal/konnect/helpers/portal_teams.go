package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalTeamAPI defines the interface for portal team operations
type PortalTeamAPI interface {
	ListPortalTeams(
		ctx context.Context,
		request kkOps.ListPortalTeamsRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListPortalTeamsResponse, error)
	GetPortalTeam(
		ctx context.Context,
		teamID string,
		portalID string,
		opts ...kkOps.Option,
	) (*kkOps.GetPortalTeamResponse, error)
}

// PortalTeamAPIImpl provides an implementation of PortalTeamAPI backed by the SDK
type PortalTeamAPIImpl struct {
	SDK *kkSDK.SDK
}

// ListPortalTeams lists developer teams for a portal
func (p *PortalTeamAPIImpl) ListPortalTeams(
	ctx context.Context, request kkOps.ListPortalTeamsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListPortalTeamsResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalTeams.ListPortalTeams(ctx, request, opts...)
}

// GetPortalTeam fetches a single developer team for a portal
func (p *PortalTeamAPIImpl) GetPortalTeam(
	ctx context.Context, teamID string, portalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalTeamResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalTeams.GetPortalTeam(ctx, teamID, portalID, opts...)
}

var _ PortalTeamAPI = (*PortalTeamAPIImpl)(nil)
