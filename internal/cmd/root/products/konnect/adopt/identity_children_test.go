package adopt

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

type identityAdoptionSDK struct {
	helpers.SDKAPI
	groups  helpers.IdentityGroupsAPI
	clients helpers.IdentityAuthServerClientsAPI
	basic   helpers.IdentityBasicAuthAPI
}

func (s identityAdoptionSDK) GetIdentityGroupsAPI() helpers.IdentityGroupsAPI { return s.groups }
func (s identityAdoptionSDK) GetIdentityAuthServerClientsAPI() helpers.IdentityAuthServerClientsAPI {
	return s.clients
}

func (s identityAdoptionSDK) GetIdentityBasicAuthAPI() helpers.IdentityBasicAuthAPI { return s.basic }

func TestIdentityGroupBulkAdoptionPreflight(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		first,
			second,
			outside := fixture.Group("team-first",
			"",
			true),
			fixture.Group("team-second",
				"",
				false),
			fixture.Group("outside",
				"foreign",
				false)
		if conflict {
			second.ManagedBy[resources.ChildNamespaceKey] = "foreign"
		}
		api := &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{
			adoptionGatewayID: {
				first,
				second,
				outside,
			},
			adoptionChildID: {outside},
		}}
		setup, out := childTestSetup(t, false, cmdCommon.JSON)
		setup.SDK = identityAdoptionSDK{groups: api}
		command, err := NewIdentityGroupCmd(verbs.Adopt, nil, nil)
		require.NoError(t, err)
		require.NoError(t, command.ParseFlags([]string{"--name-pattern", "team-*"}))
		err = runIdentityGroupAdoptions(command, nil, setup, adoptionGatewayID)
		if conflict {
			require.ErrorContains(t, err, "overwrite-namespace")
			require.Empty(t, api.Writes)
			require.Empty(t, out.String())
		} else {
			require.NoError(t, err)
			require.Len(t, api.Writes, 2)
			require.Equal(t, "foreign", api.Directories[adoptionGatewayID][2].ManagedBy[resources.ChildNamespaceKey])
			require.Equal(t, []helpers.IdentityGroup{outside}, api.Directories[adoptionChildID])
			require.Zero(t, api.MembershipCreates)
			require.Zero(t, api.MembershipDeletes)
			require.Zero(t, api.Deletes)
			require.Zero(t, api.Creates)
		}
	}
}

func TestIdentityClientAndUsernameBulkAdoption(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		method := kkComps.TokenEndpointAuthMethod("client_secret_basic")
		clients := &fixture.AuthServerChildrenAPI{Clients: map[string][]kkComps.Client{adoptionGatewayID: {
			{
				ID:                      "first",
				Name:                    "team-first",
				TokenEndpointAuthMethod: &method,
			},
			{
				ID:                      "second",
				Name:                    "team-second",
				TokenEndpointAuthMethod: &method,
			},
		}}}
		first, second := fixture.BasicAuth("team-first", "", true), fixture.BasicAuth("team-second", "", false)
		key := adoptionGatewayID + "/" + adoptionChildID
		basic := &fixture.BasicAuthAPI{
			Parents: map[string][]helpers.IdentityBasicAuth{key: {
				first,
				second,
			}},
			Passwords: map[string][]helpers.IdentityPassword{key + "/" + first.ID: {{ID: adoptionChildID}}},
		}
		setup, _ := childTestSetup(t, dryRun, cmdCommon.JSON)
		setup.SDK = identityAdoptionSDK{clients: clients, basic: basic}
		command, err := NewIdentityAuthServerClientCmd(verbs.Adopt, nil, nil)
		require.NoError(t, err)
		require.NoError(t, command.ParseFlags([]string{"--all"}))
		require.NoError(t, runIdentityClientAdoptions(command, nil, setup, adoptionGatewayID))
		command, err = NewIdentityBasicAuthCmd(verbs.Adopt, nil, nil)
		require.NoError(t, err)
		require.NoError(t, command.ParseFlags([]string{"--name-pattern", "team-*"}))
		require.NoError(t, runIdentityBasicAuthAdoptions(command, nil, setup, adoptionGatewayID, adoptionChildID))
		if dryRun {
			require.Empty(t, clients.ClientUpdates)
			require.Empty(t, basic.Writes)
		} else {
			require.Len(t, clients.ClientUpdates, 2)
			require.Len(t, basic.Writes, 2)
			for _, request := range clients.ClientUpdates {
				require.Equal(t, &method, request.UpdateClient.TokenEndpointAuthMethod)
				require.Nil(t, request.UpdateClient.ClientSecret)
			}
		}
		require.Len(t, basic.Passwords[key+"/"+first.ID], 1)
	}
}
