package helpers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func basicAuthTestAPI(handler secretHTTPFunc) helpers.IdentityBasicAuthAPI {
	sdk := kk.New(kk.WithServerURL("https://identity.test"), kk.WithClient(handler),
		kk.WithRetryConfig(retry.Config{Strategy: "none"}))
	return (&helpers.KonnectSDK{SDK: sdk}).GetIdentityBasicAuthAPI()
}

func basicAuthFixture(id string, password bool) map[string]any {
	value := map[string]any{
		"id": id, "created_at": "2026-10-31T00:00:00Z", "updated_at": "2026-10-31T00:00:00Z",
		"managed_by": map[string]string{"tool": "kongctl", "kongctl-namespace": "basic-auth-test"},
		"secret":     "unexpected-read-material", "password": "unexpected-read-material",
	}
	if password {
		value["expires_at"] = "2031-01-01T00:00:00Z"
	} else {
		value["username"] = "worker@example.test"
	}
	return value
}

func basicAuthCollectionPath(password bool) string {
	path := "/v2/directories/" + identityDirectory + "/principals/" + identityPrincipal + "/basic-auths"
	if password {
		path += "/" + identityID + "/passwords"
	}
	return path
}

func TestBasicAuthAndPasswordReadsProjectPublicMetadata(t *testing.T) {
	for _, password := range []bool{false, true} {
		t.Run(fmt.Sprint(password), func(t *testing.T) {
			api := basicAuthTestAPI(func(request *http.Request) (*http.Response, error) {
				require.Equal(t, basicAuthCollectionPath(password)+"/"+identityID, request.URL.Path)
				return identityWriteResponse(t, request, 200, basicAuthFixture(identityID, password)), nil
			})
			var value any
			var err error
			if password {
				value, err = helpers.GetIdentityPassword(
					t.Context(),
					api,
					identityDirectory,
					identityPrincipal,
					identityID,
					identityID,
				)
			} else {
				value, err = helpers.GetIdentityBasicAuth(t.Context(), api, identityDirectory, identityPrincipal, identityID)
			}
			require.NoError(t, err)
			for _, encode := range []func(any) ([]byte, error){json.Marshal, yaml.Marshal} {
				encoded, err := encode(value)
				require.NoError(t, err)
				require.Contains(t, string(encoded), "managed_by")
				require.NotContains(t, string(encoded), "unexpected-read-material")
				require.NotContains(t, string(encoded), "secret")
				require.NotContains(t, string(encoded), "password")
			}
		})
	}
}

