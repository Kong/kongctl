package adopt

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestIdentityAdoptionExportsOnlySelectedChildren(t *testing.T) {
	for _, kind := range []resources.ResourceType{
		resources.ResourceTypeIdentityPrincipal, resources.ResourceTypeIdentityPrincipalIdentity,
		resources.ResourceTypeIdentityGroup, resources.ResourceTypeIdentityBasicAuth,
		resources.ResourceTypeIdentityAuthServerClient,
	} {
		t.Run(string(kind), func(t *testing.T) {
			setup, export := childExportTestSetup(t, false, "")
			ctx := setup.Helper.GetContext()
			var result *adoptCommon.AdoptResult
			var err error
			//exhaustive:ignore
			switch kind {
			case resources.ResourceTypeIdentityPrincipal:
				api := &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{adoptionGatewayID: {{
					ID: adoptionChildID, DisplayName: new("Worker"),
					Metadata: map[string]kkComps.KongIdentityMetadata{
						"exact": kkComps.CreateKongIdentityMetadataInteger(9223372036854775807),
					},
				}}}}
				result, err = adoptIdentityPrincipal(ctx, api, adoptionGatewayID, adoptionChildID, "worker-ref", "team-a", false)
			case resources.ResourceTypeIdentityPrincipalIdentity:
				current := kkComps.CreateKongPrincipalIdentityCustom(kkComps.KongPrincipalIdentityCustomResponse{
					ID: adoptionChildID, Key: "login", Value: "worker", Labels: map[string]string{"keep": "public"},
				})
				api := &fixture.PrincipalIdentitiesAPI{Parents: map[string][]kkComps.KongPrincipalIdentity{
					adoptionGatewayID + "/" + adoptionChildID: {current},
				}}
				result,
					err = adoptPrincipalIdentity(ctx,
					api,
					adoptionGatewayID,
					adoptionChildID,
					adoptionChildID,
					"login-ref",
					"team-a",
					false)
			case resources.ResourceTypeIdentityGroup:
				current := fixture.Group("Workers", "", true)
				api := &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{adoptionGatewayID: {current}}}
				result, err = adoptIdentityGroup(ctx, api, adoptionGatewayID, current.ID, "team-a", false)
			case resources.ResourceTypeIdentityBasicAuth:
				current := fixture.BasicAuth("worker@example.test", "", true)
				key := adoptionGatewayID + "/" + adoptionChildID
				api := &fixture.BasicAuthAPI{
					Parents:   map[string][]helpers.IdentityBasicAuth{key: {current}},
					Passwords: map[string][]helpers.IdentityPassword{key + "/" + current.ID: {{ID: adoptionChildID}}},
				}
				result, err = adoptIdentityBasicAuth(ctx, api, adoptionGatewayID, adoptionChildID, current.ID, "", "team-a", false)
				require.Len(t, api.Passwords[key+"/"+current.ID], 1)
			case resources.ResourceTypeIdentityAuthServerClient:
				current := kkComps.Client{
					ID: "opaque-client", Name: "worker", GrantTypes: []kkComps.GrantType{kkComps.GrantTypeClientCredentials},
					ResponseTypes:           []kkComps.ResponseType{},
					TokenEndpointAuthMethod: new(kkComps.TokenEndpointAuthMethod("client_secret_basic")),
				}
				api := &fixture.AuthServerChildrenAPI{Clients: map[string][]kkComps.Client{adoptionGatewayID: {current}}}
				result, err = adoptIdentityAuthServerClient(ctx, api, adoptionGatewayID, current.ID, false, "team-a", false)
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			data, err := os.ReadFile(export.Path)
			require.NoError(t, err)
			require.NotContains(t, string(data), "managed_by:")
			require.NotContains(t, string(data), "passwords:")
			require.NotContains(t, string(data), "client_secret:")
			loaded, err := loader.New().LoadFile(export.Path)
			require.NoError(t, err)
			children := loaded.AllResourcesByType(kind)
			require.Len(t, children, 1)
			//exhaustive:ignore
			switch kind {
			case resources.ResourceTypeIdentityPrincipal:
				require.Equal(t, "worker-ref", children[0].GetRef())
				principal := children[0].(*resources.IdentityPrincipalResource)
				require.Equal(t, json.Number("9223372036854775807"), principal.Metadata["exact"])
			case resources.ResourceTypeIdentityPrincipalIdentity:
				require.Equal(t, "login-ref", children[0].GetRef())
				require.Len(t, loaded.Identity.Principals, 1)
				require.True(t, loaded.Identity.Principals[0].IsExternal())
			}
			if strings.Contains(string(data), "KONGCTL-") {
				t.Fatalf("reserved ownership labels leaked into declarations: %s", data)
			}
		})
	}
}

func TestExportPreflightFailureRemovesEmptyOutput(t *testing.T) {
	setup, export := childExportTestSetup(t, false, "known-parent")
	err := prepareDeclarativeChild(setup.Helper.GetContext(), resources.ResourceTypeAIGatewayModel,
		adoptionChildID, adoptionChildID, "team-a", false, map[string]any{"name": "invalid"},
		adoptionParent{Kind: resources.ResourceTypeAIGateway, ID: adoptionGatewayID})
	require.Error(t, err)
	export.Close()
	_, err = os.Stat(export.Path)
	require.True(t, os.IsNotExist(err))
}

func TestExportRejectsParentChildRefCollision(t *testing.T) {
	setup, _ := childExportTestSetup(t, false, "same-ref")
	err := prepareDeclarativeChild(setup.Helper.GetContext(), resources.ResourceTypeAIGatewayConsumerGroup,
		adoptionChildID, "same-ref", "team-a", false, map[string]any{"name": "group", "display_name": "Group"},
		adoptionParent{Kind: resources.ResourceTypeAIGateway, ID: adoptionGatewayID})
	require.ErrorContains(t, err, "collides")
}
