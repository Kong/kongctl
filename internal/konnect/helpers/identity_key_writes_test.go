package helpers_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kk "github.com/Kong/sdk-konnect-go"
	"github.com/Kong/sdk-konnect-go/optionalnullable"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/secretoutput"
	"github.com/stretchr/testify/require"
)

func keyWriteClient(t *testing.T, group bool, handler secretHTTPFunc) *helpers.IdentityAPIKeyClient {
	t.Helper()
	transport := httpclient.NewRetryingHTTPClient(handler, httpclient.RetryConfig{
		Strategy: "backoff", MaxAttempts: 3, InitialIntervalMS: 1, MaxIntervalMS: 2, BackoffFactor: 1,
	}, nil)
	sdk := &helpers.KonnectSDK{SDK: kk.New(kk.WithClient(transport), kk.WithRetryConfig(retry.Config{
		Strategy: "backoff", Backoff: &retry.BackoffStrategy{
			InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10,
		},
	}))}
	var client *helpers.IdentityAPIKeyClient
	var err error
	if group {
		client, err = helpers.NewGroupAPIKeyClient(
			sdk.GetIdentityGroupKeysAPI(), writeFixtureDirectory, writeFixturePrincipal,
		)
	} else {
		client, err = helpers.NewPrincipalAPIKeyClient(
			sdk.GetIdentityPrincipalKeysAPI(), writeFixtureDirectory, writeFixturePrincipal,
		)
	}
	require.NoError(t, err)
	return client
}

func keyWritePath(group bool) string {
	parent := "principals"
	if group {
		parent = "groups"
	}
	return "/v2/directories/" + writeFixtureDirectory + "/" + parent + "/" + writeFixturePrincipal + "/api-keys"
}

