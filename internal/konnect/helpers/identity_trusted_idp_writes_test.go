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

const (
	trustWritePath   = "/v2/directories/" + writeFixtureDirectory + "/trusted-idps"
	trustWriteIssuer = "https://issuer.example.test/auth"
)

func trustWriteAPI(client kk.HTTPClient) helpers.IdentityTrustedIDPsAPI {
	transport := httpclient.NewRetryingHTTPClient(client, httpclient.RetryConfig{
		Strategy: "backoff", MaxAttempts: 3, InitialIntervalMS: 1, MaxIntervalMS: 2, BackoffFactor: 1,
	}, nil)
	sdk := kk.New(kk.WithClient(transport), kk.WithRetryConfig(retry.Config{
		Strategy: "backoff", Backoff: &retry.BackoffStrategy{
			InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10,
		},
	}))
	return (&helpers.KonnectSDK{SDK: sdk}).GetIdentityTrustedIDPsAPI()
}

func trustWriteRecord() map[string]any {
	return map[string]any{
		"id": writeFixtureID, "issuer_url": trustWriteIssuer, "display_name": "Fixture trust",
		"jwks_uri": "https://issuer.example.test/keys", "jit_provisioning_enabled": false, "vault_access_enabled": true,
		"jit": map[string]any{"mapping": map[string]any{
			"email": "email", "groups": "groups", "display_name": "name",
			"metadata": map[string]string{"department": "department", "team": "team"},
		}},
		"created_at": "2026-10-31T00:00:00Z", "updated_at": "2026-10-31T00:00:00Z",
	}
}

func trustWriteBody() kkComps.ReplaceTrustedIdpBody {
	return kkComps.ReplaceTrustedIdpBody{
		IssuerURL: trustWriteIssuer, DisplayName: new("Updated trust"), JwksURI: new("https://issuer.example.test/keys"),
		JitProvisioningEnabled: new(false), VaultAccessEnabled: new(true), Jit: &kkComps.TrustedIdpJit{
			Mapping: &kkComps.TrustedIdpJitMapping{
				Email: new("email"), Groups: new("groups"), DisplayName: new("name"),
				Metadata: map[string]string{"department": "department", "team": "team"},
			},
		},
	}
}

func trustPage(value any) map[string]any {
	return map[string]any{"data": value, "meta": map[string]any{"page": map[string]any{"size": 20, "next": nil}}}
}

func TestTrustedIDPCreateChecksPublicReceiptAndDoesNotRetry(t *testing.T) {
	for _, mode := range []string{"created", "error", "wrong-issuer", "missing-timestamps", "ignored-jit"} {
		t.Run(mode, func(t *testing.T) {
			posts := 0
			api := trustWriteAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				posts++
				require.Equal(t, http.MethodPost, request.Method)
				require.Equal(t, trustWritePath, request.URL.Path)
				data, err := io.ReadAll(request.Body)
				require.NoError(t, err)
				var fields map[string]any
				require.NoError(t, json.Unmarshal(data, &fields))
				require.Equal(t, false, fields["jit_provisioning_enabled"])
				require.Equal(t, true, fields["vault_access_enabled"])
				require.NotContains(t, fields, "managed_by")
				require.NotContains(t, fields, "labels")
				if mode == "error" {
					return secretResponse(request, 503, `{"detail":"private-backend-body"}`), nil
				}
				value := trustWriteRecord()
				switch mode {
				case "wrong-issuer":
					value["issuer_url"] = "https://other.example.test/auth"
				case "missing-timestamps":
					delete(value, "created_at")
				case "ignored-jit":
					delete(value, "jit")
				}
				return providerWriteResponse(t, request, 201, value), nil
			}))
			body := trustWriteBody()
			value, err := helpers.CreateIdentityTrustedIDP(
				t.Context(),
				api,
				writeFixtureDirectory,
				kkComps.CreateTrustedIdpPayload{
					IssuerURL: body.IssuerURL, DisplayName: new("Fixture trust"), JwksURI: body.JwksURI,
					JitProvisioningEnabled: body.JitProvisioningEnabled, VaultAccessEnabled: body.VaultAccessEnabled, Jit: body.Jit,
				},
			)
			require.Equal(t, 1, posts, "uncertain creates must not be repeated by either retry layer")
			if mode == "created" {
				require.NoError(t, err)
				require.Equal(t, writeFixtureID, value.ID)
			} else {
				require.Error(t, err)
				require.Nil(t, value)
				require.NotContains(t, err.Error(), "private-backend-body")
			}
		})
	}
}

