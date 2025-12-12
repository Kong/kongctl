package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalTeamMembershipAPI defines the interface for developer team membership operations
type PortalTeamMembershipAPI interface {
	ListPortalTeamDevelopers(
		ctx context.Context,
		request kkOps.ListPortalTeamDevelopersRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListPortalTeamDevelopersResponse, error)
}

// PortalTeamMembershipAPIImpl provides an implementation backed by the SDK
type PortalTeamMembershipAPIImpl struct {
	SDK *kkSDK.SDK
}

// ListPortalTeamDevelopers lists developers for a specific team
func (p *PortalTeamMembershipAPIImpl) ListPortalTeamDevelopers(
	ctx context.Context,
	request kkOps.ListPortalTeamDevelopersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListPortalTeamDevelopersResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	return p.SDK.PortalTeamMembership.ListPortalTeamDevelopers(ctx, request, opts...)
}

var _ PortalTeamMembershipAPI = (*PortalTeamMembershipAPIImpl)(nil)
