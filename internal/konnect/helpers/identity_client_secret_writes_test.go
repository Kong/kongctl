package helpers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/stretchr/testify/require"
)

type secretHTTPFunc func(*http.Request) (*http.Response, error)

func (f secretHTTPFunc) Do(request *http.Request) (*http.Response, error) { return f(request) }

func secretResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(body)), Request: request,
	}
}

const (
	fixtureServerID    = "731559d3-39bd-48c7-b33c-eb192847eb20"
	fixtureRecordID    = "731559d3-39bd-48c7-b33c-eb192847eb21"
	secretTestResponse = `{"id":"731559d3-39bd-48c7-b33c-eb192847eb21","enabled":false,` +
		`"secret":"issued-private","created_at":"2026-10-31T00:00:00Z","updated_at":"2026-10-31T00:00:00Z"}`
)

func TestSecretCapturePreflightSingleAttemptAndPartialFailureRecovery(t *testing.T) {
	for _, mode := range []string{
		"success", "imported", "api-failure", "missing-value", "removed-output", "existing-output", "invalid-parent",
		"invalid-id-output-failure",
	} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "issued.json")
			if mode == "existing-output" {
				require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
			}
			calls := 0
			inner := secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, http.MethodPost, request.Method)
				require.Equal(t, "/v1/auth-servers/"+fixtureServerID+"/clients/opaque-client/secrets", request.URL.Path)
				pending, err := os.ReadFile(path)
				require.NoError(t, err, "output must be reserved before POST")
				require.Contains(t, string(pending), "creation_not_confirmed")
				body, err := io.ReadAll(request.Body)
				require.NoError(t, err)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(body, &payload))
				require.Equal(t, false, payload["enabled"])
				if mode == "imported" {
					require.Equal(t, "caller-private", payload["secret"])
				} else {
					require.NotContains(t, payload, "secret")
				}
				if mode == "api-failure" {
					return secretResponse(request, 503, `{"secret":"issued-private","message":"uncertain"}`), nil
				}
				response := secretTestResponse
				if mode == "imported" {
					response = strings.ReplaceAll(response, "issued-private", "caller-private")
				}
				if mode == "missing-value" {
					response = strings.ReplaceAll(response, `"secret":"issued-private",`, "")
				}
				if mode == "invalid-id-output-failure" {
					response = strings.ReplaceAll(response, fixtureRecordID, "issued-private")
				}
				if mode == "removed-output" || mode == "invalid-id-output-failure" {
					require.NoError(t, os.Remove(path))
				}
				return secretResponse(request, 201, response), nil
			})
			transport := httpclient.NewRetryingHTTPClient(inner, httpclient.RetryConfig{
				Strategy: "backoff", MaxAttempts: 3, InitialIntervalMS: 1, MaxIntervalMS: 2, BackoffFactor: 1,
			}, nil)
			sdk := kk.New(kk.WithClient(transport), kk.WithServerURL("https://probe.invalid"),
				kk.WithRetryConfig(retry.Config{Strategy: "backoff", Backoff: &retry.BackoffStrategy{
					InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10,
				}}))
			api := (&helpers.KonnectSDK{SDK: sdk}).GetIdentityAuthServerClientsAPI()
			server := fixtureServerID
			if mode == "invalid-parent" {
				server = "../other"
			}
			request := kkComps.CreateSecretPayload{Enabled: new(false)}
			if mode == "imported" {
				request.Secret = new("caller-private")
			}
			value, err := helpers.CreateAndCaptureIdentityClientSecret(t.Context(), api, server, "opaque-client", request, path)
			if mode == "success" || mode == "imported" {
				require.NoError(t, err)
				require.Equal(t, fixtureRecordID, value.ID)
				require.False(t, *value.Enabled)
				public, err := json.Marshal(value)
				require.NoError(t, err)
				require.NotContains(t, string(public), "issued-private")
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				var capture map[string]any
				require.NoError(t, json.Unmarshal(data, &capture))
				material := "issued-private"
				if mode == "imported" {
					material = "caller-private"
				}
				require.Equal(t, material, capture["secret"])
				require.Equal(t, "captured", capture["status"])
				require.Equal(t, fixtureRecordID, capture["secret_id"])
			} else {
				require.Error(t, err)
				require.Nil(t, value)
				require.NotContains(t, err.Error(), "issued-private")
				if mode == "api-failure" {
					require.ErrorContains(t, err, "HTTP 503")
					require.ErrorContains(t, err, "unconfirmed")
				}
			}
			if mode == "invalid-parent" || mode == "existing-output" {
				require.Zero(t, calls, "preflight must finish before creating anything")
			} else {
				require.Equal(t, 1, calls, "both SDK and transport must avoid retrying a create")
			}
		})
	}
}

func TestSecretUpdateDeleteAndErrorResponsesNeverPublishValues(t *testing.T) {
	for _, mode := range []string{"update", "delete", "update-error", "delete-error"} {
		t.Run(mode, func(t *testing.T) {
			client := kk.New(kk.WithServerURL("https://probe.invalid"), kk.WithClient(secretHTTPFunc(
				func(request *http.Request) (*http.Response, error) {
					require.Equal(t, "/v1/auth-servers/"+fixtureServerID+"/clients/client/secrets/"+fixtureRecordID,
						request.URL.Path)
					if strings.HasSuffix(mode, "error") {
						return secretResponse(request, 400, `{"status":400,"title":"Bad request",`+
							`"detail":"issued-private","instance":"trace"}`), nil
					}
					if mode == "delete" {
						require.Equal(t, http.MethodDelete, request.Method)
						return secretResponse(request, 204, ""), nil
					}
					require.Equal(t, http.MethodPatch, request.Method)
					body, err := io.ReadAll(request.Body)
					require.NoError(t, err)
					require.JSONEq(t, `{"enabled":false}`, string(body))
					return secretResponse(request, 200, secretTestResponse), nil
				},
			)))
			api := (&helpers.KonnectSDK{SDK: client}).GetIdentityAuthServerClientsAPI()
			var err error
			if strings.HasPrefix(mode, "delete") {
				err = helpers.DeleteIdentityClientSecret(t.Context(), api, fixtureServerID, "client", fixtureRecordID)
			} else {
				var value *helpers.IdentityClientSecret
				value, err = helpers.UpdateIdentityClientSecret(t.Context(), api, fixtureServerID, "client", fixtureRecordID, false)
				if err == nil {
					data, marshalErr := json.Marshal(value)
					require.NoError(t, marshalErr)
					require.NotContains(t, string(data), "issued-private")
				}
			}
			if strings.HasSuffix(mode, "error") {
				require.ErrorContains(t, err, "HTTP 400")
				require.NotContains(t, err.Error(), "issued-private")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