func TestTrustedIDPReplacementVerifiesScopeFullConfigurationAndPersistence(t *testing.T) {
	for _, mode := range []string{
		"updated", "clear", "error", "wrong-scope", "changed-issuer", "ignored-boolean", "lost-jit", "readback-not-persisted",
	} {
		t.Run(mode, func(t *testing.T) {
			gets, puts := 0, 0
			api := trustWriteAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				value := trustWriteRecord()
				if request.URL.Path == trustWritePath {
					require.Equal(t, http.MethodGet, request.Method)
					require.Equal(t, "100", request.URL.Query().Get("page[size]"))
					if mode == "wrong-scope" {
						return providerWriteResponse(t, request, 200, trustPage([]any{})), nil
					}
					return providerWriteResponse(t, request, 200, trustPage([]any{value})), nil
				}
				require.Equal(t, trustWritePath+"/"+writeFixtureID, request.URL.Path)
				if request.Method == http.MethodGet {
					gets++
					if gets == 1 {
						if mode == "changed-issuer" {
							value["issuer_url"] = "https://other.example.test/auth"
						}
						return providerWriteResponse(t, request, 200, value), nil
					}
				} else {
					puts++
					require.Equal(t, http.MethodPut, request.Method)
					data, err := io.ReadAll(request.Body)
					require.NoError(t, err)
					var fields map[string]any
					require.NoError(t, json.Unmarshal(data, &fields))
					require.Equal(t, false, fields["jit_provisioning_enabled"])
					require.Equal(t, true, fields["vault_access_enabled"])
					if mode == "clear" {
						require.NotContains(t, fields, "display_name")
						require.NotContains(t, fields, "jwks_uri")
						require.Equal(t, map[string]any{}, fields["jit"])
					} else {
						require.Contains(t, fields, "display_name")
						require.Contains(t, fields, "jwks_uri")
						require.Contains(t, fields, "jit")
					}
					if mode == "error" {
						return secretResponse(request, 503, `{"detail":"private-backend-body"}`), nil
					}
				}
				value["display_name"] = "Updated trust"
				if mode == "clear" {
					value["display_name"], value["jwks_uri"] = nil, nil
					delete(value, "jit")
				}
				if mode == "ignored-boolean" {
					value["jit_provisioning_enabled"] = true
				}
				if mode == "lost-jit" {
					delete(value, "jit")
				}
				if mode == "readback-not-persisted" && gets == 2 {
					value["display_name"] = "Original trust"
				}
				return providerWriteResponse(t, request, 200, value), nil
			}))
			body := trustWriteBody()
			if mode == "clear" {
				body.DisplayName, body.JwksURI = nil, nil
				body.Jit = &kkComps.TrustedIdpJit{}
			}
			value, err := helpers.ReplaceIdentityTrustedIDP(
				t.Context(),
				api,
				writeFixtureDirectory,
				writeFixtureID,
				trustWriteIssuer,
				body,
			)
			if mode == "updated" || mode == "clear" {
				require.NoError(t, err)
				require.Equal(t, 2, gets, "a fresh GET verifies actual persistence")
				require.False(t, *value.JitProvisioningEnabled)
			} else {
				require.Error(t, err)
				require.Nil(t, value)
				require.NotContains(t, err.Error(), "private-backend-body")
			}
			if mode == "wrong-scope" || mode == "changed-issuer" {
				require.Zero(t, puts)
			} else {
				require.Equal(t, 1, puts)
			}
		})
	}
}

