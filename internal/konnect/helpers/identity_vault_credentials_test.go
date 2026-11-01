package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

type vaultCredentialReaderStub struct {
	provider  func(kkOps.ListVaultProviderCredentialsRequest) (*kkOps.ListVaultProviderCredentialsResponse, error)
	principal func(kkOps.ListVaultCredentialsRequest) (*kkOps.ListVaultCredentialsResponse, error)
}

func (s vaultCredentialReaderStub) ListVaultProviderCredentials(
	_ context.Context,
	r kkOps.ListVaultProviderCredentialsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListVaultProviderCredentialsResponse, error) {
	return s.provider(r)
}

func (s vaultCredentialReaderStub) ListVaultCredentials(_ context.Context, r kkOps.ListVaultCredentialsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListVaultCredentialsResponse, error) {
	return s.principal(r)
}

func vaultCredentialRecord(id string, user bool) map[string]any {
	value := map[string]any{
		"id": id, "credential_type": "shared", "status": "pending", "selector": nil, "priority": nil,
		"created_at": "2026-10-31T12:00:00Z", "updated_at": "2026-10-31T12:00:01Z",
		"secret": "private-fixture-value", "enrollment_url": "https://private.example.test/?token=private-fixture-value",
	}
	if user {
		value["credential_type"] = "user"
		value["status"] = "active"
		value["expires_at"] = nil
		value["principal"] = map[string]any{"id": providerTestDirectory, "display_name": "Fixture subject"}
		delete(value, "selector")
		delete(value, "priority")
	}
	return value
}

func TestVaultCredentialListsPreserveVariantsFollowEveryPageAndExcludeCapabilities(t *testing.T) {
	calls := 0
	api := vaultCredentialReaderStub{
		provider: func(r kkOps.ListVaultProviderCredentialsRequest) (*kkOps.ListVaultProviderCredentialsResponse, error) {
			require.Equal(t, providerTestDirectory, r.DirectoryID)
			require.Equal(t, providerTestFirst, r.ProviderID)
			require.NotNil(t, r.Page)
			require.EqualValues(t, 1, *r.Page.Size)
			calls++
			value, next := vaultCredentialRecord(providerTestFirst, false), any("next-page")
			if r.Page.After != nil {
				require.Equal(t, "next-page", *r.Page.After)
				value, next = vaultCredentialRecord(providerTestSecond, true), nil
			} else {
				value["selector"] = "true"
				value["priority"] = 0
			}
			return &kkOps.ListVaultProviderCredentialsResponse{
				StatusCode:  200,
				RawResponse: providerHTTP(t, providerPage([]any{value}, next)),
			}, nil
		},
	}
	values, err := ListIdentityVaultProviderCredentials(t.Context(), api, providerTestDirectory, providerTestFirst, 1)
	require.NoError(t, err)
	require.Len(t, values, 2)
	require.Equal(t, 2, calls)
	require.EqualValues(t, 0, *values[0].Priority)
	require.Equal(t, "true", *values[0].Selector)
	require.Equal(t, "Fixture subject", values[1].Principal.DisplayName)
	for _, encode := range []func(any) ([]byte, error){json.Marshal, yaml.Marshal} {
		data, err := encode(values)
		require.NoError(t, err)
		require.NotContains(t, string(data), "private-fixture-value")
		require.NotContains(t, string(data), "enrollment_url")
		require.NotContains(t, string(data), "secret")
	}
	shared, err := json.Marshal(values[0])
	require.NoError(t, err)
	require.NotContains(t, string(shared), "principal")
	require.NotContains(t, string(shared), "expires_at")
	user, err := json.Marshal(values[1])
	require.NoError(t, err)
	require.NotContains(t, string(user), "selector")
	require.NotContains(t, string(user), "priority")
	require.Contains(t, string(user), `"expires_at":null`)
}