func TestAPIKeyCreationCapturesBeforeVerificationWithoutReplaying(t *testing.T) {
	for _, group := range []bool{false, true} {
		for _, mode := range []string{
			"generated", "generated-managed", "imported", "failed", "transport", "missing-secret",
			"missing-expiration", "missing-map", "wrong-map", "invalid-id", "read-back", "existing-output", "import-mismatch",
		} {
			t.Run(keyWritePath(group)+"/"+mode, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "key.json")
				if mode == "existing-output" {
					require.NoError(t, os.WriteFile(path, []byte("untouched"), 0o600))
				}
				posts := 0
				public := passwordPublicBody(writeFixtureExpiry)
				public["name"] = nil
				ownership := map[string]string{
					"tool": "kongctl", "version": "test", "kongctl-namespace": "keys",
					"kongctl-ref": "primary", "kongctl-protected": "false",
				}
				if mode == "generated-managed" {
					public["managed_by"] = ownership
				}
				client := keyWriteClient(t, group, func(request *http.Request) (*http.Response, error) {
					if request.Method == http.MethodGet {
						require.Equal(t, keyWritePath(group)+"/"+writeFixtureID, request.URL.Path)
						if mode == "read-back" {
							return secretResponse(request, 404, `{"detail":"private-key-value"}`), nil
						}
						data, err := json.Marshal(public)
						require.NoError(t, err)
						return secretResponse(request, 200, string(data)), nil
					}
					posts++
					require.Equal(t, http.MethodPost, request.Method)
					require.Equal(t, keyWritePath(group), request.URL.Path)
					pending, err := os.ReadFile(path)
					require.NoError(t, err)
					require.Contains(t, string(pending), "creation_not_confirmed")
					body, err := io.ReadAll(request.Body)
					require.NoError(t, err)
					var sent map[string]any
					require.NoError(t, json.Unmarshal(body, &sent))
					if mode == "generated-managed" {
						require.Len(t, sent["managed_by"], 5)
					}
					require.Equal(t, writeFixtureExpiry, sent["expires_at"])
					if mode == "imported" || mode == "import-mismatch" {
						require.Equal(t, "imported", sent["type"])
						require.Equal(t, "private-key-value", sent["secret"])
					} else {
						require.Equal(t, "v1", sent["type"])
						require.NotContains(t, sent, "secret")
					}
					if mode == "failed" {
						return secretResponse(request, 503, `{"detail":"private-key-value"}`), nil
					}
					if mode == "transport" {
						return nil, errors.New("private-key-value")
					}
					issued := passwordPublicBody(writeFixtureExpiry)
					issued["name"], issued["secret"] = nil, "private-key-value"
					if mode == "generated-managed" {
						issued["managed_by"] = ownership
					}
					switch mode {
					case "missing-secret":
						delete(issued, "secret")
					case "missing-expiration":
						delete(issued, "expires_at")
					case "missing-map":
						delete(issued, "managed_by")
					case "wrong-map":
						issued["managed_by"] = map[string]string{"unexpected": "owner"}
					case "invalid-id":
						issued["id"] = "bad-id"
					case "import-mismatch":
						issued["secret"] = "different-private-key"
					}
					data, err := json.Marshal(issued)
					require.NoError(t, err)
					return secretResponse(request, 201, string(data)), nil
				})
				expiration, err := time.Parse(time.RFC3339, writeFixtureExpiry)
				require.NoError(t, err)
				options := helpers.IdentityAPIKeyCreate{ExpiresAt: optionalnullable.From(&expiration)}
				if mode == "generated-managed" {
					options.ManagedBy = ownership
				}
				if mode == "imported" || mode == "import-mismatch" {
					options.Secret = new("private-key-value")
				}
				value, err := client.CreateAndCapture(t.Context(), options, path)
				if mode == "generated" || mode == "generated-managed" || mode == "imported" {
					require.NoError(t, err)
					require.Equal(t, writeFixtureID, value.ID)
					data, err := json.Marshal(value)
					require.NoError(t, err)
					require.NotContains(t, string(data), "private-key")
				} else {
					require.Error(t, err)
					require.Nil(t, value)
					require.NotContains(t, err.Error(), "private-key-value")
					require.NotContains(t, err.Error(), "different-private-key")
				}
				if mode == "existing-output" {
					require.Zero(t, posts)
					data, err := os.ReadFile(path)
					require.NoError(t, err)
					require.Equal(t, "untouched", string(data))
					return
				}
				require.Equal(t, 1, posts)
				info, err := os.Stat(path)
				require.NoError(t, err)
				require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				var capture map[string]any
				require.NoError(t, json.Unmarshal(data, &capture))
				switch mode {
				case "failed", "transport":
					require.Equal(t, "creation_not_confirmed", capture["status"])
				case "missing-secret":
					require.Equal(t, "secret_missing", capture["status"])
				default:
					require.Equal(t, "captured", capture["status"])
					require.NotEmpty(t, capture["secret"])
				}
				if group {
					require.Equal(t, writeFixturePrincipal, capture["group_id"])
					require.NotContains(t, capture, "principal_id")
				} else {
					require.Equal(t, writeFixturePrincipal, capture["principal_id"])
					require.NotContains(t, capture, "group_id")
				}
			})
		}
	}
}

