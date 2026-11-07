package helpers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/stretchr/testify/require"
)

const staticCredentialPath = providerWritePath + "/" + writeFixtureID + "/credentials"

func staticCredentialScope() helpers.IdentityStaticVaultCredentialScope {
	return helpers.IdentityStaticVaultCredentialScope{
		DirectoryID: writeFixtureDirectory, ProviderID: writeFixtureID, ProviderName: "fixture",
	}
}

func staticCredentialAPI(client kk.HTTPClient) helpers.IdentityStaticVaultCredentialsAPI {
	transport := httpclient.NewRetryingHTTPClient(client, httpclient.RetryConfig{
		Strategy: "backoff", MaxAttempts: 3, InitialIntervalMS: 1, MaxIntervalMS: 2, BackoffFactor: 1,
	}, nil)
	sdk := kk.New(kk.WithClient(transport), kk.WithRetryConfig(retry.Config{
		Strategy: "backoff", Backoff: &retry.BackoffStrategy{
			InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10,
		},
	}))
	return (&helpers.KonnectSDK{SDK: sdk}).GetIdentityStaticVaultCredentialsAPI()
}

func staticProviderRecord() map[string]any {
	value := providerWriteRecord()
	value["template_name"], value["credential_type"] = "static_secret", "shared"
	value["dcr_enabled"] = false
	return value
}

func staticCredentialRecord(id string, routing helpers.IdentityVaultCredentialRouting) map[string]any {
	return map[string]any{
		"id": id, "credential_type": "shared", "status": "active",
		"selector": routing.Selector, "priority": routing.Priority,
		"created_at": "2026-10-31T00:00:00Z", "updated_at": "2026-10-31T00:00:01Z",
	}
}

func staticCredentialPage(values []map[string]any, next any) map[string]any {
	return map[string]any{"data": values, "meta": map[string]any{
		"page": map[string]any{"size": 100, "next": next},
	}}
}

func staticCredentialBody(t *testing.T, request *http.Request) map[string]any {
	t.Helper()
	data, err := io.ReadAll(request.Body)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(data, &fields))
	require.Equal(t, "fixture-material", fields["secret"])
	for _, key := range []string{"labels", "managed_by", "ref", "enrollment_url"} {
		require.NotContains(t, fields, key)
	}
	return fields
}

func TestStaticVaultCredentialCreatePreservesDefaultAndZeroPriority(t *testing.T) {
	for _, withRule := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "priority-zero"}[withRule], func(t *testing.T) {
			routing := helpers.IdentityVaultCredentialRouting{}
			if withRule {
				routing.Selector, routing.Priority = new("true"), new(int64(0))
			}
			created, lists, writes := false, 0, 0
			api := staticCredentialAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				switch request.Method + " " + request.URL.Path {
				case "GET " + providerWritePath + "/" + writeFixtureID:
					return providerWriteResponse(t, request, 200, staticProviderRecord()), nil
				case "GET " + staticCredentialPath:
					lists++
					require.Equal(t, "100", request.URL.Query().Get("page[size]"))
					require.False(t, request.URL.Query().Has("size"))
					values := []map[string]any{}
					if created {
						values = append(values, staticCredentialRecord(writeFixturePrincipal, routing))
					}
					return providerWriteResponse(t, request, 200, staticCredentialPage(values, nil)), nil
				case "POST " + staticCredentialPath:
					writes++
					fields := staticCredentialBody(t, request)
					if withRule {
						require.Equal(t, "true", fields["selector"])
						require.EqualValues(t, 0, fields["priority"])
					} else {
						require.NotContains(t, fields, "selector")
						require.NotContains(t, fields, "priority")
					}
					created = true
					value := staticCredentialRecord(writeFixturePrincipal, routing)
					value["secret"], value["enrollment_url"] = "private-response", "private-capability"
					return providerWriteResponse(t, request, 201, value), nil
				default:
					t.Fatalf(
						"unexpected operation, including credential detail GET: %s %s",
						request.Method,
						request.URL.Path,
					)
					return nil, nil
				}
			}))
			value, err := helpers.CreateIdentityStaticVaultCredential(
				t.Context(), api, staticCredentialScope(), routing, "fixture-material",
			)
			require.NoError(t, err)
			require.Equal(t, writeFixturePrincipal, value.ID)
			require.Equal(t, 1, writes)
			require.Equal(t, 2, lists)
			data, err := json.Marshal(value)
			require.NoError(t, err)
			require.NotContains(t, string(data), "private-")
			require.NotContains(t, string(data), "secret")
		})
	}
}

