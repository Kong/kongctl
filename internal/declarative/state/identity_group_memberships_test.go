package state

import (
	"context"
	"net/http"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

type disappearingMembershipGroup struct {
	*fixture.GroupAPI
	disappear bool
}

func (a *disappearingMembershipGroup) ListGroupMembership(_ context.Context, request kkOps.ListGroupMembershipRequest,
	_ ...kkOps.Option,
) (*kkOps.ListGroupMembershipResponse, error) {
	if a.disappear {
		delete(a.Directories, request.DirectoryID)
	}
	return nil, &sdkerrors.SDKError{Message: "not found", StatusCode: http.StatusNotFound}
}

func TestMembership404RequiresFreshProofThatGroupDisappeared(t *testing.T) {
	const directoryID = "4078f559-063b-4f10-9098-9e6553b21d9b"
	const principalID = "5078f559-063b-4f10-9098-9e6553b21d9b"
	for _, disappears := range []bool{true, false} {
		group := fixture.Group("Operators", "group-team", false)
		api := &disappearingMembershipGroup{GroupAPI: &fixture.GroupAPI{
			Directories: map[string][]helpers.IdentityGroup{directoryID: {group}},
		}, disappear: disappears}
		client := NewClient(ClientConfig{IdentityGroupsAPI: api})
		err := client.VerifyPrincipalGroupMembershipsEmpty(t.Context(), directoryID, principalID)
		if disappears {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
		value, err := client.GetIdentityGroupMembership(
			t.Context(),
			directoryID,
			group.ID,
			"6078f559-063b-4f10-9098-9e6553b21d9b",
		)
		if disappears {
			require.NoError(t, err)
			require.Nil(t, value)
		} else {
			require.Error(t, err)
		}
	}
}

type changedMembershipResponse struct{ *fixture.GroupAPI }

type missingGroupDirectory struct{ *fixture.GroupAPI }

func (a missingGroupDirectory) ListGroups(
	context.Context,
	kkOps.ListGroupsRequest,
	...kkOps.Option,
) (*kkOps.ListGroupsResponse, error) {
	return nil, &sdkerrors.SDKError{Message: "not found", StatusCode: http.StatusNotFound}
}

type missingDirectoryAPI struct {
	*fixture.DirectoryAPI
	exists bool
}

func (a missingDirectoryAPI) GetDirectory(
	_ context.Context,
	id string,
	_ ...kkOps.Option,
) (*kkOps.GetDirectoryResponse, error) {
	if !a.exists {
		return nil, &sdkerrors.SDKError{Message: "not found", StatusCode: http.StatusNotFound}
	}
	value := fixture.Directory("people", "directory-team", false)
	value.ID = id
	return &kkOps.GetDirectoryResponse{KongDirectory: &value}, nil
}

func TestMissingGroupCollectionRequiresFreshDirectoryAbsence(t *testing.T) {
	const directoryID = "4078f559-063b-4f10-9098-9e6553b21d9b"
	for _, exists := range []bool{true, false} {
		client := NewClient(ClientConfig{
			IdentityGroupsAPI:      &missingGroupDirectory{&fixture.GroupAPI{}},
			IdentityDirectoriesAPI: &missingDirectoryAPI{DirectoryAPI: &fixture.DirectoryAPI{}, exists: exists},
		})
		value, err := client.GetIdentityGroup(t.Context(), directoryID, "5078f559-063b-4f10-9098-9e6553b21d9b")
		require.Nil(t, value)
		if exists {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
}

func (a changedMembershipResponse) CreateGroupMembershipEntry(
	ctx context.Context,
	request kkOps.CreateGroupMembershipEntryRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateGroupMembershipEntryResponse, error) {
	response, err := a.GroupAPI.CreateGroupMembershipEntry(ctx, request, opts...)
	if err == nil {
		response.KongGroupMembershipEntry.Member.ID = "7078f559-063b-4f10-9098-9e6553b21d9b"
	}
	return response, err
}

func TestMembershipCreateReportsChangedRemoteIdentityWithoutRollback(t *testing.T) {
	const directoryID = "4078f559-063b-4f10-9098-9e6553b21d9b"
	const principalID = "5078f559-063b-4f10-9098-9e6553b21d9b"
	api := &changedMembershipResponse{&fixture.GroupAPI{}}
	client := NewClient(ClientConfig{IdentityGroupsAPI: api, IdentityPrincipalsAPI: &fixture.PrincipalAPI{
		Directories: map[string][]kkComps.KongPrincipal{directoryID: {{ID: principalID}}},
	}})
	_, err := client.CreateIdentityGroupMembership(
		t.Context(),
		directoryID,
		"6078f559-063b-4f10-9098-9e6553b21d9b",
		kkComps.CreateGroupMembershipEntryBody{
			Member: kkComps.KongGroupMemberInput{Type: kkComps.KongGroupMembershipTypePrincipal, ID: principalID},
		},
	)
	require.ErrorContains(t, err, "does not match")
	require.Equal(t, 1, api.MembershipCreates)
	require.Zero(t, api.MembershipDeletes)
}
