package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
	CreatePortalTeam(
		ctx context.Context,
		portalID string,
		portalCreateTeamRequest *kkComps.PortalCreateTeamRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreatePortalTeamResponse, error)
	UpdatePortalTeam(
		ctx context.Context,
		request kkOps.UpdatePortalTeamRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdatePortalTeamResponse, error)
	DeletePortalTeam(
		ctx context.Context,
		teamID string,
		portalID string,
		opts ...kkOps.Option,
	) (*kkOps.DeletePortalTeamResponse, error)
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

// CreatePortalTeam creates a new developer team for a portal
func (p *PortalTeamAPIImpl) CreatePortalTeam(
	ctx context.Context, portalID string, portalCreateTeamRequest *kkComps.PortalCreateTeamRequest,
	opts ...kkOps.Option,
) (*kkOps.CreatePortalTeamResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalTeams.CreatePortalTeam(ctx, portalID, portalCreateTeamRequest, opts...)
}

// UpdatePortalTeam updates a developer team for a portal
func (p *PortalTeamAPIImpl) UpdatePortalTeam(
	ctx context.Context, request kkOps.UpdatePortalTeamRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalTeamResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalTeams.UpdatePortalTeam(ctx, request, opts...)
}

// DeletePortalTeam deletes a developer team from a portal
func (p *PortalTeamAPIImpl) DeletePortalTeam(
	ctx context.Context, teamID string, portalID string,
	opts ...kkOps.Option,
) (*kkOps.DeletePortalTeamResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalTeams.DeletePortalTeam(ctx, teamID, portalID, opts...)
}

var _ PortalTeamAPI = (*PortalTeamAPIImpl)(nil)