func TestBasicAuthAndPasswordPaginationRequiresCompleteObservations(t *testing.T) {
	for _, password := range []bool{false, true} {
		for _, mode := range []string{
			"complete", "empty", "failed-page", "repeated-id", "repeated-cursor", "bad-cursor", "invalid-id",
			"missing-data", "missing-meta", "missing-page", "missing-size", "invalid-size", "missing-next", "invalid-next",
		} {
			t.Run(fmt.Sprintf("password=%t/%s", password, mode), func(t *testing.T) {
				calls := 0
				api := basicAuthTestAPI(func(request *http.Request) (*http.Response, error) {
					calls++
					require.Equal(t, basicAuthCollectionPath(password), request.URL.Path)
					require.Equal(t, "1", request.URL.Query().Get("page[size]"))
					require.Empty(t, request.URL.Query().Get("size"))
					id := identityID
					if calls > 1 {
						require.Equal(t, "next-cursor", request.URL.Query().Get("page[after]"))
						if mode == "failed-page" {
							return secretResponse(request, 503, `{"secret":"unexpected-read-material"}`), nil
						}
						if mode != "repeated-id" {
							id = "731559d3-39bd-48c7-b33c-eb192847eb23"
						}
					}
					if mode == "invalid-id" {
						id = "invalid"
					}
					var next any
					if calls == 1 && mode != "empty" || mode == "repeated-cursor" {
						next = basicAuthCollectionPath(password) + "?page%5Bafter%5D=next-cursor&page%5Bsize%5D=1"
					}
					if mode == "bad-cursor" {
						next = "https://untrusted.invalid/?unexpected=value"
					}
					page := map[string]any{"size": 1, "next": next}
					meta := map[string]any{"page": page}
					value := map[string]any{"data": []any{basicAuthFixture(id, password)}, "meta": meta}
					switch mode {
					case "empty":
						value["data"] = []any{}
					case "missing-data":
						delete(value, "data")
					case "missing-meta":
						delete(value, "meta")
					case "missing-page":
						delete(meta, "page")
					case "missing-size":
						delete(page, "size")
					case "invalid-size":
						page["size"] = 0
					case "missing-next":
						delete(page, "next")
					case "invalid-next":
						page["next"] = map[string]string{"secret": "unexpected-read-material"}
					}
					return identityWriteResponse(t, request, 200, value), nil
				})
				var value any
				var err error
				var count int
				if password {
					values, readErr := helpers.ListIdentityPasswords(
						t.Context(),
						api,
						identityDirectory,
						identityPrincipal,
						identityID,
						1,
					)
					value, err, count = values, readErr, len(values)
				} else {
					values, readErr := helpers.ListIdentityBasicAuths(t.Context(), api, identityDirectory, identityPrincipal, 1)
					value, err, count = values, readErr, len(values)
				}
				if mode == "complete" || mode == "empty" {
					require.NoError(t, err)
					require.NotNil(t, value)
					if mode == "complete" {
						require.Equal(t, 2, count)
					} else {
						require.Zero(t, count)
					}
					encoded, err := json.Marshal(value)
					require.NoError(t, err)
					require.NotContains(t, string(encoded), "unexpected-read-material")
				} else {
					require.Error(t, err)
					require.NotContains(t, err.Error(), "unexpected-read-material")
					require.Nil(t, value)
				}
			})
		}
	}
}

func TestBasicAuthUsernameLookupIsExplicitAndRejectsAmbiguity(t *testing.T) {
	for _, mode := range []string{"uuid-username", "duplicate", "missing", "changed", "failed-page"} {
		t.Run(mode, func(t *testing.T) {
			username := identityPrincipal
			gets := 0
			api := basicAuthTestAPI(func(request *http.Request) (*http.Response, error) {
				fixture := basicAuthFixture(identityID, false)
				fixture["username"] = username
				if strings.HasSuffix(request.URL.Path, "/basic-auths") {
					if mode == "failed-page" {
						return secretResponse(request, 503, `{"secret":"unexpected-read-material"}`), nil
					}
					values := []any{fixture}
					switch mode {
					case "duplicate":
						duplicate := basicAuthFixture(identityPrincipal, false)
						duplicate["username"] = username
						values = append(values, duplicate)
					case "missing":
						values = []any{}
					}
					return identityTestCollection(t, request, values, nil), nil
				}
				gets++
				require.True(t, strings.HasSuffix(request.URL.Path, "/basic-auths/"+identityID))
				if mode == "changed" {
					fixture["username"] = "changed"
				}
				return identityWriteResponse(t, request, 200, fixture), nil
			})
			value, err := helpers.ResolveIdentityBasicAuthUsername(
				t.Context(),
				api,
				identityDirectory,
				identityPrincipal,
				username,
				1,
			)
			if mode == "uuid-username" {
				require.NoError(t, err)
				require.Equal(t, identityID, value.ID)
				require.Equal(t, username, value.Username)
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "unexpected-read-material")
				require.Nil(t, value)
				if mode != "changed" {
					require.Zero(t, gets)
				}
			}
		})
	}
}

