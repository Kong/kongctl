package helpers_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/stretchr/testify/require"
)

func identityWriteFixture(t *testing.T, fixture map[string]any) helpers.PrincipalIdentity {
	t.Helper()
	data, err := json.Marshal(fixture)
	require.NoError(t, err)
	var value helpers.PrincipalIdentity
	require.NoError(t, json.Unmarshal(data, &value))
	return value
}

func identityWriteResponse(t *testing.T, request *http.Request, status int, fixture map[string]any) *http.Response {
	t.Helper()
	data, err := json.Marshal(fixture)
	require.NoError(t, err)
	return secretResponse(request, status, string(data))
}

func TestPrincipalIdentityReplacementPreservesPublicFields(t *testing.T) {
	for _, kind := range []string{"oidc", "auth_server_client", "control_plane_consumer", "custom"} {
		t.Run(kind, func(t *testing.T) {
			fixture := principalIdentityFixture(identityID, kind)
			value := identityWriteFixture(t, fixture)
			body, err := helpers.PrincipalIdentityReplacement(value)
			require.NoError(t, err)
			encoded, err := json.Marshal(body)
			require.NoError(t, err)
			for _, key := range []string{"id", "created_at", "updated_at", "control_plane_type"} {
				delete(fixture, key)
			}
			expected, err := json.Marshal(fixture)
			require.NoError(t, err)
			require.JSONEq(t, string(expected), string(encoded))
			value.Labels["team"] = "changed after projection"
			value.ManagedBy["kongctl-ref"] = "changed-ref"
			encodedAfter, err := json.Marshal(body)
			require.NoError(t, err)
			require.JSONEq(t, string(encoded), string(encodedAfter), "full replacement must own its metadata snapshot")
		})
	}
	for _, plane := range []string{"api_gateway", "ai_gateway"} {
		fixture := principalIdentityFixture(identityID, "control_plane_consumer")
		delete(fixture, "workspace_id")
		fixture["control_plane_type"] = plane
		body, err := helpers.PrincipalIdentityReplacement(identityWriteFixture(t, fixture))
		require.NoError(t, err)
		data, err := json.Marshal(body)
		require.NoError(t, err)
		require.NotContains(t, string(data), "workspace_id")
		require.NotContains(t, string(data), "control_plane_type")
	}
	for _, value := range []helpers.PrincipalIdentity{
		{Type: "oidc"}, {Type: "auth_server_client"}, {Type: "control_plane_consumer"}, {Type: "custom"}, {Type: "new"},
	} {
		_, err := helpers.PrincipalIdentityReplacement(value)
		require.Error(t, err)
	}
}

func TestPrincipalIdentityCreateVerifiesReadBack(t *testing.T) {
	for _, kind := range []string{"oidc", "auth_server_client", "control_plane_consumer", "custom"} {
		t.Run(kind, func(t *testing.T) {
			fixture := principalIdentityFixture(identityID, kind)
			body, err := helpers.PrincipalIdentityReplacement(identityWriteFixture(t, fixture))
			require.NoError(t, err)
			var methods []string
			api := identityTestAPI(func(request *http.Request) (*http.Response, error) {
				methods = append(methods, request.Method)
				require.Contains(t, request.URL.Path, "/principals/"+identityPrincipal+"/identities")
				if request.Method == http.MethodPost {
					var payload map[string]any
					require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
					require.NotContains(t, payload, "id")
					require.Equal(t, kind, payload["type"])
					return identityWriteResponse(t, request, 201, fixture), nil
				}
				if strings.HasSuffix(request.URL.Path, "/identities") {
					return identityTestCollection(t, request, []any{fixture}, nil), nil
				}
				return identityWriteResponse(t, request, 200, fixture), nil
			})
			created, err := helpers.CreatePrincipalIdentity(
				t.Context(),
				api,
				identityDirectory,
				identityPrincipal,
				body,
			)
			require.NoError(t, err)
			require.Equal(t, identityID, created.ID)
			require.Equal(t, kind, created.Type)
			require.Equal(t, []string{"POST", "GET", "GET"}, methods)
		})
	}
	for _, mode := range []string{"dropped-metadata", "changed-type", "not-in-collection", "changed-read-back"} {
		t.Run(mode, func(t *testing.T) {
			fixture := principalIdentityFixture(identityID, "custom")
			body, err := helpers.PrincipalIdentityReplacement(identityWriteFixture(t, fixture))
			require.NoError(t, err)
			posts := 0
			api := identityTestAPI(func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodPost {
					posts++
					switch mode {
					case "dropped-metadata":
						fixture["managed_by"] = map[string]string{}
					case "changed-type":
						fixture = principalIdentityFixture(identityID, "oidc")
					}
					return identityWriteResponse(t, request, 201, fixture), nil
				}
				if strings.HasSuffix(request.URL.Path, "/identities") {
					if mode == "not-in-collection" {
						return identityTestCollection(t, request, []any{}, nil), nil
					}
					return identityTestCollection(t, request, []any{fixture}, nil), nil
				}
				fixture["managed_by"] = map[string]string{}
				return identityWriteResponse(t, request, 200, fixture), nil
			})
			created, err := helpers.CreatePrincipalIdentity(
				t.Context(),
				api,
				identityDirectory,
				identityPrincipal,
				body,
			)
			require.Error(t, err)
			require.Nil(t, created)
			require.Equal(t, 1, posts, "do not retry or roll back an unconfirmed creation")
		})
	}
}