func TestStaticVaultCredentialReplacementHydratesRoutingAndAllowsExplicitDefault(t *testing.T) {
	for _, mode := range []string{"preserve", "new-selector", "default"} {
		t.Run(mode, func(t *testing.T) {
			before := helpers.IdentityVaultCredentialRouting{Selector: new("true"), Priority: new(int64(0))}
			desired := before
			var requested *helpers.IdentityVaultCredentialRouting
			switch mode {
			case "new-selector":
				desired.Selector = new("false")
				requested = &desired
			case "default":
				desired = helpers.IdentityVaultCredentialRouting{}
				requested = &desired
			}
			writes, lists := 0, 0
			api := staticCredentialAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodGet {
					if request.URL.Path == providerWritePath+"/"+writeFixtureID {
						return providerWriteResponse(t, request, 200, staticProviderRecord()), nil
					}
					require.Equal(t, staticCredentialPath, request.URL.Path, "never GET credential detail")
					lists++
					routing := before
					if writes > 0 {
						routing = desired
					}
					return providerWriteResponse(t, request, 200, staticCredentialPage(
						[]map[string]any{staticCredentialRecord(writeFixturePrincipal, routing)}, nil,
					)), nil
				}
				writes++
				require.Equal(t, http.MethodPut, request.Method)
				require.Equal(t, staticCredentialPath+"/"+writeFixturePrincipal, request.URL.Path)
				fields := staticCredentialBody(t, request)
				if mode == "default" {
					require.NotContains(t, fields, "selector")
					require.NotContains(t, fields, "priority")
				} else {
					require.Equal(t, *desired.Selector, fields["selector"])
					require.EqualValues(t, 0, fields["priority"])
				}
				return providerWriteResponse(
					t,
					request,
					200,
					staticCredentialRecord(writeFixturePrincipal, desired),
				), nil
			}))
			value, err := helpers.ReplaceIdentityStaticVaultCredential(t.Context(), api, staticCredentialScope(),
				writeFixturePrincipal, before, requested, "fixture-material")
			require.NoError(t, err)
			require.Equal(t, writeFixturePrincipal, value.ID)
			require.Equal(t, 1, writes)
			require.Equal(t, 2, lists)
		})
	}
}

func TestStaticVaultCredentialGuardsObserveAllPagesBeforeAnyWrite(t *testing.T) {
	for _, operation := range []string{"create", "replace", "delete"} {
		for _, mode := range []string{
			"provider-name", "provider-directory", "provider-template", "provider-mode", "later-page-error",
			"duplicate-slot", "duplicate-default", "user", "pending", "missing-target", "stale-routing", "occupied-slot",
		} {
			t.Run(operation+"/"+mode, func(t *testing.T) {
				routing := helpers.IdentityVaultCredentialRouting{Selector: new("true"), Priority: new(int64(0))}
				api := staticCredentialAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
					require.Equal(t, http.MethodGet, request.Method, "guard failure must precede all mutations")
					if request.URL.Path == providerWritePath+"/"+writeFixtureID {
						body := staticProviderRecord()
						switch mode {
						case "provider-name":
							body["name"] = "another"
						case "provider-directory":
							body["directory_id"] = writeFixturePrincipal
						case "provider-template":
							body["template_name"] = "github"
						case "provider-mode":
							body["credential_type"] = "user"
						}
						return providerWriteResponse(t, request, 200, body), nil
					}
					require.Equal(t, staticCredentialPath, request.URL.Path)
					value := staticCredentialRecord(writeFixturePrincipal, routing)
					if mode == "later-page-error" {
						if request.URL.Query().Get("page[after]") == "second" {
							return secretResponse(request, 403, `{"message":"private-response"}`), nil
						}
						return providerWriteResponse(t, request, 200, staticCredentialPage(
							[]map[string]any{value}, "second",
						)), nil
					}
					values := []map[string]any{value}
					switch mode {
					case "duplicate-default":
						value["selector"], value["priority"] = nil, nil
						values = append(values, staticCredentialRecord(writeFixtureBasicAuth,
							helpers.IdentityVaultCredentialRouting{}))
					case "duplicate-slot":
						values = append(values, staticCredentialRecord(writeFixtureBasicAuth, routing))
					case "user":
						value["credential_type"] = "user"
						value["principal"] = map[string]any{"id": writeFixtureBasicAuth, "display_name": "Fixture"}
					case "pending":
						value["status"] = "pending"
					case "missing-target":
						value["id"] = writeFixtureBasicAuth
					case "stale-routing":
						value["selector"] = "false"
					case "occupied-slot":
						values = append(values, staticCredentialRecord(writeFixtureBasicAuth,
							helpers.IdentityVaultCredentialRouting{}))
					}
					return providerWriteResponse(t, request, 200, staticCredentialPage(values, nil)), nil
				}))
				var err error
				switch operation {
				case "create":
					_, err = helpers.CreateIdentityStaticVaultCredential(
						t.Context(), api, staticCredentialScope(), routing, "fixture-material",
					)
				case "replace":
					_, err = helpers.ReplaceIdentityStaticVaultCredential(t.Context(), api, staticCredentialScope(),
						writeFixturePrincipal, routing, &helpers.IdentityVaultCredentialRouting{}, "fixture-material")
				case "delete":
					if mode == "occupied-slot" {
						// Deletion targets the observed UUID/slot; unrelated slots are preserved.
						return
					}
					err = helpers.DeleteIdentityStaticVaultCredential(
						t.Context(), api, staticCredentialScope(), writeFixturePrincipal, routing,
					)
				}
				require.Error(t, err)
				require.NotContains(t, err.Error(), "private-response")
			})
		}
	}
}