func TestBasicAuthAndPasswordDetailRejectsBadResponsesWithoutBodyLeaks(t *testing.T) {
	for _, password := range []bool{false, true} {
		for _, mode := range []string{"wrong-parent", "wrong-id", "malformed-map", "malformed-expiry", "missing-username"} {
			t.Run(fmt.Sprintf("password=%t/%s", password, mode), func(t *testing.T) {
				if !password && mode == "malformed-expiry" || password && mode == "missing-username" {
					return
				}
				api := basicAuthTestAPI(func(request *http.Request) (*http.Response, error) {
					if mode == "wrong-parent" {
						return secretResponse(request, 404, `{"status":404,"detail":"unexpected-read-material"}`), nil
					}
					fixture := basicAuthFixture(identityID, password)
					switch mode {
					case "wrong-id":
						fixture["id"] = identityPrincipal
					case "malformed-map":
						fixture["managed_by"] = "unexpected-read-material"
					case "malformed-expiry":
						fixture["expires_at"] = "unexpected-read-material"
					case "missing-username":
						delete(fixture, "username")
					}
					return identityWriteResponse(t, request, 200, fixture), nil
				})
				var err error
				if password {
					_, err = helpers.GetIdentityPassword(
						t.Context(),
						api,
						identityDirectory,
						identityPrincipal,
						identityID,
						identityID,
					)
				} else {
					_, err = helpers.GetIdentityBasicAuth(t.Context(), api, identityDirectory, identityPrincipal, identityID)
				}
				require.Error(t, err)
				require.NotContains(t, fmt.Sprintf("%+v", err), "unexpected-read-material")
			})
		}
	}
}

func TestBasicAuthWritesVerifyRetainedMetadataAndFreshReadback(t *testing.T) {
	for _, create := range []bool{false, true} {
		for _, mode := range []string{
			"success", "dropped-metadata", "changed-id", "wrong-username", "readback-drift", "uncertain",
		} {
			t.Run(fmt.Sprintf("create=%t/%s", create, mode), func(t *testing.T) {
				metadata := map[string]string{"tool": "kongctl", "kongctl-namespace": "basic-auth-test"}
				writes, reads := 0, 0
				api := basicAuthTestAPI(func(request *http.Request) (*http.Response, error) {
					fixture := basicAuthFixture(identityID, false)
					if request.Method == http.MethodGet {
						reads++
						if mode == "readback-drift" {
							fixture["managed_by"] = map[string]string{}
						}
						return identityWriteResponse(t, request, 200, fixture), nil
					}
					writes++
					var body map[string]any
					require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
					require.Len(t, body, 2)
					require.Equal(t, "worker@example.test", body["username"])
					require.Contains(t, body, "managed_by")
					status := 200
					if create {
						require.Equal(t, http.MethodPost, request.Method)
						status = 201
					} else {
						require.Equal(t, http.MethodPut, request.Method)
					}
					switch mode {
					case "dropped-metadata":
						fixture["managed_by"] = map[string]string{}
					case "changed-id":
						fixture["id"] = "invalid"
					case "wrong-username":
						fixture["username"] = "other"
					case "uncertain":
						return secretResponse(request, 503, `{"secret":"unexpected-read-material"}`), nil
					}
					return identityWriteResponse(t, request, status, fixture), nil
				})
				var value *helpers.IdentityBasicAuth
				var err error
				if create {
					value, err = helpers.CreateIdentityBasicAuth(t.Context(), api, identityDirectory, identityPrincipal,
						kkComps.CreateBasicAuthBody{Username: "worker@example.test", ManagedBy: metadata})
				} else {
					value, err = helpers.ReplaceIdentityBasicAuth(t.Context(), api, identityDirectory, identityPrincipal, identityID,
						kkComps.ReplaceBasicAuthBody{Username: "worker@example.test", ManagedBy: metadata})
				}
				require.Equal(t, 1, writes)
				if mode == "success" {
					require.NoError(t, err)
					require.Equal(t, identityID, value.ID)
					require.Equal(t, 1, reads)
				} else {
					require.Error(t, err)
					require.Nil(t, value)
					require.NotContains(t, err.Error(), "unexpected-read-material")
				}
			})
		}
	}
}

