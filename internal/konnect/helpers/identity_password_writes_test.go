package helpers_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/Kong/sdk-konnect-go/optionalnullable"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/stretchr/testify/require"
)

const (
	writeFixtureDirectory = "731559d3-39bd-48c7-b33c-eb192847eb20"
	writeFixturePrincipal = "731559d3-39bd-48c7-b33c-eb192847eb21"
	writeFixtureBasicAuth = "731559d3-39bd-48c7-b33c-eb192847eb22"
	writeFixtureID        = "731559d3-39bd-48c7-b33c-eb192847eb23"
	writeFixturePath      = "/v2/directories/" + writeFixtureDirectory + "/principals/" + writeFixturePrincipal +
		"/basic-auths/" + writeFixtureBasicAuth + "/passwords"
	writeFixtureExpiry = "2031-01-01T00:00:00Z"
)

func passwordPublicBody(expiration any) map[string]any {
	return map[string]any{
		"id": writeFixtureID, "expires_at": expiration,
		"managed_by": map[string]string{}, "created_at": "2026-10-31T00:00:00Z", "updated_at": "2026-10-31T00:00:00Z",
	}
}

func passwordWriteAPI(client kk.HTTPClient) helpers.IdentityPasswordWritesAPI {
	transport := httpclient.NewRetryingHTTPClient(client, httpclient.RetryConfig{
		Strategy: "backoff", MaxAttempts: 3, InitialIntervalMS: 1, MaxIntervalMS: 2, BackoffFactor: 1,
	}, nil)
	sdk := kk.New(kk.WithClient(transport), kk.WithRetryConfig(retry.Config{
		Strategy: "backoff", Backoff: &retry.BackoffStrategy{
			InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10,
		},
	}))
	return (&helpers.KonnectSDK{SDK: sdk}).GetIdentityPasswordWritesAPI()
}

func TestPasswordCreationCaptureAndUncertainOutcome(t *testing.T) {
	for _, mode := range []string{
		"generated", "imported", "api-failure", "missing-secret", "missing-expiration",
		"missing-management", "missing-readback", "removed-output", "invalid-id", "import-mismatch", "existing-output",
	} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "password.json")
			if mode == "existing-output" {
				require.NoError(t, os.WriteFile(path, []byte("preserve-original"), 0o600))
			}
			posts, gets := 0, 0
			api := passwordWriteAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodGet {
					gets++
					require.Equal(t, writeFixturePath+"/"+writeFixtureID, request.URL.Path)
					if mode == "missing-readback" {
						return secretResponse(request, 404, `{"secret":"never-public"}`), nil
					}
					data, err := json.Marshal(passwordPublicBody(writeFixtureExpiry))
					require.NoError(t, err)
					return secretResponse(request, 200, string(data)), nil
				}
				posts++
				require.Equal(t, http.MethodPost, request.Method)
				require.Equal(t, writeFixturePath, request.URL.Path)
				pending, err := os.ReadFile(path)
				require.NoError(t, err, "reserve private output before creating a password")
				var metadata map[string]any
				require.NoError(t, json.Unmarshal(pending, &metadata))
				require.Equal(t, "creation_not_confirmed", metadata["status"])
				require.Equal(t, writeFixtureDirectory, metadata["directory_id"])
				require.Equal(t, writeFixturePrincipal, metadata["principal_id"])
				require.Equal(t, writeFixtureBasicAuth, metadata["basic_auth_id"])
				require.NotContains(t, metadata, "auth_server_id")
				encoded, err := io.ReadAll(request.Body)
				require.NoError(t, err)
				var body map[string]any
				require.NoError(t, json.Unmarshal(encoded, &body))
				require.Equal(t, writeFixtureExpiry, body["expires_at"])
				if mode == "imported" || mode == "import-mismatch" {
					require.Equal(t, "supplied-private", body["secret"])
				} else {
					require.NotContains(t, body, "secret")
				}
				if mode == "api-failure" {
					return secretResponse(request, 503, `{"secret":"never-public"}`), nil
				}
				value := passwordPublicBody(writeFixtureExpiry)
				value["secret"] = "issued-private"
				if mode == "imported" {
					value["secret"] = "supplied-private"
				}
				if mode == "invalid-id" {
					value["id"] = "never-public"
				}
				if mode == "missing-secret" {
					delete(value, "secret")
				}
				if mode == "missing-expiration" {
					delete(value, "expires_at")
				}
				if mode == "missing-management" {
					delete(value, "managed_by")
				}
				if mode == "removed-output" {
					require.NoError(t, os.Remove(path))
				}
				data, err := json.Marshal(value)
				require.NoError(t, err)
				return secretResponse(request, 201, string(data)), nil
			}))
			expiration, err := time.Parse(time.RFC3339, writeFixtureExpiry)
			require.NoError(t, err)
			body := kkComps.CreatePasswordBody{ExpiresAt: optionalnullable.From(&expiration)}
			if mode == "imported" || mode == "import-mismatch" {
				body.Secret = new("supplied-private")
			}
			value, err := helpers.CreateAndCaptureIdentityPassword(t.Context(), api,
				writeFixtureDirectory, writeFixturePrincipal, writeFixtureBasicAuth, body, path)
			if mode == "generated" || mode == "imported" {
				require.NoError(t, err)
				require.Equal(t, writeFixtureID, value.ID)
				require.Equal(t, 1, gets)
				public, err := json.Marshal(value)
				require.NoError(t, err)
				require.NotContains(t, string(public), "private")
				captured, err := os.ReadFile(path)
				require.NoError(t, err)
				var record map[string]any
				require.NoError(t, json.Unmarshal(captured, &record))
				require.Equal(t, "captured", record["status"])
				require.Equal(t, writeFixtureID, record["password_id"])
				require.NotEmpty(t, record["secret"])
				info, err := os.Stat(path)
				require.NoError(t, err)
				require.Zero(t, info.Mode().Perm()&0o077)
			} else {
				require.Error(t, err)
				require.Nil(t, value)
				for _, material := range []string{"never-public", "issued-private", "supplied-private"} {
					require.NotContains(t, err.Error(), material)
				}
			}
			if mode == "existing-output" {
				require.Zero(t, posts)
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, "preserve-original", string(data))
			} else {
				require.Equal(t, 1, posts, "neither retry layer may replay an uncertain create")
			}
		})
	}
}