func TestAPIKeyUpdatesPreserveExpirationAndOwnership(t *testing.T) {
	for _, group := range []bool{false, true} {
		for _, mode := range []string{"expiration", "clear", "rename", "ignored", "missing-expiration", "failed"} {
			t.Run(keyWritePath(group)+"/"+mode, func(t *testing.T) {
				public := passwordPublicBody(writeFixtureExpiry)
				public["name"] = "old"
				public["managed_by"] = map[string]string{"tool": "other", "protected": "yes"}
				patches := 0
				client := keyWriteClient(t, group, func(request *http.Request) (*http.Response, error) {
					require.Equal(t, keyWritePath(group)+"/"+writeFixtureID, request.URL.Path)
					if mode == "missing-expiration" {
						delete(public, "expires_at")
					}
					if request.Method == http.MethodPatch {
						patches++
						body, err := io.ReadAll(request.Body)
						require.NoError(t, err)
						var sent map[string]any
						require.NoError(t, json.Unmarshal(body, &sent))
						require.NotContains(t, sent, "managed_by")
						require.NotContains(t, sent, "secret")
						if mode == "rename" {
							require.Equal(t, writeFixtureExpiry, sent["expires_at"])
							public["name"] = sent["name"]
						} else if mode != "ignored" {
							public["expires_at"] = sent["expires_at"]
						}
						if mode == "failed" {
							return secretResponse(request, 503, `{"detail":"private-key-value"}`), nil
						}
					}
					body, err := json.Marshal(public)
					require.NoError(t, err)
					return secretResponse(request, 200, string(body)), nil
				})
				expiration, err := time.Parse(time.RFC3339, "2032-01-01T00:00:00Z")
				require.NoError(t, err)
				options := helpers.IdentityAPIKeyUpdate{ExpiresAt: optionalnullable.From(&expiration)}
				if mode == "clear" {
					options.ExpiresAt = optionalnullable.From[time.Time](nil)
				}
				if mode == "rename" {
					options = helpers.IdentityAPIKeyUpdate{Name: new("new")}
				}
				value, err := client.Update(t.Context(), writeFixtureID, options)
				if mode == "ignored" || mode == "missing-expiration" || mode == "failed" || group && mode == "rename" {
					require.Error(t, err)
					require.Nil(t, value)
					require.NotContains(t, err.Error(), "private-key-value")
				} else {
					require.NoError(t, err)
					require.Equal(t, map[string]string{"tool": "other", "protected": "yes"}, value.ManagedBy)
				}
				if mode == "missing-expiration" || group && mode == "rename" {
					require.Zero(t, patches)
				} else {
					require.Equal(t, 1, patches)
				}
			})
		}
	}
}

func TestAPIKeyUnsupportedCreatesDoNotReserveOrIssue(t *testing.T) {
	for _, options := range []helpers.IdentityAPIKeyCreate{
		{Secret: new("supplied"), ManagedBy: map[string]string{"tool": "kongctl"}},
		{Secret: new("supplied"), Name: new("ignored-group-name")},
		{Secret: new("")},
		{Secret: new(strings.Repeat("x", 256))},
	} {
		client := keyWriteClient(t, true, func(*http.Request) (*http.Response, error) {
			t.Fatal("unsupported input must fail before any API request")
			return nil, nil
		})
		path := filepath.Join(t.TempDir(), "key.json")
		value, err := client.CreateAndCapture(t.Context(), options, path)
		require.Error(t, err)
		require.Nil(t, value)
		_, err = os.Stat(path)
		require.True(t, os.IsNotExist(err))
	}
}

