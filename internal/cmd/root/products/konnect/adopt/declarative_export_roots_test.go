package adopt

import (
	"context"
	"os"
	"strings"
	"testing"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestRootAdoptionDeclarativeExports(t *testing.T) {
	cfg := stubConfig{pageSize: 100}
	cases := []struct {
		kind  resources.ResourceType
		adopt func(*testing.T, adoptCommon.AdoptRunSetup) (*adoptCommon.AdoptResult, error)
	}{
		{
			resources.ResourceTypePortal,
			func(t *testing.T, s adoptCommon.AdoptRunSetup) (*adoptCommon.AdoptResult, error) {
				api := &portalAPIStub{t: t, fetchResponse: &kkComps.PortalResponse{ID: adoptionChildID, Name: "portal"}}
				return adoptPortal(s.Helper, api, cfg, "team-a", false, adoptionChildID)
			},
		},
		{
			resources.ResourceTypeAPI,
			func(t *testing.T, s adoptCommon.AdoptRunSetup) (*adoptCommon.AdoptResult, error) {
				api := &apiAPIStub{t: t, fetchResponse: &kkComps.APIResponseSchema{ID: adoptionChildID, Name: "api"}}
				return adoptAPI(s.Helper, api, cfg, "team-a", false, adoptionChildID)
			},
		},
		{
			resources.ResourceTypeControlPlane,
			func(t *testing.T, s adoptCommon.AdoptRunSetup) (*adoptCommon.AdoptResult, error) {
				api := &controlPlaneAPIStub{t: t, control: &kkComps.ControlPlane{ID: adoptionChildID, Name: "cp"}}
				return adoptControlPlane(s.Helper, api, "team-a", false, adoptionChildID)
			},
		},
		{
			resources.ResourceTypeEventGatewayControlPlane,
			func(t *testing.T, s adoptCommon.AdoptRunSetup) (*adoptCommon.AdoptResult, error) {
				api := &egwControlPlaneAPIStub{t: t, fetchResponse: &kkComps.EventGatewayInfo{ID: adoptionChildID, Name: "egw"}}
				return adoptEventGatewayControlPlane(s.Helper, api, cfg, "team-a", false, adoptionChildID)
			},
		},
		{
			resources.ResourceTypeAIGateway,
			func(t *testing.T, s adoptCommon.AdoptRunSetup) (*adoptCommon.AdoptResult, error) {
				api := &adoptAIGatewayAPIStub{
					t: t, gateways: []kkComps.AIGateway{{ID: adoptionChildID, Name: "ai", DisplayName: "AI"}},
				}
				return adoptAIGateway(s.Helper, api, cfg, "team-a", false, adoptionChildID)
			},
		},
		{
			resources.ResourceTypeDashboard,
			func(t *testing.T, s adoptCommon.AdoptRunSetup) (*adoptCommon.AdoptResult, error) {
				api := &adoptDashboardAPIStub{t: t, dashboards: []kkComps.DashboardResponse{{
					ID: new(adoptionChildID), Name: "dashboard",
					Definition: kkComps.Dashboard{Tiles: []kkComps.Tile{}},
				}}}
				return adoptDashboard(s.Helper, api, cfg, "team-a", false, adoptionChildID)
			},
		},
		{
			resources.ResourceTypeApplicationAuthStrategy,
			func(t *testing.T, s adoptCommon.AdoptRunSetup) (*adoptCommon.AdoptResult, error) {
				api := &authStrategyAPIStub{t: t, fetchResponse: keyAuthStrategyResponse(nil)}
				return adoptAuthStrategy(s.Helper, api, cfg, "team-a", false, authStrategyTestID)
			},
		},
		{
			resources.ResourceTypeDCRProvider,
			func(t *testing.T, s adoptCommon.AdoptRunSetup) (*adoptCommon.AdoptResult, error) {
				api := &dcrProviderAPIStub{t: t, listPayloads: &helpers.DCRProviderListPayload{Total: 1, Data: []any{map[string]any{
					"id": adoptionChildID, "name": "dcr", "provider_type": "auth0", "issuer": "https://issuer.example.com",
					"dcr_config": map[string]any{"dcr_token": "never-export-this"},
				}}}}
				return adoptDCRProvider(s.Helper, api, cfg, "team-a", false, "dcr")
			},
		},
		{
			resources.ResourceTypeIdentityDirectory,
			func(_ *testing.T, s adoptCommon.AdoptRunSetup) (*adoptCommon.AdoptResult, error) {
				api := &fixture.DirectoryAPI{Current: fixture.Directory("directory", "", true)}
				return adoptIdentityDirectory(s.Helper.GetContext(), api, api.Current.ID, "team-a", false)
			},
		},
		{
			resources.ResourceTypeIdentityAuthServer,
			func(_ *testing.T, s adoptCommon.AdoptRunSetup) (*adoptCommon.AdoptResult, error) {
				api := &fixture.AuthServerAPI{Servers: []kkComps.AuthServer{fixture.AuthServer("issuer", "", true)}}
				return adoptIdentityAuthServer(s.Helper.GetContext(), api, api.Servers[0].ID, "team-a", false)
			},
		},
	}
	for _, tc := range cases {
		for _, dryRun := range []bool{true, false} {
			mode := "adopt"
			if dryRun {
				mode = "dry-run"
			}
			t.Run(string(tc.kind)+"/"+mode, func(t *testing.T) {
				setup, export := childExportTestSetup(t, dryRun, "")
				result, err := tc.adopt(t, setup)
				require.NoError(t, err)
				require.NoError(t, adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "resource"))
				content, err := os.ReadFile(export.Path)
				require.NoError(t, err)
				require.NotContains(t, string(content), "never-export-this")
				require.NotContains(t, string(content), "managed_by:")
				require.NotContains(t, string(content), "_external:")
				require.Equal(t, dryRun, len(content) > 10 && string(content[:10]) == "# DRY RUN:")
				loaded, err := loader.New().LoadFile(export.Path)
				require.NoError(t, err)
				values := loaded.AllResourcesByType(tc.kind)
				require.Len(t, values, 1)
				require.Equal(t, result.ID, values[0].GetRef())
				require.Equal(t, 1, loaded.ResourceCount())
			})
		}
	}
}

