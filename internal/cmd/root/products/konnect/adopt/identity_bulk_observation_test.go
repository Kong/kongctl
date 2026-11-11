package adopt

import (
	"context"
	"fmt"
	"testing"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

type countedGroupAPI struct {
	*fixture.GroupAPI
	lists, gets int
}

func (a *countedGroupAPI) ListGroups(
	ctx context.Context, req kkOps.ListGroupsRequest, opts ...kkOps.Option,
) (*kkOps.ListGroupsResponse, error) {
	a.lists++
	return a.GroupAPI.ListGroups(ctx, req, opts...)
}

func (a *countedGroupAPI) GetGroup(
	ctx context.Context, directory, id string, opts ...kkOps.Option,
) (*kkOps.GetGroupResponse, error) {
	a.gets++
	return a.GroupAPI.GetGroup(ctx, directory, id, opts...)
}

type countedBasicAuthAPI struct {
	*fixture.BasicAuthAPI
	lists, gets int
}

func (a *countedBasicAuthAPI) ListPrincipalBasicAuths(ctx context.Context, req kkOps.ListPrincipalBasicAuthsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListPrincipalBasicAuthsResponse, error) {
	a.lists++
	return a.BasicAuthAPI.ListPrincipalBasicAuths(ctx, req, opts...)
}

func (a *countedBasicAuthAPI) GetPrincipalBasicAuth(ctx context.Context, req kkOps.GetPrincipalBasicAuthRequest,
	opts ...kkOps.Option,
) (*kkOps.GetPrincipalBasicAuthResponse, error) {
	a.gets++
	return a.BasicAuthAPI.GetPrincipalBasicAuth(ctx, req, opts...)
}

func TestIdentityBulkAdoptionListsEachParentOnce(t *testing.T) {
	groups := &countedGroupAPI{GroupAPI: &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{}}}
	basic := &countedBasicAuthAPI{BasicAuthAPI: &fixture.BasicAuthAPI{Parents: map[string][]helpers.IdentityBasicAuth{}}}
	parent := adoptionGatewayID + "/" + adoptionChildID
	const count = 4
	for i := range count {
		groups.Directories[adoptionGatewayID] = append(
			groups.Directories[adoptionGatewayID], fixture.Group(fmt.Sprintf("group-%d", i), "", false),
		)
		basic.Parents[parent] = append(basic.Parents[parent], fixture.BasicAuth(fmt.Sprintf("domain/user-%d", i), "", false))
	}
	setup, _ := childTestSetup(t, false, cmdCommon.JSON)
	setup.SDK = identityAdoptionSDK{groups: groups, basic: basic}
	command, err := NewIdentityGroupCmd(verbs.Adopt, nil, nil)
	require.NoError(t, err)
	require.NoError(t, command.ParseFlags([]string{"--all"}))
	require.NoError(t, runIdentityGroupAdoptions(command, nil, setup, adoptionGatewayID))
	require.Equal(t, 1, groups.lists)
	require.Equal(t, 2*count, groups.gets)
	require.Len(t, groups.Writes, count)
	command, err = NewIdentityBasicAuthCmd(verbs.Adopt, nil, nil)
	require.NoError(t, err)
	require.NoError(t, command.ParseFlags([]string{"--name-pattern", "*"}))
	require.NoError(t, runIdentityBasicAuthAdoptions(command, nil, setup, adoptionGatewayID, adoptionChildID))
	require.Equal(t, 1, basic.lists)
	require.Equal(t, 3*count, basic.gets)
	require.Len(t, basic.Writes, count)
}

func TestBulkBasicAuthRejectsDuplicateUsernamesBeforeWriting(t *testing.T) {
	first := fixture.BasicAuth("same-user", "", false)
	second := first
	second.ID = adoptionChildID
	api := &countedBasicAuthAPI{BasicAuthAPI: &fixture.BasicAuthAPI{Parents: map[string][]helpers.IdentityBasicAuth{
		adoptionGatewayID + "/" + adoptionChildID: {first, second},
	}}}
	setup, _ := childTestSetup(t, false, cmdCommon.JSON)
	setup.SDK = identityAdoptionSDK{basic: api}
	command, err := NewIdentityBasicAuthCmd(verbs.Adopt, nil, nil)
	require.NoError(t, err)
	require.NoError(t, command.ParseFlags([]string{"--all"}))
	err = runIdentityBasicAuthAdoptions(command, nil, setup, adoptionGatewayID, adoptionChildID)
	require.ErrorContains(t, err, "multiple basic-auth credentials")
	require.Equal(t, 1, api.lists)
	require.Zero(t, api.gets)
	require.Empty(t, api.Writes)
}
