package helpers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/stretchr/testify/require"
)

const providerWritePath = "/v2/directories/" + writeFixtureDirectory + "/vault/providers"

func providerWriteRecord() map[string]any {
	return map[string]any{ // #nosec G101 -- public OAuth endpoint fixture, with no credentials.
		"id": writeFixtureID, "directory_id": writeFixtureDirectory, "name": "fixture", "template_name": "custom-oauth",
		"client_id": nil, "bearer_methods_supported": []string{"header"}, "enabled": true,
		"scopes": []string{"read"}, "dcr_enabled": true, "credential_type": "user", "pkce_enabled": true,
		"token_endpoint_effective":         "https://fixture.example.test/token",
		"authorization_endpoint_effective": "https://fixture.example.test/authorize",
		"registration_endpoint_effective":  "https://fixture.example.test/register", "refresh_buffer_seconds": 0,
		"created_at": "2026-10-31T00:00:00Z", "updated_at": "2026-10-31T00:00:00Z",
	}
}

func providerWriteResponse(t *testing.T, request *http.Request, status int, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	return secretResponse(request, status, string(data))
}

func providerWriteAPI(client kk.HTTPClient) helpers.IdentityVaultProvidersAPI {
	transport := httpclient.NewRetryingHTTPClient(client, httpclient.RetryConfig{
		Strategy: "backoff", MaxAttempts: 3, InitialIntervalMS: 1, MaxIntervalMS: 2, BackoffFactor: 1,
	}, nil)
	sdk := kk.New(kk.WithClient(transport), kk.WithRetryConfig(retry.Config{
		Strategy: "backoff", Backoff: &retry.BackoffStrategy{
			InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10,
		},
	}))
	return (&helpers.KonnectSDK{SDK: sdk}).GetIdentityVaultProvidersAPI()
}

func TestVaultProviderCreateWireAndUncertainOutcome(t *testing.T) {
	for _, mode := range []string{"created", "error", "wrong-directory", "wrong-name", "wrong-dcr", "ignored-scopes"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			api := providerWriteAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, http.MethodPost, request.Method)
				require.Equal(t, providerWritePath, request.URL.Path)
				data, err := io.ReadAll(request.Body)
				require.NoError(t, err)
				var fields map[string]any
				require.NoError(t, json.Unmarshal(data, &fields))
				require.Equal(t, "fixture", fields["name"])
				require.Equal(t, "custom-oauth", fields["template_name"])
				require.Equal(t, true, fields["dcr_enabled"])
				require.NotContains(t, fields, "client_secret")
				require.NotContains(t, fields, "enabled", "create schema has no enabled field")
				require.NotContains(t, fields, "labels")
				require.NotContains(t, fields, "managed_by")
				if mode == "error" {
					return secretResponse(request, 503, `{"message":"private-response"}`), nil
				}
				body := providerWriteRecord()
				switch mode {
				case "wrong-directory":
					body["directory_id"] = writeFixturePrincipal
				case "wrong-name":
					body["name"] = "another"
				case "wrong-dcr":
					body["dcr_enabled"] = false
				case "ignored-scopes":
					body["scopes"] = []string{"default"}
				}
				return providerWriteResponse(t, request, 201, body), nil
			}))
			value, err := helpers.CreateIdentityVaultProvider(t.Context(), api, writeFixtureDirectory,
				kkComps.CreateVaultProviderBody{
					Name: "fixture", TemplateName: kkComps.ProviderTemplateNameCustomOauth, DcrEnabled: new(true),
					Scopes: []string{"read"}, PkceEnabled: new(true),
					TokenEndpoint:         new("https://fixture.example.test/token"),
					AuthorizationEndpoint: new("https://fixture.example.test/authorize"),
					RegistrationEndpoint:  new("https://fixture.example.test/register"),
				})
			require.Equal(t, 1, calls, "neither retry layer may repeat uncertain creation")
			if mode == "created" {
				require.NoError(t, err)
				require.Equal(t, writeFixtureID, value.ID)
			} else {
				require.Error(t, err)
				require.Nil(t, value)
				require.NotContains(t, err.Error(), "private-response")
			}
		})
	}
}

