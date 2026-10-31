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
	identityDirectory = "731559d3-39bd-48c7-b33c-eb192847eb20"
	identityPrincipal = "731559d3-39bd-48c7-b33c-eb192847eb21"
	identityID        = "731559d3-39bd-48c7-b33c-eb192847eb22"
)

func identityTestAPI(handler secretHTTPFunc) helpers.IdentityPrincipalIdentitiesAPI {
	sdk := kk.New(kk.WithServerURL("https://identity.test"), kk.WithClient(handler),
		kk.WithRetryConfig(retry.Config{Strategy: "none"}))
	return (&helpers.KonnectSDK{SDK: sdk}).GetIdentityPrincipalIdentitiesAPI()
}

func principalIdentityFixture(id, kind string) map[string]any {
	result := map[string]any{
		"id": id, "type": kind, "created_at": "2026-10-31T00:00:00Z", "updated_at": "2026-10-31T00:00:00Z",
		"labels": map[string]string{"team": "identity"},
		"managed_by": map[string]string{
			"tool":              "kongctl",
			"version":           "test",
			"kongctl-namespace": "test",
			"kongctl-protected": "false",
			"kongctl-ref":       kind,
		},
	}
	switch kind {
	case "oidc":
		result["issuer"], result["claim"] = "https://issuer.example.test", map[string]string{
			"name":  "sub",
			"value": "employee-123",
		}
	case "auth_server_client":
		result["auth_server_id"], result["client_id"] = identityDirectory, "opaque-client"
	case "control_plane_consumer":
		result["control_plane_id"], result["consumer_id"] = identityDirectory, identityPrincipal
		result["control_plane_type"] = "api_gateway"
		result["workspace_id"] = "731559d3-39bd-48c7-b33c-eb192847eb24"
	case "custom":
		result["key"], result["value"] = "employee", "employee-123"
	}
	return result
}

func identityTestCollection(t *testing.T, request *http.Request, data any, next *string) *http.Response {
	t.Helper()
	encoded, err := json.Marshal(
		map[string]any{"data": data, "meta": map[string]any{"page": map[string]any{"next": next, "size": 1}}},
	)
	require.NoError(t, err)
	return secretResponse(request, 200, string(encoded))
}

func TestPrincipalIdentityReaderPreservesFourVariantsAndMetadata(t *testing.T) {
	for _, kind := range []string{"oidc", "auth_server_client", "control_plane_consumer", "custom"} {
		t.Run(kind, func(t *testing.T) {
			fixture := principalIdentityFixture(identityID, kind)
			api := identityTestAPI(func(request *http.Request) (*http.Response, error) {
				require.Equal(
					t,
					"/v2/directories/"+identityDirectory+"/principals/"+identityPrincipal+"/identities",
					request.URL.Path,
				)
				return identityTestCollection(t, request, []any{fixture}, nil), nil
			})
			values, err := helpers.ListPrincipalIdentities(t.Context(), api, identityDirectory, identityPrincipal, 1)
			require.NoError(t, err)
			require.Len(t, values, 1)
			require.Equal(t, kind, values[0].Type)
			require.Equal(t, "test", values[0].ManagedBy["kongctl-namespace"])
			require.Equal(t, "identity", values[0].Labels["team"])
			encoded, err := json.Marshal(values[0])
			require.NoError(t, err)
			expected, err := json.Marshal(fixture)
			require.NoError(t, err)
			require.JSONEq(t, string(expected), string(encoded))
		})
	}
}

func TestPrincipalIdentityPaginationRejectsIncompleteObservations(t *testing.T) {
	for _, mode := range []string{
		"success", "failed-page", "repeated-id", "repeated-cursor", "missing-data", "invalid-id", "unknown-type",
	} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			api := identityTestAPI(func(request *http.Request) (*http.Response, error) {
				calls++
				if mode == "missing-data" {
					return secretResponse(request, 200, `{"meta":{"page":{"next":null,"size":1}}}`), nil
				}
				if calls > 1 && mode == "failed-page" {
					return secretResponse(request, 503, `{}`), nil
				}
				id := identityID
				if mode == "invalid-id" {
					id = "invalid"
				}
				if calls > 1 && mode != "repeated-id" {
					id = "731559d3-39bd-48c7-b33c-eb192847eb23"
				}
				kind := "custom"
				if mode == "unknown-type" {
					kind = "unknown"
				}
				var next *string
				if calls == 1 || mode == "repeated-cursor" {
					next = new(
						"/v2/directories/" + identityDirectory + "/principals/" + identityPrincipal +
							"/identities?page%5Bafter%5D=next-cursor&page%5Bsize%5D=1",
					)
				}
				if calls > 1 {
					require.Equal(t, "next-cursor", request.URL.Query().Get("page[after]"))
				}
				return identityTestCollection(t, request, []any{principalIdentityFixture(id, kind)}, next), nil
			})
			values, err := helpers.ListPrincipalIdentities(t.Context(), api, identityDirectory, identityPrincipal, 1)
			if mode == "success" {
				require.NoError(t, err)
				require.Len(t, values, 2)
			} else {
				require.Error(t, err)
				require.Nil(t, values)
			}
		})
	}
}

func TestPrincipalIdentityGetRequiresCompleteParentMembershipProof(t *testing.T) {
	for _, mode := range []string{"success", "wrong-parent", "failed-membership", "mismatched-detail"} {
		t.Run(mode, func(t *testing.T) {
			gets := 0
			api := identityTestAPI(func(request *http.Request) (*http.Response, error) {
				if strings.HasSuffix(request.URL.Path, "/identities") {
					if mode == "failed-membership" {
						return secretResponse(request, 503, `{}`), nil
					}
					data := []any{principalIdentityFixture(identityID, "custom")}
					if mode == "wrong-parent" {
						data = []any{}
					}
					return identityTestCollection(t, request, data, nil), nil
				}
				gets++
				id := identityID
				if mode == "mismatched-detail" {
					id = "731559d3-39bd-48c7-b33c-eb192847eb23"
				}
				encoded, err := json.Marshal(principalIdentityFixture(id, "custom"))
				require.NoError(t, err)
				return secretResponse(request, 200, string(encoded)), nil
			})
			value, err := helpers.GetPrincipalIdentity(
				t.Context(),
				api,
				identityDirectory,
				identityPrincipal,
				identityID,
				1,
			)
			if mode == "success" {
				require.NoError(t, err)
				require.Equal(t, identityID, value.ID)
			} else {
				require.Error(t, err)
				require.Nil(t, value)
			}
			if mode == "wrong-parent" || mode == "failed-membership" {
				require.Zero(t, gets, "a misleading detail route must not bypass failed membership proof")
			}
		})
	}
	_, err := helpers.GetPrincipalIdentity(t.Context(), nil, identityDirectory, identityPrincipal, "display-name", 100)
	require.ErrorContains(t, err, "UUID")
}