func TestTrustedIDPDeletionRequiresExplicitTerminalEmptyAssociations(t *testing.T) {
	for _, mode := range []string{
		"empty", "inactive-associated", "nonterminal", "null-data", "missing-next",
		"fractional-page", "error", "delete-error",
	} {
		t.Run(mode, func(t *testing.T) {
			deletes := 0
			api := trustWriteAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				switch request.URL.Path {
				case trustWritePath:
					return providerWriteResponse(t, request, 200, trustPage([]any{trustWriteRecord()})), nil
				case trustWritePath + "/" + writeFixtureID:
					if request.Method == http.MethodGet {
						return providerWriteResponse(t, request, 200, trustWriteRecord()), nil
					}
					require.Equal(t, http.MethodDelete, request.Method)
					deletes++
					if mode == "delete-error" {
						return secretResponse(request, 503, `{"detail":"private-backend-body"}`), nil
					}
					return secretResponse(request, 204, ""), nil
				case trustWritePath + "/" + writeFixtureID + "/principals":
					require.Equal(t, http.MethodGet, request.Method)
					require.Empty(t, request.URL.RawQuery)
					if mode == "inactive-associated" {
						return providerWriteResponse(t, request, 200, trustPage([]any{map[string]any{
							"id": writeFixturePrincipal, "display_name": "Inactive fixture", "description": "", "type": "user",
							"active": false, "provisioned_by": "konnect_user", "managed_by": map[string]string{},
							"labels": map[string]string{}, "metadata": map[string]any{},
							"created_at": "2026-10-31T00:00:00Z", "updated_at": "2026-10-31T00:00:00Z",
						}})), nil
					}
					body := `{"data":[],"meta":{"page":{"size":20,"next":null}}}`
					switch mode {
					case "nonterminal":
						body = `{"data":[],"meta":{"page":{"size":20,"next":"cursor"}}}`
					case "null-data":
						body = `{"data":null,"meta":{"page":{"size":20,"next":null}}}`
					case "missing-next":
						body = `{"data":[],"meta":{"page":{"size":20}}}`
					case "fractional-page":
						body = `{"data":[],"meta":{"page":{"size":1.5,"next":null}}}`
					case "error":
						return secretResponse(request, 403, `{"detail":"private-backend-body"}`), nil
					}
					return secretResponse(request, 200, body), nil
				default:
					t.Fatalf("unexpected trust route %s", request.URL.Path)
					return nil, nil
				}
			}))
			err := helpers.DeleteIdentityTrustedIDP(
				t.Context(),
				api,
				writeFixtureDirectory,
				writeFixtureID,
				trustWriteIssuer,
			)
			if mode == "empty" {
				require.NoError(t, err)
				require.Equal(t, 1, deletes)
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "private-backend-body")
				if mode == "delete-error" {
					require.Equal(t, 1, deletes)
				} else {
					require.Zero(t, deletes)
				}
			}
		})
	}
}

func TestTrustedIDPReplacementRequiresHydratedBooleansAndStableIssuer(t *testing.T) {
	api := trustWriteAPI(secretHTTPFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("incomplete replacement must be rejected before observation or mutation")
		return nil, nil
	}))
	for _, body := range []kkComps.ReplaceTrustedIdpBody{
		{IssuerURL: trustWriteIssuer},
		{IssuerURL: trustWriteIssuer, JitProvisioningEnabled: new(false)},
		{IssuerURL: "https://different.example.test", JitProvisioningEnabled: new(false), VaultAccessEnabled: new(false)},
		{
			IssuerURL: trustWriteIssuer, DisplayName: new(strings.Repeat("x", 256)),
			JitProvisioningEnabled: new(false), VaultAccessEnabled: new(false),
		},
	} {
		_, err := helpers.ReplaceIdentityTrustedIDP(
			t.Context(),
			api,
			writeFixtureDirectory,
			writeFixtureID,
			trustWriteIssuer,
			body,
		)
		require.Error(t, err)
	}
}