func TestRootExportFileFailureStillPrintsReceipt(t *testing.T) {
	setup, export := childExportTestSetup(t, false, "")
	api := &adoptAIGatewayAPIStub{
		t: t, gateways: []kkComps.AIGateway{{ID: adoptionChildID, Name: "ai", DisplayName: "AI"}},
	}
	result, err := adoptAIGateway(setup.Helper, api, stubConfig{pageSize: 100}, "team-a", false, adoptionChildID)
	require.NoError(t, err)
	require.NoError(t, export.File.Close())
	err = adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "AI Gateway")
	require.ErrorContains(t, err, "was adopted but declarative output")
	require.Contains(t, setup.Helper.GetStreams().Out.(*strings.Builder).String(), adoptionChildID)
}

type adoptionMembershipAPI struct {
	helpers.ControlPlaneGroupsAPI
	calls int
}

func (a *adoptionMembershipAPI) GetControlPlanesIDGroupMemberships(
	context.Context, kkOps.GetControlPlanesIDGroupMembershipsRequest, ...kkOps.Option,
) (*kkOps.GetControlPlanesIDGroupMembershipsResponse, error) {
	a.calls++
	return &kkOps.GetControlPlanesIDGroupMembershipsResponse{ListGroupMemberships: &kkComps.ListGroupMemberships{
		Data: []kkComps.ControlPlane{{ID: adoptionGatewayID}},
	}}, nil
}

func TestControlPlaneGroupExportPreservesMemberships(t *testing.T) {
	for _, dryRun := range []bool{true, false} {
		setup, export := childExportTestSetup(t, dryRun, "")
		api := &controlPlaneAPIStub{t: t, control: &kkComps.ControlPlane{
			ID: adoptionChildID, Name: "group",
			Config: kkComps.ControlPlaneConfig{ClusterType: kkComps.ControlPlaneClusterTypeClusterTypeControlPlaneGroup},
		}}
		groups := &adoptionMembershipAPI{}
		result, err := adoptControlPlane(setup.Helper, api, "team-a", false, adoptionChildID, groups)
		require.NoError(t, err)
		require.NoError(t, adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "control plane"))
		loaded, err := loader.New().LoadFile(export.Path)
		require.NoError(t, err)
		require.Len(t, loaded.ControlPlanes, 1)
		require.Equal(t, []resources.ControlPlaneGroupMember{{ID: adoptionGatewayID}}, loaded.ControlPlanes[0].Members)
		require.Equal(t, 1, groups.calls)
		if dryRun {
			require.Zero(t, api.updateCall)
		} else {
			require.Equal(t, 1, api.updateCall)
		}
	}
}