func TestVaultProviderCreateRejectsNonReconcilableFieldsBeforeWrite(t *testing.T) {
	require.Nil(t, (&helpers.KonnectSDK{}).GetIdentityVaultProvidersAPI())
	require.Nil(t, (&helpers.KonnectSDK{SDK: &kk.SDK{}}).GetIdentityVaultProvidersAPI())
	api := providerWriteAPI(secretHTTPFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid configuration must fail before mutation")
		return nil, nil
	}))
	for _, mutate := range []func(*kkComps.CreateVaultProviderBody){
		func(r *kkComps.CreateVaultProviderBody) { r.Name = "invalid name" },
		func(r *kkComps.CreateVaultProviderBody) { r.TemplateName = "" },
		func(r *kkComps.CreateVaultProviderBody) { r.Scopes = []string{} },
		func(r *kkComps.CreateVaultProviderBody) { r.Scopes = []string{"two scopes"} },
		func(r *kkComps.CreateVaultProviderBody) { r.Scopes = []string{"bad\\scope"} },
		func(r *kkComps.CreateVaultProviderBody) { r.Scopes = []string{"非ascii"} },
		func(r *kkComps.CreateVaultProviderBody) { r.CredentialType = new("other") },
		func(r *kkComps.CreateVaultProviderBody) { r.RefreshBufferSeconds = new(int64(-1)) },
		func(r *kkComps.CreateVaultProviderBody) { r.TokenEndpoint = new("") },
		func(r *kkComps.CreateVaultProviderBody) { r.RegistrationEndpoint = new("") },
		func(r *kkComps.CreateVaultProviderBody) { r.TemplateName = kkComps.ProviderTemplateNameStaticSecret },
		func(r *kkComps.CreateVaultProviderBody) {
			r.TemplateName, r.CredentialType = kkComps.ProviderTemplateNameStaticSecret, new("shared")
			r.ClientID = new("oauth-client")
		},
		func(r *kkComps.CreateVaultProviderBody) {
			r.TemplateName, r.CredentialType = kkComps.ProviderTemplateNameStaticSecret, new("shared")
			r.ClientSecret = new(strings.Repeat("s", 25))
		},
		func(r *kkComps.CreateVaultProviderBody) { r.DcrEnabled, r.ClientID = new(true), new("oauth-client") },
	} {
		request := kkComps.CreateVaultProviderBody{Name: "fixture", TemplateName: kkComps.ProviderTemplateNameGithub}
		mutate(&request)
		_, err := helpers.CreateIdentityVaultProvider(t.Context(), api, writeFixtureDirectory, request)
		require.Error(t, err)
	}
	_, err := helpers.CreateIdentityVaultProvider(t.Context(), nil, writeFixtureDirectory,
		kkComps.CreateVaultProviderBody{Name: "fixture", TemplateName: kkComps.ProviderTemplateNameGithub})
	require.ErrorContains(t, err, "not configured")
	_, err = helpers.CreateIdentityVaultProvider(t.Context(), api, "not-a-uuid",
		kkComps.CreateVaultProviderBody{Name: "fixture", TemplateName: kkComps.ProviderTemplateNameGithub})
	require.ErrorContains(t, err, "UUID")
}

func TestVaultProviderPatchChecksFreshPersistenceAndOmittedFields(t *testing.T) {
	for _, mode := range []string{
		"public", "all-public", "secret", "stale-name", "error", "ignored-enabled", "readback-ignored-enabled",
		"lost-dcr", "lost-registration", "lost-client", "lost-scopes", "readback-error",
	} {
		t.Run(mode, func(t *testing.T) {
			gets, patches := 0, 0
			api := providerWriteAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				require.Equal(t, providerWritePath+"/"+writeFixtureID, request.URL.Path)
				body := providerWriteRecord()
				if mode == "lost-client" || mode == "secret" {
					body["client_id"], body["dcr_enabled"] = "static-client", false
				}
				if request.Method == http.MethodGet {
					gets++
					if gets == 1 {
						if mode == "stale-name" {
							body["name"] = "renamed"
						}
						return providerWriteResponse(t, request, 200, body), nil
					}
					if mode == "readback-error" {
						return secretResponse(request, 403, `{"message":"private-response"}`), nil
					}
				} else {
					patches++
					require.Equal(t, http.MethodPatch, request.Method, "never use replacement PUT")
					data, err := io.ReadAll(request.Body)
					require.NoError(t, err)
					var fields map[string]any
					require.NoError(t, json.Unmarshal(data, &fields))
					require.Equal(t, false, fields["enabled"])
					require.EqualValues(t, 0, fields["refresh_buffer_seconds"], "explicit zero is sent")
					for _, key := range []string{
						"client_id", "template_name", "credential_type", "registration_endpoint", "dcr_enabled",
					} {
						require.NotContains(t, fields, key)
					}
					if mode == "secret" {
						require.Equal(t, strings.Repeat("s", 25), fields["client_secret"])
					} else {
						require.NotContains(t, fields, "client_secret", "ordinary updates must omit write-only fields")
					}
					if mode == "error" {
						return secretResponse(request, 503, `{"message":"private-response"}`), nil
					}
				}
				body["enabled"] = false
				if mode == "all-public" {
					body["name"], body["scopes"], body["pkce_enabled"] = "renamed", []string{"write"}, false
					body["token_endpoint_effective"] = "https://other.example.test/token"
					body["authorization_endpoint_effective"] = "https://other.example.test/authorize"
				}
				if mode == "ignored-enabled" || mode == "readback-ignored-enabled" && gets == 2 {
					body["enabled"] = true
				}
				switch mode {
				case "lost-dcr":
					body["dcr_enabled"] = false
				case "lost-registration":
					body["registration_endpoint_effective"] = nil
				case "lost-client":
					body["client_id"] = nil
				case "lost-scopes":
					body["scopes"] = []string{"default"}
				}
				return providerWriteResponse(t, request, 200, body), nil
			}))
			request := kkComps.PatchVaultProviderBody{Enabled: new(false), RefreshBufferSeconds: new(int64(0))}
			if mode == "secret" {
				request.ClientSecret = new(strings.Repeat("s", 25))
			}
			if mode == "all-public" {
				request.Name, request.Scopes, request.PkceEnabled = new("renamed"), []string{"write"}, new(false)
				request.TokenEndpoint = new("https://other.example.test/token")
				request.AuthorizationEndpoint = new("https://other.example.test/authorize")
			}
			value, err := helpers.UpdateIdentityVaultProvider(t.Context(), api, writeFixtureDirectory,
				writeFixtureID, "fixture", request)
			if mode == "public" || mode == "all-public" || mode == "secret" {
				require.NoError(t, err)
				require.False(t, *value.Enabled)
				require.Equal(t, 2, gets, "fresh GET must prove persistence after successful PATCH")
				encoded, err := json.Marshal(value)
				require.NoError(t, err)
				require.NotContains(t, string(encoded), "client_secret")
				require.NotContains(t, string(encoded), strings.Repeat("s", 25))
			} else {
				require.Error(t, err)
				require.Nil(t, value)
				require.NotContains(t, err.Error(), "private-response")
			}
			if mode == "stale-name" {
				require.Zero(t, patches)
			} else {
				require.Equal(t, 1, patches, "failed writes must not be retried")
			}
		})
	}
}

