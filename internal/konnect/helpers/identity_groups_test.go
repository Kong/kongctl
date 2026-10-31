package helpers_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

const (
	identityGroupOne = "731559d3-39bd-48c7-b33c-eb192847eb31"
	identityGroupTwo = "731559d3-39bd-48c7-b33c-eb192847eb32"
)

func identityGroupTestSDK(handler secretHTTPFunc) *helpers.KonnectSDK {
	return &helpers.KonnectSDK{SDK: kk.New(kk.WithClient(handler), kk.WithServerURL("https://identity.test"),
		kk.WithRetryConfig(retry.Config{Strategy: "none"}))}
}

func groupFixture(id string) map[string]any {
	return map[string]any{
		"id": id, "name": "Platform", "description": "Public description",
		"labels":     map[string]string{"team": "platform"},
		"managed_by": map[string]string{"tool": "kongctl", "kongctl-namespace": "platform"},
		"metadata":   map[string]any{"count": int64(9007199254740994)},
		"created_at": "2026-10-31T00:00:00Z", "updated_at": "2026-10-31T00:00:00Z",
	}
}

func groupMembershipFixture(id, principal string) map[string]any {
	return map[string]any{
		"id": id, "created_at": "2026-10-31T00:00:00Z",
		"member": map[string]any{"id": principal, "type": "principal", "display_name": "Alice"},
	}
}

func groupJSONResponse(t *testing.T, request *http.Request, value any) *http.Response {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return secretResponse(request, 200, string(data))
}

func groupCollection(items []any, next any) map[string]any {
	return map[string]any{"data": items, "meta": map[string]any{"page": map[string]any{"size": 1, "next": next}}}
}

func TestIdentityGroupLookupPreservesObservedFieldsWithoutDefaults(t *testing.T) {
	calls := 0
	sdk := identityGroupTestSDK(func(request *http.Request) (*http.Response, error) {
		calls++
		value := groupFixture(identityGroupOne)
		if strings.HasSuffix(request.URL.Path, "/groups") {
			return groupJSONResponse(t, request, groupCollection([]any{value}, nil)), nil
		}
		require.Equal(t, "/v2/directories/"+identityDirectory+"/groups/"+identityGroupOne, request.URL.Path)
		return groupJSONResponse(t, request, value), nil
	})
	for _, selector := range []string{"Platform", identityGroupOne} {
		value, err := helpers.ResolveIdentityGroup(t.Context(), sdk.GetIdentityGroupsReader(),
			identityDirectory, selector, 1)
		require.NoError(t, err)
		require.Equal(t, identityGroupOne, value.ID)
		require.Equal(t, "platform", value.ManagedBy["kongctl-namespace"])
		require.Equal(t, int64(9007199254740994), *value.Metadata["count"].Integer)
		require.Nil(t, value.ProvisionedBy)
		require.Nil(t, value.TeamManager)
		require.Nil(t, value.Type)
		data, err := json.Marshal(value)
		require.NoError(t, err)
		for _, absent := range []string{"provisioned_by", "team_manager", `"type":`} {
			require.NotContains(t, string(data), absent)
		}
	}
	require.Equal(t, 4, calls)
}

func TestIdentityGroupLookupRejectsAmbiguityScopeAndChangedIdentity(t *testing.T) {
	for _, mode := range []string{"duplicate-name", "absent-in-directory", "renamed", "changed-id"} {
		t.Run(mode, func(t *testing.T) {
			details := 0
			sdk := identityGroupTestSDK(func(request *http.Request) (*http.Response, error) {
				value := groupFixture(identityGroupOne)
				if strings.HasSuffix(request.URL.Path, "/groups") {
					items := []any{value}
					switch mode {
					case "duplicate-name":
						items = append(items, groupFixture(identityGroupTwo))
					case "absent-in-directory":
						items = []any{}
					}
					return groupJSONResponse(t, request, groupCollection(items, nil)), nil
				}
				details++
				if mode == "renamed" {
					value["name"] = "Renamed"
				} else {
					value["id"] = identityGroupTwo
				}
				return groupJSONResponse(t, request, value), nil
			})
			value, err := helpers.ResolveIdentityGroup(t.Context(), sdk.GetIdentityGroupsReader(),
				identityDirectory, "Platform", 1)
			require.Error(t, err)
			require.Nil(t, value)
			if mode == "duplicate-name" || mode == "absent-in-directory" {
				require.Zero(t, details)
			}
		})
	}
}