func TestVaultCredentialListsRejectIncompleteFailedOrRepeatedInventories(t *testing.T) {
	for _, mode := range []string{
		"nil", "error", "status", "null-data", "missing-next", "fractional", "duplicate", "cursor",
		"bad-id", "missing-time", "unknown-type", "bad-routing", "missing-principal",
	} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			api := vaultCredentialReaderStub{
				provider: func(_ kkOps.ListVaultProviderCredentialsRequest) (*kkOps.ListVaultProviderCredentialsResponse, error) {
					calls++
					if mode == "nil" {
						return nil, nil
					}
					if mode == "error" {
						return nil, &sdkerrors.SDKError{StatusCode: 403, Body: "private-fixture-value"}
					}
					value := vaultCredentialRecord(providerTestFirst, false)
					page := providerPage([]any{value}, nil)
					switch mode {
					case "null-data":
						page["data"] = nil
					case "missing-next":
						page["meta"] = map[string]any{"page": map[string]any{"size": 1}}
					case "fractional":
						page["meta"] = map[string]any{"page": map[string]any{"size": 1.5, "next": nil}}
					case "duplicate":
						page = providerPage([]any{value}, "next-page")
					case "cursor":
						value["id"] = fmt.Sprintf("731559d3-39bd-48c7-b33c-%012d", calls)
						page = providerPage([]any{value}, "next-page")
					case "bad-id":
						value["id"] = "invalid"
					case "missing-time":
						delete(value, "created_at")
					case "unknown-type":
						value["credential_type"] = "future"
					case "bad-routing":
						value["priority"] = 0
					case "missing-principal":
						value["credential_type"] = "user"
					}
					status := 200
					if mode == "status" {
						status = 403
					}
					return &kkOps.ListVaultProviderCredentialsResponse{
						StatusCode:  status,
						RawResponse: providerHTTP(t, page),
					}, nil
				},
			}
			values, err := ListIdentityVaultProviderCredentials(
				t.Context(),
				api,
				providerTestDirectory,
				providerTestFirst,
				1,
			)
			require.Error(t, err)
			require.Nil(t, values)
			require.LessOrEqual(t, calls, 2)
			require.NotContains(t, err.Error(), "private-fixture-value")
		})
	}
}

func TestPrincipalVaultCredentialMetadataAndEmptyCollection(t *testing.T) {
	for _, empty := range []bool{false, true} {
		api := vaultCredentialReaderStub{
			principal: func(r kkOps.ListVaultCredentialsRequest) (*kkOps.ListVaultCredentialsResponse, error) {
				require.Equal(t, providerTestDirectory, r.DirectoryID)
				require.Equal(t, providerTestFirst, r.PrincipalID)
				require.EqualValues(t, 1, *r.Page.Size)
				values := []any{}
				if !empty {
					values = append(values, map[string]any{
						"id": providerTestSecond,
						"provider": map[string]string{
							"id":   providerTestDirectory,
							"name": "github-work",
						}, "template_name": "github",
						"status": "active", "expires_at": nil, "created_at": "2026-10-31T12:00:00Z", "updated_at": "2026-10-31T12:00:00Z",
						"token": "private-fixture-value", "enrollment_url": "private-fixture-value",
					})
				}
				return &kkOps.ListVaultCredentialsResponse{
					StatusCode:  200,
					RawResponse: providerHTTP(t, providerPage(values, nil)),
				}, nil
			},
		}
		values, err := ListIdentityPrincipalVaultCredentials(
			t.Context(),
			api,
			providerTestDirectory,
			providerTestFirst,
			1,
		)
		require.NoError(t, err)
		require.NotNil(t, values)
		if empty {
			require.Empty(t, values)
		} else {
			require.Len(t, values, 1)
			require.Equal(t, "github-work", values[0].Provider.Name)
			data, err := json.Marshal(values)
			require.NoError(t, err)
			require.NotContains(t, string(data), "private-fixture-value")
		}
	}
	_, err := ListIdentityVaultProviderCredentials(t.Context(), nil, providerTestDirectory, providerTestFirst, 1)
	require.Error(t, err)
	_, err = ListIdentityPrincipalVaultCredentials(t.Context(), nil, providerTestDirectory, providerTestFirst, 1)
	require.Error(t, err)
}
