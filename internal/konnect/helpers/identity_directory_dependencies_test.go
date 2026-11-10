package helpers_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestDirectoryDeletionRequiresUnfilteredTerminalEmptyVaultCollections(t *testing.T) {
	for _, resource := range []string{"vault/keys", "vault/providers", "trusted-idps"} {
		for _, mode := range []string{
			"empty", "attached", "missing-data", "missing-next", "missing-size",
			"fractional-size", "null-data", "nonterminal", "invalid-json", "error",
		} {
			t.Run(resource+"/"+mode, func(t *testing.T) {
				calls := 0
				client := kk.New(kk.WithClient(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
					calls++
					require.Equal(t, http.MethodGet, request.Method, "observation must never change directory settings")
					require.Equal(t, "", request.URL.RawQuery, "no filtering or pagination may hide dependencies")
					body := `{"data":[],"meta":{"page":{"size":20,"next":null}}}`
					status := 200
					if strings.HasSuffix(request.URL.Path, "/"+resource) {
						switch mode {
						case "attached":
							item := map[string]any{
								"id":           "731559d3-39bd-48c7-b33c-eb192847eb21",
								"directory_id": "731559d3-39bd-48c7-b33c-eb192847eb20",
								"name":         "fixture", "public_key": "public", "kid": "public-kid",
								"algorithm": "RSA", "key_size": 2048,
								"template_name": "static_secret", "bearer_methods_supported": []string{"header"},
								"enabled": true, "scopes": []string{}, "credential_type": "shared",
								"pkce_enabled": false, "dcr_enabled": false, "refresh_buffer_seconds": 0,
								"issuer_url": "https://issuer.example.test", "jwks_uri": nil,
								"jit_provisioning_enabled": true, "vault_access_enabled": false,
								"created_at": "2026-10-31T00:00:00Z", "updated_at": "2026-10-31T00:00:00Z",
								"secret": "never-public",
							}
							encoded, err := json.Marshal(map[string]any{
								"data": []any{item},
								"meta": map[string]any{"page": map[string]any{"size": 20, "next": nil}},
							})
							require.NoError(t, err)
							body = string(encoded)
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
						case "nonterminal":
							body = `{"data":[],"meta":{"page":{"size":20,"next":"cursor"}}}`
						case "invalid-json":
							body = `{"detail":"never-public"`
						case "error":
							status, body = 403, `{"detail":"never-public"}`
						}
					}
					return secretResponse(request, status, body), nil
				})))
				api := (&helpers.KonnectSDK{SDK: client}).GetIdentityDirectoryDependenciesAPI()
				err := helpers.RequireEmptyIdentityDirectoryVault(t.Context(), api,
					"731559d3-39bd-48c7-b33c-eb192847eb20", new(true))
				if mode == "empty" {
					require.NoError(t, err)
					require.Equal(t, 3, calls)
				} else {
					require.Error(t, err)
					if mode == "attached" {
						require.ErrorContains(t, err, "attached")
					}
					require.NotContains(t, err.Error(), "never-public")
				}
			})
		}
	}
}

func TestDirectoryDeletionDoesNotEnableVaultOrAssumeMissingAPIsAreEmpty(t *testing.T) {
	client := kk.New(kk.WithClient(secretHTTPFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("disabled or unknown Vault must not trigger configuration writes or hidden collection assumptions")
		return nil, nil
	})))
	api := (&helpers.KonnectSDK{SDK: client}).GetIdentityDirectoryDependenciesAPI()
	for _, value := range []*bool{nil, new(false)} {
		err := helpers.RequireEmptyIdentityDirectoryVault(t.Context(), api,
			"731559d3-39bd-48c7-b33c-eb192847eb20", value)
		require.ErrorContains(t, err, "explicitly enable Vault")
	}
	err := helpers.RequireEmptyIdentityDirectoryVault(t.Context(), nil,
		"731559d3-39bd-48c7-b33c-eb192847eb20", new(true))
	require.ErrorContains(t, err, "not configured")
}
