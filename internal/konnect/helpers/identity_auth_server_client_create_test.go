package helpers_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/secretoutput"
	"github.com/stretchr/testify/require"
)

func TestInitialClientSecretCapturedWithoutPublishingOrRetrying(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "uncertain"}[fail], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "initial.json")
			metadata := secretoutput.Metadata{ResourceRef: "worker"}
			output, err := secretoutput.Reserve(path, metadata)
			require.NoError(t, err)
			t.Cleanup(func() { _ = output.Close() })
			calls := 0
			transport := httpclient.NewRetryingHTTPClient(
				secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
					calls++
					require.Equal(t, http.MethodPost, request.Method)
					pending, err := os.ReadFile(path)
					require.NoError(t, err)
					require.Contains(t, string(pending), "creation_not_confirmed")
					require.Contains(
						t,
						string(pending),
						fixtureServerID,
						"resolved parent must be persisted before POST",
					)
					if fail {
						return secretResponse(request, 503, `{"detail":"issued-private"}`), nil
					}
					return secretResponse(
						request,
						201,
						`{"id":"opaque-client","name":"worker","grant_types":["client_credentials"],`+
							`"response_types":[],"client_secret":"issued-private",`+
							`"created_at":"2026-10-31T00:00:00Z","updated_at":"2026-10-31T00:00:00Z"}`,
					), nil
				}),
				httpclient.RetryConfig{
					Strategy:          "backoff",
					MaxAttempts:       3,
					InitialIntervalMS: 1,
					MaxIntervalMS:     2,
					BackoffFactor:     1,
				},
				nil,
			)
			sdk := kk.New(
				kk.WithClient(transport),
				kk.WithServerURL("https://probe.invalid"),
				kk.WithRetryConfig(retry.Config{
					Strategy: "backoff",
					Backoff:  &retry.BackoffStrategy{InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10},
				}),
			)
			value, err := helpers.CreateAndCaptureIdentityAuthServerClient(
				t.Context(),
				(&helpers.KonnectSDK{SDK: sdk}).GetIdentityAuthServerClientsAPI(),
				fixtureServerID,
				kkComps.CreateClient{
					Name:          "worker",
					GrantTypes:    []kkComps.AuthServerGrantType{kkComps.AuthServerGrantTypeClientCredentials},
					ResponseTypes: []kkComps.ResponseType{},
				},
				output,
				metadata,
			)
			require.Equal(t, 1, calls)
			data, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			if fail {
				require.ErrorContains(t, err, "unconfirmed")
				require.NotContains(t, err.Error(), "issued-private")
				require.Nil(t, value)
				require.Contains(t, string(data), "creation_not_confirmed")
			} else {
				require.NoError(t, err)
				public, err := json.Marshal(value)
				require.NoError(t, err)
				require.NotContains(t, string(public), "issued-private")
				require.Contains(t, string(data), "issued-private")
				require.Contains(t, string(data), "opaque-client")
			}
		})
	}
}