func TestBasicAuthDeletionRequiresCompleteEmptyPasswordCollection(t *testing.T) {
	for _, mode := range []string{"empty", "password", "failed-list", "incomplete", "late-page-password", "delete-error"} {
		t.Run(mode, func(t *testing.T) {
			deletes, lists := 0, 0
			api := basicAuthTestAPI(func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodDelete {
					deletes++
					if mode == "delete-error" {
						return secretResponse(request, 403, `{"status":403,"detail":"unexpected-read-material"}`), nil
					}
					return secretResponse(request, 204, ""), nil
				}
				lists++
				require.Equal(t, basicAuthCollectionPath(true), request.URL.Path)
				if mode == "failed-list" {
					return secretResponse(request, 403, `{"status":403,"detail":"unexpected-read-material"}`), nil
				}
				if mode == "incomplete" {
					return secretResponse(request, 200, `{"data":[]}`), nil
				}
				var next *string
				values := []any{}
				if mode == "late-page-password" && lists == 1 {
					next = new("next-cursor")
				}
				if mode == "password" || mode == "late-page-password" && lists == 2 {
					values = []any{basicAuthFixture(identityID, true)}
				}
				return identityTestCollection(t, request, values, next), nil
			})
			err := helpers.DeleteIdentityBasicAuth(t.Context(), api, identityDirectory, identityPrincipal, identityID)
			if mode == "empty" {
				require.NoError(t, err)
				require.Equal(t, 1, deletes)
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "unexpected-read-material")
				if mode == "delete-error" {
					require.Equal(t, 1, deletes)
				} else {
					require.Zero(t, deletes)
				}
			}
		})
	}
}

func TestBasicAuthReplacementClonesMetadataWithoutPasswords(t *testing.T) {
	value := helpers.IdentityBasicAuth{Username: "worker", ManagedBy: map[string]string{"tool": "kongctl"}}
	body := helpers.IdentityBasicAuthReplacement(value)
	body.ManagedBy["tool"] = "changed"
	require.Equal(t, "kongctl", value.ManagedBy["tool"])
	require.Equal(t, "worker", body.Username)
}

func TestBasicAuthCreateDisablesBothRetryLayers(t *testing.T) {
	calls := 0
	inner := secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return secretResponse(request, 503, `{"secret":"unexpected-read-material"}`), nil
	})
	transport := httpclient.NewRetryingHTTPClient(inner, httpclient.RetryConfig{
		Strategy: httpclient.RetryStrategyBackoff, MaxAttempts: 3,
		InitialIntervalMS: 1, MaxIntervalMS: 2, BackoffFactor: 1,
	}, nil)
	sdk := kk.New(kk.WithServerURL("https://identity.test"), kk.WithClient(transport),
		kk.WithRetryConfig(retry.Config{Strategy: "backoff", Backoff: &retry.BackoffStrategy{
			InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10,
		}}))
	value, err := helpers.CreateIdentityBasicAuth(
		t.Context(),
		(&helpers.KonnectSDK{SDK: sdk}).GetIdentityBasicAuthAPI(),
		identityDirectory,
		identityPrincipal,
		kkComps.CreateBasicAuthBody{Username: "worker"},
	)
	require.ErrorContains(t, err, "inspect the collection before retrying")
	require.NotContains(t, err.Error(), "unexpected-read-material")
	require.Nil(t, value)
	require.Equal(t, 1, calls)
}

func TestBasicAuthWriteValidatesExplicitUsernameBeforeRequests(t *testing.T) {
	for _, username := range []string{"", strings.Repeat("a", 256), string([]byte{0xff})} {
		api := basicAuthTestAPI(func(_ *http.Request) (*http.Response, error) {
			t.Fatal("invalid username must not send a request")
			return nil, nil
		})
		_, err := helpers.CreateIdentityBasicAuth(t.Context(), api, identityDirectory, identityPrincipal,
			kkComps.CreateBasicAuthBody{Username: username})
		require.ErrorContains(t, err, "1 to 255")
		_, err = helpers.ReplaceIdentityBasicAuth(t.Context(), api, identityDirectory, identityPrincipal, identityID,
			kkComps.ReplaceBasicAuthBody{Username: username})
		require.ErrorContains(t, err, "1 to 255")
	}
}