func TestAPIKeyCreationAcceptsWholePlanCaptureReservation(t *testing.T) {
	for _, group := range []bool{false, true} {
		for _, mode := range []string{"captured", "closed", "replaced", "unsupported-import", "missing-output"} {
			t.Run(keyWritePath(group)+"/"+mode, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "key.json")
				local := secretoutput.Metadata{
					ResourceRef: "primary", Namespace: "parent-namespace", ParentRef: "worker",
					DirectoryID: "unresolved", PrincipalID: "unresolved", GroupID: "unresolved",
					APIKeyID: "unresolved", AuthServerID: "must-not-persist",
				}
				output, err := secretoutput.Reserve(path, local)
				require.NoError(t, err)
				defer func() { _ = output.Close() }()
				if mode == "closed" {
					require.NoError(t, output.Close())
				}
				if mode == "replaced" {
					require.NoError(t, os.Rename(path, path+".original"))
					require.NoError(t, os.WriteFile(path, []byte("leave-unrelated"), 0o600))
				}
				posts := 0
				metadata := map[string]string{
					"tool": "kongctl", "version": "creator", "kongctl-ref": "primary", "kongctl-create-type": "v1",
				}
				client := keyWriteClient(t, group, func(request *http.Request) (*http.Response, error) {
					public := passwordPublicBody(nil)
					public["managed_by"] = metadata
					if request.Method == http.MethodPost {
						posts++
						pending, err := os.ReadFile(path)
						require.NoError(t, err)
						var observed map[string]any
						require.NoError(t, json.Unmarshal(pending, &observed))
						require.Equal(t, "creation_not_confirmed", observed["status"])
						require.Equal(t, writeFixtureDirectory, observed["directory_id"])
						require.Equal(t, "primary", observed["resource_ref"])
						require.Equal(t, "parent-namespace", observed["namespace"])
						require.Equal(t, "worker", observed["parent_ref"])
						require.NotContains(t, observed, "api_key_id")
						require.NotContains(t, observed, "auth_server_id")
						parent, other := "principal_id", "group_id"
						if group {
							parent, other = other, parent
						}
						require.Equal(t, writeFixturePrincipal, observed[parent])
						require.NotContains(t, observed, other)
						public["secret"] = "reserved-private"
					} else {
						require.Equal(t, http.MethodGet, request.Method)
						captured, err := os.ReadFile(path)
						require.NoError(t, err)
						require.Contains(t, string(captured), "reserved-private", "capture must precede read-back")
					}
					body, err := json.Marshal(public)
					require.NoError(t, err)
					status := 200
					if request.Method == http.MethodPost {
						status = 201
					}
					return secretResponse(request, status, string(body)), nil
				})
				options := helpers.IdentityAPIKeyCreate{ManagedBy: metadata}
				if mode == "unsupported-import" {
					options.Secret = new("supplied")
				}
				capture := output
				if mode == "missing-output" {
					capture = nil
				}
				value, err := client.CreateWithReservedCapture(t.Context(), options, capture, local)
				if mode == "captured" {
					require.NoError(t, err)
					require.Equal(t, writeFixtureID, value.ID)
					require.Equal(t, metadata, value.ManagedBy)
					data, err := json.Marshal(value)
					require.NoError(t, err)
					require.NotContains(t, string(data), "reserved-private")
					_, err = client.CreateWithReservedCapture(t.Context(), options, capture, local)
					require.Error(t, err, "a completed reservation cannot issue another key")
					require.Equal(t, 1, posts)
					return
				}
				require.Error(t, err)
				require.Nil(t, value)
				require.Zero(t, posts)
				switch mode {
				case "replaced":
					data, err := os.ReadFile(path)
					require.NoError(t, err)
					require.Equal(t, "leave-unrelated", string(data))
				case "unsupported-import", "missing-output":
					require.NoError(t, output.Discard(), "unused reservations remain owned by preflight")
					_, err = os.Stat(path)
					require.True(t, os.IsNotExist(err))
				}
			})
		}
	}
}

func TestAPIKeyDeletionRequiresCollectionMembershipAndVerifiesAbsence(t *testing.T) {
	for _, group := range []bool{false, true} {
		for _, mode := range []string{"deleted", "not-member", "failed-page", "still-present", "failed-delete"} {
			t.Run(keyWritePath(group)+"/"+mode, func(t *testing.T) {
				deletes := 0
				client := keyWriteClient(t, group, func(request *http.Request) (*http.Response, error) {
					if request.Method == http.MethodDelete {
						deletes++
						require.Equal(t, keyWritePath(group)+"/"+writeFixtureID, request.URL.Path)
						if mode == "failed-delete" {
							return secretResponse(request, 503, `{"detail":"private-key-value"}`), nil
						}
						return secretResponse(request, 204, ""), nil
					}
					if strings.HasSuffix(request.URL.Path, "/"+writeFixtureID) {
						body, err := json.Marshal(passwordPublicBody(nil))
						require.NoError(t, err)
						return secretResponse(request, 200, string(body)), nil
					}
					require.Equal(t, keyWritePath(group), request.URL.Path)
					if mode == "failed-page" {
						return secretResponse(request, 404, `{"detail":"private-key-value"}`), nil
					}
					values := []any{}
					if mode != "not-member" && (deletes == 0 || mode == "still-present") {
						values = append(values, passwordPublicBody(nil))
					}
					return identityTestCollection(t, request, values, nil), nil
				})
				err := client.Delete(t.Context(), writeFixtureID)
				if mode == "deleted" {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.NotContains(t, err.Error(), "private-key-value")
				}
				if mode == "not-member" || mode == "failed-page" {
					require.Zero(t, deletes)
				} else {
					require.Equal(t, 1, deletes)
				}
			})
		}
	}
}