func TestVaultProviderDeletionRequiresFreshTerminalEmptyCredentials(t *testing.T) {
	for _, mode := range []string{
		"empty", "attached", "nonterminal", "missing-data", "missing-next", "missing-size",
		"fractional-size", "zero-size", "null-data", "invalid-json", "error", "stale-name", "delete-error",
	} {
		t.Run(mode, func(t *testing.T) {
			deletes, observations := 0, 0
			api := providerWriteAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				path := providerWritePath + "/" + writeFixtureID
				switch request.URL.Path {
				case path:
					if request.Method == http.MethodGet {
						body := providerWriteRecord()
						if mode == "stale-name" {
							body["name"] = "renamed"
						}
						return providerWriteResponse(t, request, 200, body), nil
					}
					require.Equal(t, http.MethodDelete, request.Method)
					deletes++
					if mode == "delete-error" {
						return secretResponse(request, 503, `{"message":"private-response"}`), nil
					}
					return secretResponse(request, 204, ""), nil
				case path + "/credentials":
					observations++
					require.Equal(t, http.MethodGet, request.Method)
					require.Empty(t, request.URL.RawQuery, "no filters or cursor arguments may hide credentials")
					body := `{"data":[],"meta":{"page":{"size":20,"next":null}}}`
					status := 200
					switch mode {
					case "attached":
						return providerWriteResponse(t, request, 200, map[string]any{
							"data": []any{map[string]any{
								"id": writeFixturePrincipal, "credential_type": "shared", "status": "active",
								"created_at": "2026-10-31T00:00:00Z", "updated_at": "2026-10-31T00:00:00Z",
							}}, "meta": map[string]any{"page": map[string]any{"size": 20, "next": nil}},
						}), nil
					case "nonterminal":
						body = `{"data":[],"meta":{"page":{"size":20,"next":"cursor"}}}`
					case "missing-data":
						body = `{"meta":{"page":{"size":20,"next":null}}}`
					case "null-data":
						body = `{"data":null,"meta":{"page":{"size":20,"next":null}}}`
					case "missing-next":
						body = `{"data":[],"meta":{"page":{"size":20}}}`
					case "missing-size":
						body = `{"data":[],"meta":{"page":{"next":null}}}`
					case "fractional-size":
						body = `{"data":[],"meta":{"page":{"size":1.5,"next":null}}}`
					case "zero-size":
						body = `{"data":[],"meta":{"page":{"size":0,"next":null}}}`
					case "invalid-json":
						body = `{"private-response"`
					case "error":
						status, body = 403, `{"message":"private-response"}`
					}
					return secretResponse(request, status, body), nil
				default:
					t.Fatalf("unexpected route %s: credential detail must never be read", request.URL.Path)
					return nil, nil
				}
			}))
			err := helpers.DeleteIdentityVaultProvider(
				t.Context(),
				api,
				writeFixtureDirectory,
				writeFixtureID,
				"fixture",
			)
			if mode == "empty" {
				require.NoError(t, err)
				require.Equal(t, 1, deletes)
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "private-response")
				if mode == "delete-error" {
					require.Equal(t, 1, deletes)
				} else {
					require.Zero(t, deletes)
				}
			}
			if mode == "stale-name" {
				require.Zero(t, observations)
			} else {
				require.GreaterOrEqual(t, observations, 1, "read-only retries may repeat a failed observation")
			}
		})
	}
}