func TestPasswordExpirationAndReplacementPreserveFreshMetadata(t *testing.T) {
	for _, operation := range []string{"expiration", "replace", "replace-clear", "replace-failure", "missing-before"} {
		t.Run(operation, func(t *testing.T) {
			current := passwordPublicBody(writeFixtureExpiry)
			current["managed_by"] = map[string]string{"owner": "other-system", "keep": "unchanged"}
			gets, writes := 0, 0
			api := passwordWriteAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				require.Equal(t, writeFixturePath+"/"+writeFixtureID, request.URL.Path)
				if request.Method == http.MethodGet {
					gets++
					value := current
					if operation == "missing-before" {
						value = passwordPublicBody(writeFixtureExpiry)
						delete(value, "expires_at")
					}
					data, err := json.Marshal(value)
					require.NoError(t, err)
					return secretResponse(request, 200, string(data)), nil
				}
				writes++
				encoded, err := io.ReadAll(request.Body)
				require.NoError(t, err)
				var body map[string]any
				require.NoError(t, json.Unmarshal(encoded, &body))
				require.Contains(t, body, "expires_at")
				if operation == "expiration" {
					require.Equal(t, http.MethodPatch, request.Method)
					require.Nil(t, body["expires_at"])
					require.NotContains(t, body, "secret")
					require.NotContains(t, body, "managed_by")
				} else {
					require.Equal(t, http.MethodPut, request.Method)
					require.Equal(t, "replacement-private", body["secret"])
					require.Equal(t, map[string]any{"owner": "other-system", "keep": "unchanged"}, body["managed_by"])
					if operation == "replace-clear" {
						require.Nil(t, body["expires_at"])
					} else {
						require.Equal(t, writeFixtureExpiry, body["expires_at"])
					}
				}
				if operation == "replace-failure" {
					return secretResponse(request, 503, `{"secret":"replacement-private"}`), nil
				}
				current["expires_at"] = body["expires_at"]
				data, err := json.Marshal(current)
				require.NoError(t, err)
				return secretResponse(request, 200, string(data)), nil
			}))
			var value *helpers.IdentityPassword
			var err error
			if operation == "expiration" {
				value, err = helpers.UpdateIdentityPasswordExpiration(
					t.Context(),
					api,
					writeFixtureDirectory,
					writeFixturePrincipal,
					writeFixtureBasicAuth,
					writeFixtureID,
					nil,
				)
			} else {
				value, err = helpers.ReplaceIdentityPassword(t.Context(), api,
					writeFixtureDirectory, writeFixturePrincipal, writeFixtureBasicAuth, writeFixtureID,
					"replacement-private", nil, operation == "replace-clear")
			}
			if operation == "replace-failure" || operation == "missing-before" {
				require.Error(t, err)
				require.Nil(t, value)
				require.NotContains(t, err.Error(), "replacement-private")
				require.Equal(t, 1, gets)
			} else {
				require.NoError(t, err)
				require.Equal(t, 2, gets)
				require.Equal(t, map[string]string{"owner": "other-system", "keep": "unchanged"}, value.ManagedBy)
			}
			if operation == "missing-before" {
				require.Zero(t, writes, "do not hydrate missing expiration as null")
			} else {
				require.Equal(t, 1, writes)
			}
		})
	}
}