func TestStaticVaultCredentialUncertainWritesAreNeverRetriedOrLeaked(t *testing.T) {
	for _, operation := range []string{"create", "replace", "delete"} {
		t.Run(operation, func(t *testing.T) {
			writes := 0
			routing := helpers.IdentityVaultCredentialRouting{Selector: new("true"), Priority: new(int64(0))}
			api := staticCredentialAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodGet {
					if request.URL.Path == providerWritePath+"/"+writeFixtureID {
						return providerWriteResponse(t, request, 200, staticProviderRecord()), nil
					}
					require.Equal(t, staticCredentialPath, request.URL.Path)
					values := []map[string]any{}
					if operation != "create" {
						values = append(values, staticCredentialRecord(writeFixturePrincipal, routing))
					}
					return providerWriteResponse(t, request, 200, staticCredentialPage(values, nil)), nil
				}
				writes++
				return secretResponse(request, 503, `{"message":"private-response fixture-material"}`), nil
			}))
			var err error
			switch operation {
			case "create":
				_, err = helpers.CreateIdentityStaticVaultCredential(
					t.Context(), api, staticCredentialScope(), routing, "fixture-material",
				)
			case "replace":
				_, err = helpers.ReplaceIdentityStaticVaultCredential(t.Context(), api, staticCredentialScope(),
					writeFixturePrincipal, routing, nil, "fixture-material")
			case "delete":
				err = helpers.DeleteIdentityStaticVaultCredential(
					t.Context(), api, staticCredentialScope(), writeFixturePrincipal, routing,
				)
			}
			require.Error(t, err)
			require.Equal(t, 1, writes)
			require.NotContains(t, err.Error(), "private-response")
			require.NotContains(t, err.Error(), "fixture-material")
		})
	}
}

func TestStaticVaultCredentialReadbackFailuresNeverClaimSuccess(t *testing.T) {
	for _, mode := range []string{"receipt-routing", "receipt-id", "receipt-time", "readback", "delete-retained"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			routing := helpers.IdentityVaultCredentialRouting{Selector: new("true"), Priority: new(int64(0))}
			api := staticCredentialAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				value := staticCredentialRecord(writeFixturePrincipal, routing)
				if request.Method == http.MethodGet {
					if request.URL.Path == providerWritePath+"/"+writeFixtureID {
						return providerWriteResponse(t, request, 200, staticProviderRecord()), nil
					}
					require.Equal(t, staticCredentialPath, request.URL.Path)
					if mode == "readback" && writes > 0 {
						value["selector"] = "false"
					}
					return providerWriteResponse(
						t,
						request,
						200,
						staticCredentialPage([]map[string]any{value}, nil),
					), nil
				}
				writes++
				if mode == "delete-retained" {
					return secretResponse(request, 204, ""), nil
				}
				switch mode {
				case "receipt-routing":
					value["priority"] = 9
				case "receipt-id":
					value["id"] = writeFixtureBasicAuth
				case "receipt-time":
					value["created_at"] = "2026-10-31T01:00:00Z"
				}
				return providerWriteResponse(t, request, 200, value), nil
			}))
			if mode == "delete-retained" {
				err := helpers.DeleteIdentityStaticVaultCredential(
					t.Context(), api, staticCredentialScope(), writeFixturePrincipal, routing,
				)
				require.ErrorContains(t, err, "remains after deletion")
			} else {
				value, err := helpers.ReplaceIdentityStaticVaultCredential(t.Context(), api, staticCredentialScope(),
					writeFixturePrincipal, routing, nil, "fixture-material")
				require.Error(t, err)
				require.Nil(t, value)
			}
			require.Equal(t, 1, writes)
		})
	}
}