func TestPrincipalIdentityCreateDoesNotReplayUncertainResponse(t *testing.T) {
	calls := 0
	inner := secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return secretResponse(request, 503, `{"message":"uncertain outcome"}`), nil
	})
	transport := httpclient.NewRetryingHTTPClient(inner, httpclient.RetryConfig{
		Strategy: httpclient.RetryStrategyBackoff, MaxAttempts: 3,
		InitialIntervalMS: 1, MaxIntervalMS: 2, BackoffFactor: 1,
	}, nil)
	sdk := kk.New(kk.WithServerURL("https://identity.test"), kk.WithClient(transport),
		kk.WithRetryConfig(retry.Config{Strategy: "backoff", Backoff: &retry.BackoffStrategy{
			InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10,
		}}))
	body, err := helpers.PrincipalIdentityReplacement(
		identityWriteFixture(t, principalIdentityFixture(identityID, "custom")),
	)
	require.NoError(t, err)
	_, err = helpers.CreatePrincipalIdentity(
		t.Context(),
		(&helpers.KonnectSDK{SDK: sdk}).GetIdentityPrincipalIdentitiesAPI(),
		identityDirectory,
		identityPrincipal,
		body,
	)
	require.ErrorContains(t, err, "inspect the collection before retrying")
	require.Equal(t, 1, calls, "disable SDK and transport POST retries")
}

func TestPrincipalIdentityMutationsRequireCompleteFreshMembership(t *testing.T) {
	for _, action := range []string{"replace", "delete"} {
		for _, mode := range []string{"success", "wrong-parent", "failed-page", "missing-data", "changed-type"} {
			t.Run(action+"/"+mode, func(t *testing.T) {
				fixture := principalIdentityFixture(identityID, "custom")
				body, err := helpers.PrincipalIdentityReplacement(identityWriteFixture(t, fixture))
				require.NoError(t, err)
				lists, details, writes := 0, 0, 0
				api := identityTestAPI(func(request *http.Request) (*http.Response, error) {
					if request.Method == http.MethodGet {
						if strings.HasSuffix(request.URL.Path, "/identities") {
							lists++
							if mode == "wrong-parent" {
								return identityTestCollection(t, request, []any{}, nil), nil
							}
							if mode == "missing-data" {
								return secretResponse(request, 200, `{"meta":{"page":{"size":1,"next":null}}}`), nil
							}
							if mode == "failed-page" {
								if lists == 1 {
									return identityTestCollection(t, request, []any{fixture}, new("next")), nil
								}
								return secretResponse(request, 503, `{}`), nil
							}
							return identityTestCollection(t, request, []any{fixture}, nil), nil
						}
						details++
						if mode == "changed-type" {
							return identityWriteResponse(
								t,
								request,
								200,
								principalIdentityFixture(identityID, "oidc"),
							), nil
						}
						return identityWriteResponse(t, request, 200, fixture), nil
					}
					writes++
					if action == "delete" {
						require.Equal(t, http.MethodDelete, request.Method)
						return secretResponse(request, 204, ""), nil
					}
					require.Equal(t, http.MethodPut, request.Method)
					var payload map[string]any
					require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
					require.NotContains(t, payload, "id")
					require.NotContains(t, payload, "created_at")
					require.NotContains(t, payload, "control_plane_type")
					return identityWriteResponse(t, request, 200, fixture), nil
				})
				if action == "delete" {
					err = helpers.DeletePrincipalIdentity(
						t.Context(),
						api,
						identityDirectory,
						identityPrincipal,
						identityID,
					)
				} else {
					_, err = helpers.ReplacePrincipalIdentity(t.Context(), api, identityDirectory, identityPrincipal, identityID, body)
				}
				if mode == "success" || action == "delete" && mode == "changed-type" {
					require.NoError(t, err)
					require.Equal(t, 1, writes)
				} else {
					require.Error(t, err)
					require.Zero(t, writes)
				}
				if mode == "wrong-parent" || mode == "failed-page" || mode == "missing-data" {
					require.Zero(t, details, "an unscoped GET is not membership proof")
				}
			})
		}
	}
}

func TestPrincipalIdentityReplacementRejectsUnconfirmedWrite(t *testing.T) {
	for _, mode := range []string{"changed-id", "dropped-metadata", "missing-response"} {
		t.Run(mode, func(t *testing.T) {
			fixture := principalIdentityFixture(identityID, "custom")
			body, err := helpers.PrincipalIdentityReplacement(identityWriteFixture(t, fixture))
			require.NoError(t, err)
			writes := 0
			api := identityTestAPI(func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodGet {
					if strings.HasSuffix(request.URL.Path, "/identities") {
						return identityTestCollection(t, request, []any{fixture}, nil), nil
					}
					return identityWriteResponse(t, request, 200, fixture), nil
				}
				writes++
				switch mode {
				case "changed-id":
					fixture["id"] = "731559d3-39bd-48c7-b33c-eb192847eb23"
				case "dropped-metadata":
					fixture["managed_by"] = map[string]string{}
				case "missing-response":
					return secretResponse(request, 200, ""), nil
				}
				return identityWriteResponse(t, request, 200, fixture), nil
			})
			value, err := helpers.ReplacePrincipalIdentity(
				t.Context(),
				api,
				identityDirectory,
				identityPrincipal,
				identityID,
				body,
			)
			require.Error(t, err)
			require.Nil(t, value)
			require.Equal(t, 1, writes)
		})
	}
}