func TestPasswordDeletesAreScopedAndVerifyCollectionEmptiness(t *testing.T) {
	for _, operation := range []string{"selected", "all", "all-not-empty", "failed"} {
		t.Run(operation, func(t *testing.T) {
			deletes := 0
			api := passwordWriteAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				path := writeFixturePath
				if operation == "selected" {
					path += "/" + writeFixtureID
				}
				require.Equal(t, path, request.URL.Path)
				if request.Method == http.MethodDelete {
					deletes++
					if operation == "failed" {
						return secretResponse(request, 400, `{"secret":"never-public"}`), nil
					}
					return secretResponse(request, 204, ""), nil
				}
				require.Equal(t, http.MethodGet, request.Method)
				data := "[]"
				if operation == "all-not-empty" {
					encoded, err := json.Marshal(passwordPublicBody(nil))
					require.NoError(t, err)
					data = "[" + string(encoded) + "]"
				}
				return secretResponse(
					request,
					200,
					fmt.Sprintf(`{"data":%s,"meta":{"page":{"size":20,"next":null}}}`, data),
				), nil
			}))
			var err error
			if operation == "selected" {
				err = helpers.DeleteIdentityPassword(
					t.Context(),
					api,
					writeFixtureDirectory,
					writeFixturePrincipal,
					writeFixtureBasicAuth,
					writeFixtureID,
				)
			} else {
				err = helpers.DeleteAllIdentityPasswords(
					t.Context(), api, writeFixtureDirectory, writeFixturePrincipal, writeFixtureBasicAuth,
				)
			}
			if operation == "failed" || operation == "all-not-empty" {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "never-public")
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, 1, deletes)
		})
	}
}

func TestPasswordWriteValidationPrecedesCaptureAndMutation(t *testing.T) {
	api := passwordWriteAPI(secretHTTPFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid input must fail before requesting the API")
		return nil, nil
	}))
	for _, secret := range []string{"", strings.Repeat("x", 256), "invalid\xff"} {
		path := filepath.Join(t.TempDir(), "password.json")
		_, err := helpers.CreateAndCaptureIdentityPassword(t.Context(), api,
			writeFixtureDirectory, writeFixturePrincipal, writeFixtureBasicAuth,
			kkComps.CreatePasswordBody{Secret: &secret}, path)
		require.Error(t, err)
		_, err = os.Stat(path)
		require.True(t, os.IsNotExist(err))
	}
	_, err := helpers.ReplaceIdentityPassword(
		t.Context(),
		api,
		writeFixtureDirectory,
		"invalid",
		writeFixtureBasicAuth,
		writeFixtureID,
		"valid",
		nil,
		false,
	)
	require.Error(t, err)
}

func TestPasswordExpirationAndDeletesDoNotRetryUncertainWrites(t *testing.T) {
	for _, operation := range []string{"expiration", "selected", "all"} {
		t.Run(operation, func(t *testing.T) {
			writes := 0
			api := passwordWriteAPI(secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodGet {
					body, err := json.Marshal(passwordPublicBody(nil))
					require.NoError(t, err)
					return secretResponse(request, http.StatusOK, string(body)), nil
				}
				writes++
				return secretResponse(request, http.StatusServiceUnavailable, `{"detail":"private-password-value"}`), nil
			}))
			var err error
			switch operation {
			case "expiration":
				_, err = helpers.UpdateIdentityPasswordExpiration(t.Context(), api,
					writeFixtureDirectory, writeFixturePrincipal, writeFixtureBasicAuth, writeFixtureID, nil)
			case "selected":
				err = helpers.DeleteIdentityPassword(t.Context(), api,
					writeFixtureDirectory, writeFixturePrincipal, writeFixtureBasicAuth, writeFixtureID)
			case "all":
				err = helpers.DeleteAllIdentityPasswords(t.Context(), api,
					writeFixtureDirectory, writeFixturePrincipal, writeFixtureBasicAuth)
			}
			require.Error(t, err)
			require.NotContains(t, err.Error(), "private-password-value")
			require.Equal(t, 1, writes, "neither retry layer may replay an uncertain credential mutation")
		})
	}
}