func TestIdentityGroupCollectionsRejectIncompleteOrAmbiguousResults(t *testing.T) {
	for _, membership := range []bool{false, true} {
		for _, mode := range []string{
			"complete", "empty", "failed-page", "repeated-id", "repeated-cursor", "bad-cursor",
			"missing-data", "missing-meta", "missing-next", "invalid-id", "repeated-member", "invalid-member",
		} {
			t.Run(mode+map[bool]string{false: "-groups", true: "-members"}[membership], func(t *testing.T) {
				if !membership && (mode == "repeated-member" || mode == "invalid-member") {
					t.Skip("membership-only invariant")
				}
				calls := 0
				sdk := identityGroupTestSDK(func(request *http.Request) (*http.Response, error) {
					calls++
					require.Equal(t, "1", request.URL.Query().Get("page[size]"))
					if mode == "empty" {
						return groupJSONResponse(t, request, groupCollection([]any{}, nil)), nil
					}
					if calls > 1 {
						require.Equal(t, "next-cursor", request.URL.Query().Get("page[after]"))
						if mode == "failed-page" {
							return secretResponse(request, 503, `{"message":"unavailable"}`), nil
						}
					}
					id := identityGroupOne
					if calls > 1 && mode != "repeated-id" {
						id = identityGroupTwo
					}
					if mode == "invalid-id" {
						id = "invalid"
					}
					item := groupFixture(id)
					if membership {
						member := identityPrincipal
						if calls > 1 && mode != "repeated-member" {
							member = identityID
						}
						item = groupMembershipFixture(id, member)
						if mode == "invalid-member" {
							item["member"] = map[string]string{"id": member, "type": "unexpected"}
						}
					}
					var next any
					if calls == 1 || mode == "repeated-cursor" {
						next = request.URL.Path + "?page%5Bafter%5D=next-cursor&page%5Bsize%5D=1"
					}
					if mode == "bad-cursor" {
						next = "https://untrusted.invalid/?unexpected=value"
					}
					body := groupCollection([]any{item}, next)
					switch mode {
					case "missing-data":
						delete(body, "data")
					case "missing-meta":
						delete(body, "meta")
					case "missing-next":
						body["meta"] = map[string]any{"page": map[string]any{"size": 1}}
					}
					return groupJSONResponse(t, request, body), nil
				})
				var count int
				var err error
				var empty bool
				if membership {
					values, listErr := helpers.ListIdentityGroupMemberships(t.Context(), sdk.GetIdentityGroupMembershipsReader(),
						identityDirectory, identityGroupOne, 1)
					err, count, empty = listErr, len(values), values == nil
				} else {
					values, listErr := helpers.ListIdentityGroups(t.Context(), sdk.GetIdentityGroupsReader(), identityDirectory, 1)
					err, count, empty = listErr, len(values), values == nil
				}
				if mode == "complete" || mode == "empty" {
					require.NoError(t, err)
					require.False(t, empty)
					if mode == "complete" {
						require.Equal(t, 2, count)
					} else {
						require.Zero(t, count)
					}
				} else {
					require.Error(t, err)
					require.True(t, empty, "do not expose partial observations")
				}
			})
		}
	}
}

func TestIdentityGroupMembershipDetailRequiresEntryScopeAndStableMember(t *testing.T) {
	for _, mode := range []string{"complete", "wrong-group", "changed-member", "changed-entry"} {
		t.Run(mode, func(t *testing.T) {
			details := 0
			sdk := identityGroupTestSDK(func(request *http.Request) (*http.Response, error) {
				item := groupMembershipFixture(identityID, identityPrincipal)
				if strings.HasSuffix(request.URL.Path, "/membership") {
					items := []any{item}
					if mode == "wrong-group" {
						items = []any{}
					}
					return groupJSONResponse(t, request, groupCollection(items, nil)), nil
				}
				details++
				switch mode {
				case "changed-member":
					item = groupMembershipFixture(identityID, identityGroupTwo)
				case "changed-entry":
					item["id"] = identityGroupTwo
				}
				return groupJSONResponse(t, request, item), nil
			})
			value, err := helpers.GetIdentityGroupMembership(t.Context(), sdk.GetIdentityGroupMembershipsReader(),
				identityDirectory, identityGroupOne, identityID, 1)
			if mode == "complete" {
				require.NoError(t, err)
				require.Equal(t, identityPrincipal, value.Member.ID)
			} else {
				require.Error(t, err)
				require.Nil(t, value)
				if mode == "wrong-group" {
					require.Zero(t, details)
				}
			}
		})
	}
}
