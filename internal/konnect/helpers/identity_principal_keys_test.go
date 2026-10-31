package helpers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func principalKeyTestAPI(handler secretHTTPFunc) helpers.IdentityPrincipalKeysReader {
	sdk := kk.New(kk.WithServerURL("https://identity.test"), kk.WithClient(handler),
		kk.WithRetryConfig(retry.Config{Strategy: "none"}))
	return (&helpers.KonnectSDK{SDK: sdk}).GetIdentityPrincipalKeysReader()
}

func principalKeyFixture(id string) map[string]any {
	return map[string]any{
		"id": id, "created_at": "2026-10-31T00:00:00Z", "updated_at": "2026-10-31T00:00:00Z",
		"expires_at": "2026-11-03T00:00:00Z", "secret": "unexpected-read-material",
		"managed_by": map[string]string{"tool": "kongctl", "kongctl-namespace": "key-team", "kongctl-ref": "primary"},
	}
}

func TestPrincipalAPIKeyMetadataReadsDoNotExposeMaterial(t *testing.T) {
	fixture := principalKeyFixture(identityID)
	api := principalKeyTestAPI(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "/v2/directories/"+identityDirectory+"/principals/"+identityPrincipal+"/api-keys/"+identityID,
			request.URL.Path)
		data, err := json.Marshal(fixture)
		require.NoError(t, err)
		return secretResponse(request, 200, string(data)), nil
	})
	value, err := helpers.GetPrincipalAPIKey(t.Context(), api, identityDirectory, identityPrincipal, identityID)
	require.NoError(t, err)
	require.Equal(t, "primary", value.ManagedBy["kongctl-ref"])
	require.Equal(t, "2026-11-03", value.ExpiresAt.Format("2006-01-02"))
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
	require.NotContains(t, string(encoded), "unexpected-read-material")
	require.NotContains(t, string(encoded), `"name":`)
}

func TestPrincipalAPIKeyPaginationRejectsIncompleteObservations(t *testing.T) {
	for _, mode := range []string{
		"complete", "empty", "failed-page", "repeated-id", "repeated-cursor", "bad-cursor",
		"missing-data", "invalid-id",
	} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			api := principalKeyTestAPI(func(request *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, "1", request.URL.Query().Get("page[size]"))
				if mode == "empty" {
					return identityTestCollection(t, request, []any{}, nil), nil
				}
				if mode == "missing-data" {
					return secretResponse(request, 200, `{"meta":{"page":{"next":null,"size":1}}}`), nil
				}
				if calls > 1 {
					require.Equal(t, "next-cursor", request.URL.Query().Get("page[after]"))
					if mode == "failed-page" {
						return secretResponse(request, 503, `{"secret":"unexpected-read-material"}`), nil
					}
				}
				id := identityID
				if calls > 1 && mode != "repeated-id" {
					id = "731559d3-39bd-48c7-b33c-eb192847eb23"
				}
				if mode == "invalid-id" {
					id = "invalid"
				}
				var next *string
				if calls == 1 || mode == "repeated-cursor" {
					next = new("/v2/directories/" + identityDirectory + "/principals/" + identityPrincipal +
						"/api-keys?page%5Bafter%5D=next-cursor&page%5Bsize%5D=1")
				}
				if mode == "bad-cursor" {
					next = new("https://untrusted.invalid/?unexpected=value")
				}
				return identityTestCollection(t, request, []any{principalKeyFixture(id)}, next), nil
			})
			values, err := helpers.ListPrincipalAPIKeys(t.Context(), api, identityDirectory, identityPrincipal, 1)
			if mode == "complete" || mode == "empty" {
				require.NoError(t, err)
				if mode == "complete" {
					require.Len(t, values, 2)
				} else {
					require.NotNil(t, values)
					require.Empty(t, values)
				}
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "unexpected-read-material")
				require.Nil(t, values)
			}
		})
	}
}

func TestPrincipalAPIKeyReadsRejectWrongScopeAndSanitizeErrors(t *testing.T) {
	for _, mode := range []string{"wrong-parent", "wrong-response-id", "invalid-name-type"} {
		t.Run(mode, func(t *testing.T) {
			api := principalKeyTestAPI(func(request *http.Request) (*http.Response, error) {
				if mode == "wrong-parent" {
					return secretResponse(request, 404, `{"status":404,"detail":"unexpected-read-material"}`), nil
				}
				value := principalKeyFixture(identityID)
				if mode == "wrong-response-id" {
					value["id"] = identityPrincipal
				} else {
					value["name"] = map[string]string{"unexpected": "unexpected-read-material"}
				}
				data, err := json.Marshal(value)
				require.NoError(t, err)
				return secretResponse(request, 200, string(data)), nil
			})
			value, err := helpers.GetPrincipalAPIKey(t.Context(), api, identityDirectory, identityPrincipal, identityID)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "unexpected-read-material")
			require.Nil(t, value)
		})
	}
}
