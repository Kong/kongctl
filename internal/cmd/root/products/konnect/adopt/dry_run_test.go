package adopt

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestRootAdoptionDryRunsNeverWrite(t *testing.T) {
	setup, output := childTestSetup(t, true, cmdCommon.JSON)
	helper := setup.Helper
	cfg := stubConfig{pageSize: 100}
	portal := &portalAPIStub{t: t, fetchResponse: &kkComps.PortalResponse{ID: adoptionChildID, Name: "portal"}}
	_, err := adoptPortal(helper, portal, cfg, "team-a", false, adoptionChildID)
	require.NoError(t, err)
	require.Zero(t, portal.updateCalls)
	api := &apiAPIStub{t: t, fetchResponse: &kkComps.APIResponseSchema{ID: adoptionChildID, Name: "api"}}
	_, err = adoptAPI(helper, api, cfg, "team-a", false, adoptionChildID)
	require.NoError(t, err)
	require.Zero(t, api.updateCalls)
	cp := &controlPlaneAPIStub{t: t, control: &kkComps.ControlPlane{ID: adoptionChildID, Name: "cp"}}
	_, err = adoptControlPlane(helper, cp, "team-a", false, adoptionChildID)
	require.NoError(t, err)
	require.Zero(t, cp.updateCall)
	egw := &egwControlPlaneAPIStub{t: t, fetchResponse: &kkComps.EventGatewayInfo{ID: adoptionChildID, Name: "egw"}}
	_, err = adoptEventGatewayControlPlane(helper, egw, cfg, "team-a", false, adoptionChildID)
	require.NoError(t, err)
	require.Zero(t, egw.updateCalls)
	ai := &adoptAIGatewayAPIStub{t: t, gateways: []kkComps.AIGateway{{ID: adoptionChildID, Name: "ai", DisplayName: "AI"}}}
	_, err = adoptAIGateway(helper, ai, cfg, "team-a", false, adoptionChildID)
	require.NoError(t, err)
	require.Empty(t, ai.updateID)
	dashboard := &adoptDashboardAPIStub{
		t: t,
		dashboards: []kkComps.DashboardResponse{{
			ID:   new(adoptionChildID),
			Name: "dashboard",
		}},
	}
	_, err = adoptDashboard(helper, dashboard, cfg, "team-a", false, adoptionChildID)
	require.NoError(t, err)
	require.Empty(t, dashboard.updateID)
	auth := &authStrategyAPIStub{t: t, fetchResponse: keyAuthStrategyResponse(nil)}
	_, err = adoptAuthStrategy(helper, auth, cfg, "team-a", false, authStrategyTestID)
	require.NoError(t, err)
	require.Zero(t, auth.updateCalls)
	dcr := &dcrProviderAPIStub{
		t: t,
		listPayloads: &helpers.DCRProviderListPayload{
			Data: []any{map[string]any{
				"id":   adoptionChildID,
				"name": "dcr",
			}},
			Total: 1,
		},
	}
	result, err := adoptDCRProvider(helper, dcr, cfg, "team-a", false, "dcr")
	require.NoError(t, err)
	require.Zero(t, dcr.updateCalls)
	require.NoError(t, adoptCommon.PrintAdoptResult(helper, setup.OutType, result, "DCR provider"))
	var preview adoptCommon.AdoptionPreview
	require.NoError(t, json.Unmarshal([]byte(output.String()), &preview))
	require.Len(t, preview.Changes, 8)
	require.NotContains(t, output.String(), "definition")
}

func TestIdentityAdoptionDryRunsNeverWrite(t *testing.T) {
	setup, output := childTestSetup(t, true, cmdCommon.JSON)
	ctx := setup.Helper.GetContext()
	directory := &fixture.DirectoryAPI{Current: fixture.Directory("directory", "", true)}
	_, err := adoptIdentityDirectory(ctx, directory, directory.Current.ID, "team-a", false)
	require.NoError(t, err)
	require.Zero(t, directory.Replaces)
	server := &fixture.AuthServerAPI{Servers: []kkComps.AuthServer{fixture.AuthServer("issuer", "", true)}}
	_, err = adoptIdentityAuthServer(ctx, server, server.Servers[0].ID, "team-a", false)
	require.NoError(t, err)
	require.Empty(t, server.Writes)
	groupValue := fixture.Group("group", "", true)
	group := &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{adoptionGatewayID: {groupValue}}}
	_, err = adoptIdentityGroup(ctx, group, adoptionGatewayID, groupValue.ID, "team-a", false)
	require.NoError(t, err)
	require.Empty(t, group.Writes)
	basicValue := fixture.BasicAuth("username", "", true)
	basic := &fixture.BasicAuthAPI{Parents: map[string][]helpers.IdentityBasicAuth{
		adoptionGatewayID + "/" + adoptionChildID: {basicValue},
	}}
	_, err = adoptIdentityBasicAuth(ctx, basic, adoptionGatewayID, adoptionChildID, basicValue.ID, "", "team-a", false)
	require.NoError(t, err)
	require.Empty(t, basic.Writes)
	method := kkComps.TokenEndpointAuthMethod("client_secret_basic")
	client := &fixture.AuthServerChildrenAPI{Clients: map[string][]kkComps.Client{adoptionGatewayID: {{
		ID:                      "opaque",
		Name:                    "client",
		TokenEndpointAuthMethod: &method,
		Labels:                  map[string]string{labels.ProtectedKey: "true"},
	}}}}
	_, err = adoptIdentityAuthServerClient(ctx, client, adoptionGatewayID, "opaque", false, "team-a", false)
	require.NoError(t, err)
	require.Empty(t, client.ClientUpdates)
	principal := &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{
		adoptionGatewayID: {{ID: adoptionChildID}},
	}}
	_, err = adoptIdentityPrincipal(ctx, principal, adoptionGatewayID, adoptionChildID, "principal-ref", "team-a", false)
	require.NoError(t, err)
	require.Empty(t, principal.Writes)
	identityValue := kkComps.CreateKongPrincipalIdentityCustom(kkComps.KongPrincipalIdentityCustomResponse{
		ID:    adoptionChildID,
		Value: "public-value",
	})
	identity := &fixture.PrincipalIdentitiesAPI{Parents: map[string][]kkComps.KongPrincipalIdentity{
		adoptionGatewayID + "/" + adoptionChildID: {identityValue},
	}}
	result, err := adoptPrincipalIdentity(ctx,
		identity,
		adoptionGatewayID,
		adoptionChildID,
		adoptionChildID,
		"identity-ref",
		"team-a",
		false)
	require.NoError(t, err)
	require.Empty(t, identity.Replaces)
	require.NoError(t, adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "identity"))
	var preview adoptCommon.AdoptionPreview
	require.NoError(t, json.Unmarshal([]byte(output.String()), &preview))
	require.Len(t, preview.Changes, 7)
	require.Equal(t, "identity-ref", preview.Changes[6].After["kongctl-ref"])
	require.NotContains(t, output.String(), "public-value")
}
