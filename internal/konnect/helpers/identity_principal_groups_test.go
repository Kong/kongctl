package helpers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

const principalGroupEntry = "731559d3-39bd-48c7-b33c-eb192847eb41"

func principalGroupFixture(id string, direct bool) map[string]any {
	value := map[string]any{"id": id, "name": "Operators", "direct": direct, "membership_id": nil}
	if direct {
		value["membership_id"] = principalGroupEntry
	}
	return value
}

func TestPrincipalGroupsPreserveProjectionAndKeepCursorRequestsScoped(t *testing.T) {
	calls, host := 0, ""
	sdk := identityGroupTestSDK(func(request *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "/v2/directories/"+identityDirectory+"/principals/"+identityPrincipal+"/groups", request.URL.Path)
		require.Equal(t, "1", request.URL.Query().Get("page[size]"))
		require.False(t, request.URL.Query().Has("size"))
		if calls == 1 {
			host = request.URL.Host
			require.Empty(t, request.URL.Query().Get("page[after]"))
			return groupJSONResponse(t, request, groupCollection([]any{principalGroupFixture(identityGroupOne, true)},
				"https://elsewhere.invalid/other?page%5Bafter%5D=cursor%2Fnext&page%5Bsize%5D=99")), nil
		}
		require.Equal(t, 2, calls)
		require.Equal(t, host, request.URL.Host)
		require.Equal(t, "cursor/next", request.URL.Query().Get("page[after]"))
		value := principalGroupFixture(identityGroupTwo, false)
		value["type"], value["provisioned_by"] = "team", "imported"
		value["team_manager"] = map[string]any{
			"id": identityPrincipal, "display_name": "Manager", "email": "manager@example.test",
		}
		return groupJSONResponse(t, request, groupCollection([]any{value}, nil)), nil
	})
	values, err := helpers.ListIdentityPrincipalGroups(t.Context(), sdk.GetIdentityPrincipalGroupsReader(),
		identityDirectory, identityPrincipal, 1)
	require.NoError(t, err)
	require.Len(t, values, 2)
	require.Equal(t, 2, calls)
	require.True(t, *values[0].Direct)
	require.Equal(t, principalGroupEntry, *values[0].MembershipID)
	require.Nil(t, values[0].Type)
	require.Nil(t, values[0].ProvisionedBy, "SDK defaults must not invent provisioning")
	require.False(t, *values[1].Direct)
	require.Nil(t, values[1].MembershipID)
	require.Equal(t, "imported", string(*values[1].ProvisionedBy))
	require.Equal(t, "Manager", values[1].TeamManager.DisplayName)
	require.Equal(t, "manager@example.test", *values[1].TeamManager.Email)
	encoded, err := json.Marshal(values[1])
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"direct":false`)
	require.Contains(t, string(encoded), `"membership_id":null`)
}

func TestPrincipalGroupSelectionUsesGroupIdentityAndRejectsAmbiguity(t *testing.T) {
	for _, selector := range []string{identityGroupOne, "Operators", principalGroupEntry, "missing"} {
		calls := 0
		sdk := identityGroupTestSDK(func(request *http.Request) (*http.Response, error) {
			calls++
			return groupJSONResponse(t, request, groupCollection([]any{principalGroupFixture(identityGroupOne, true)}, nil)), nil
		})
		value, err := helpers.ResolveIdentityPrincipalGroup(t.Context(), sdk.GetIdentityPrincipalGroupsReader(),
			identityDirectory, identityPrincipal, selector, 1)
		if selector == identityGroupOne || selector == "Operators" {
			require.NoError(t, err)
			require.Equal(t, identityGroupOne, value.ID)
		} else {
			require.ErrorContains(t, err, "not found under the selected principal")
			require.Nil(t, value)
		}
		require.Equal(t, 1, calls, "there is no association detail endpoint to call")
	}
	sdk := identityGroupTestSDK(func(request *http.Request) (*http.Response, error) {
		return groupJSONResponse(t, request, groupCollection([]any{
			principalGroupFixture(identityGroupOne, true), principalGroupFixture(identityGroupTwo, false),
		}, nil)), nil
	})
	value, err := helpers.ResolveIdentityPrincipalGroup(t.Context(), sdk.GetIdentityPrincipalGroupsReader(),
		identityDirectory, identityPrincipal, "Operators", 1)
	require.ErrorContains(t, err, "multiple groups")
	require.Nil(t, value)
}

func TestPrincipalGroupsReturnNoPartialResultForIncompleteOrContradictoryPages(t *testing.T) {
	modes := []string{
		"later-page-error", "missing-data", "missing-meta", "missing-direct", "inherited-entry-id",
		"direct-without-entry", "invalid-entry-id", "duplicate-group", "repeated-cursor", "invalid-next",
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			sdk := identityGroupTestSDK(func(request *http.Request) (*http.Response, error) {
				calls++
				value := principalGroupFixture(identityGroupOne, true)
				page := groupCollection([]any{value}, nil)
				switch mode {
				case "later-page-error":
					if calls > 1 {
						return secretResponse(request, 403, `{"status":403,"title":"Forbidden"}`), nil
					}
					page = groupCollection([]any{value}, "next")
				case "missing-data":
					delete(page, "data")
				case "missing-meta":
					delete(page, "meta")
				case "missing-direct":
					delete(value, "direct")
				case "inherited-entry-id":
					value["direct"] = false
				case "direct-without-entry":
					value["membership_id"] = nil
				case "invalid-entry-id":
					value["membership_id"] = "invalid"
				case "duplicate-group":
					page = groupCollection([]any{value, value}, nil)
				case "repeated-cursor":
					if calls > 1 {
						value["id"] = identityGroupTwo
					}
					page = groupCollection([]any{value}, "next")
				case "invalid-next":
					page = groupCollection([]any{value}, "https://elsewhere.invalid/no-cursor")
				}
				return groupJSONResponse(t, request, page), nil
			})
			values, err := helpers.ListIdentityPrincipalGroups(t.Context(), sdk.GetIdentityPrincipalGroupsReader(),
				identityDirectory, identityPrincipal, 1)
			require.Error(t, err)
			require.Nil(t, values)
			require.LessOrEqual(t, calls, 2)
		})
	}
}

func TestPrincipalGroupsValidateScopeAndPreserveEmptyCollection(t *testing.T) {
	values, err := helpers.ListIdentityPrincipalGroups(t.Context(), nil, identityDirectory, identityPrincipal, 1)
	require.ErrorContains(t, err, "not configured")
	require.Nil(t, values)
	sdk := identityGroupTestSDK(func(request *http.Request) (*http.Response, error) {
		return groupJSONResponse(t, request, groupCollection([]any{}, nil)), nil
	})
	reader := sdk.GetIdentityPrincipalGroupsReader()
	values, err = helpers.ListIdentityPrincipalGroups(t.Context(), reader, "bad", identityPrincipal, 1)
	require.ErrorContains(t, err, "UUID")
	require.Nil(t, values)
	values, err = helpers.ListIdentityPrincipalGroups(t.Context(), sdk.GetIdentityPrincipalGroupsReader(),
		identityDirectory, identityPrincipal, 0)
	require.NoError(t, err)
	require.NotNil(t, values)
	require.Empty(t, values)
}