func TestStaticVaultCredentialDeletePreservesOtherSlots(t *testing.T) {
	writes := 0
	routing := helpers.IdentityVaultCredentialRouting{Selector: new("true"), Priority: new(int64(0))}
	api := staticCredentialAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet {
			if request.URL.Path == providerWritePath+"/"+writeFixtureID {
				return providerWriteResponse(t, request, 200, staticProviderRecord()), nil
			}
			require.Equal(t, staticCredentialPath, request.URL.Path)
			values := []map[string]any{
				staticCredentialRecord(writeFixtureBasicAuth, helpers.IdentityVaultCredentialRouting{}),
			}
			if writes == 0 {
				values = append(values, staticCredentialRecord(writeFixturePrincipal, routing))
			}
			return providerWriteResponse(t, request, 200, staticCredentialPage(values, nil)), nil
		}
		require.Equal(t, http.MethodDelete, request.Method)
		require.Equal(t, staticCredentialPath+"/"+writeFixturePrincipal, request.URL.Path)
		writes++
		return secretResponse(request, 204, ""), nil
	}))
	require.NoError(t, helpers.DeleteIdentityStaticVaultCredential(
		t.Context(), api, staticCredentialScope(), writeFixturePrincipal, routing,
	))
	require.Equal(t, 1, writes)
}

func TestStaticVaultCredentialInvalidLocalSelectorsPrecedeReads(t *testing.T) {
	var nilSDK *helpers.KonnectSDK
	require.Nil(t, nilSDK.GetIdentityStaticVaultCredentialsAPI())
	require.Nil(t, (&helpers.KonnectSDK{}).GetIdentityStaticVaultCredentialsAPI())
	require.Nil(t, (&helpers.KonnectSDK{SDK: &kk.SDK{}}).GetIdentityStaticVaultCredentialsAPI())
	api := staticCredentialAPI(secretHTTPFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid local inputs must precede all HTTP requests")
		return nil, nil
	}))
	for _, scope := range []helpers.IdentityStaticVaultCredentialScope{
		{DirectoryID: "bad", ProviderID: writeFixtureID, ProviderName: "fixture"},
		{DirectoryID: writeFixtureDirectory, ProviderID: "bad", ProviderName: "fixture"},
		{DirectoryID: writeFixtureDirectory, ProviderID: writeFixtureID, ProviderName: ""},
	} {
		_, err := helpers.CreateIdentityStaticVaultCredential(t.Context(), api, scope,
			helpers.IdentityVaultCredentialRouting{}, "fixture-material")
		require.Error(t, err)
	}
	for _, routing := range []helpers.IdentityVaultCredentialRouting{
		{Selector: new("true")}, {Priority: new(int64(0))}, {Selector: new("true"), Priority: new(int64(-1))},
	} {
		_, err := helpers.CreateIdentityStaticVaultCredential(
			t.Context(), api, staticCredentialScope(), routing, "fixture-material",
		)
		require.Error(t, err)
		_, err = helpers.ReplaceIdentityStaticVaultCredential(t.Context(), api, staticCredentialScope(),
			writeFixturePrincipal, helpers.IdentityVaultCredentialRouting{}, &routing, "fixture-material")
		require.Error(t, err)
	}
	_, err := helpers.CreateIdentityStaticVaultCredential(t.Context(), nil, staticCredentialScope(),
		helpers.IdentityVaultCredentialRouting{}, "fixture-material")
	require.ErrorContains(t, err, "not configured")
	_, err = helpers.ReplaceIdentityStaticVaultCredential(t.Context(), api, staticCredentialScope(), "bad",
		helpers.IdentityVaultCredentialRouting{}, nil, "fixture-material")
	require.ErrorContains(t, err, "UUID")
	require.ErrorContains(t, helpers.DeleteIdentityStaticVaultCredential(
		t.Context(), api, staticCredentialScope(), "bad", helpers.IdentityVaultCredentialRouting{},
	), "UUID")
}
