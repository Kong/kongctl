package state

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/require"
)

type stubPortalAuthSettingsAPI struct {
	updatePortalID string
	updateReq      *kkComps.PortalTeamGroupMappingsUpdateRequest
}

func (s *stubPortalAuthSettingsAPI) UpdatePortalAuthenticationSettings(
	context.Context,
	string,
	*kkComps.PortalAuthenticationSettingsUpdateRequest,
	...kkOps.Option,
) (*kkOps.UpdatePortalAuthenticationSettingsResponse, error) {
	return &kkOps.UpdatePortalAuthenticationSettingsResponse{}, nil
}

func (s *stubPortalAuthSettingsAPI) GetPortalAuthenticationSettings(
	context.Context,
	string,
	...kkOps.Option,
) (*kkOps.GetPortalAuthenticationSettingsResponse, error) {
	return &kkOps.GetPortalAuthenticationSettingsResponse{
		PortalAuthenticationSettingsResponse: &kkComps.PortalAuthenticationSettingsResponse{},
	}, nil
}

func (s *stubPortalAuthSettingsAPI) ListPortalTeamGroupMappings(
	context.Context,
	kkOps.ListPortalTeamGroupMappingsRequest,
	...kkOps.Option,
) (*kkOps.ListPortalTeamGroupMappingsResponse, error) {
	return &kkOps.ListPortalTeamGroupMappingsResponse{
		PortalTeamGroupMappingResponse: &kkComps.PortalTeamGroupMappingResponse{},
	}, nil
}

func (s *stubPortalAuthSettingsAPI) UpdatePortalTeamGroupMappings(
	_ context.Context,
	portalID string,
	request *kkComps.PortalTeamGroupMappingsUpdateRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdatePortalTeamGroupMappingsResponse, error) {
	s.updatePortalID = portalID
	s.updateReq = request
	return &kkOps.UpdatePortalTeamGroupMappingsResponse{}, nil
}

func TestUpdatePortalTeamGroupMappingRequestShape(t *testing.T) {
	api := &stubPortalAuthSettingsAPI{}
	client := NewClient(ClientConfig{PortalAuthSettingsAPI: api})

	err := client.UpdatePortalTeamGroupMapping(t.Context(), "portal-id", "team-id", []string{})
	require.NoError(t, err)
	require.Equal(t, "portal-id", api.updatePortalID)
	require.NotNil(t, api.updateReq)
	require.Len(t, api.updateReq.Data, 1)
	require.Equal(t, "team-id", *api.updateReq.Data[0].TeamID)
	require.Empty(t, api.updateReq.Data[0].Groups)
}
