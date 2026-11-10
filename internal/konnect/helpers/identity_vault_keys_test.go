package helpers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/stretchr/testify/require"
)

const (
	vaultKeyTestDirectory = "731559d3-39bd-48c7-b33c-eb192847eb20"
	vaultKeyTestFirst     = "731559d3-39bd-48c7-b33c-eb192847eb21"
	vaultKeyTestSecond    = "731559d3-39bd-48c7-b33c-eb192847eb22"
)

type vaultKeysReaderStub struct {
	list func(kkOps.ListVaultKeysRequest) (*kkOps.ListVaultKeysResponse, error)
	get  func(string, string) (*kkOps.GetVaultKeyResponse, error)
}

func (s vaultKeysReaderStub) ListVaultKeys(_ context.Context, request kkOps.ListVaultKeysRequest,
	_ ...kkOps.Option,
) (*kkOps.ListVaultKeysResponse, error) {
	return s.list(request)
}

func (s vaultKeysReaderStub) GetVaultKey(_ context.Context, directory, id string, _ ...kkOps.Option,
) (*kkOps.GetVaultKeyResponse, error) {
	return s.get(directory, id)
}

func vaultKeyTestValues(t *testing.T) []IdentityVaultKey {
	t.Helper()
	data, err := os.ReadFile("../identitykeys/testdata/public-keys.json")
	require.NoError(t, err)
	var fixtures []struct {
		PublicKey string `json:"public_key"`
		Kid       string `json:"kid"`
	}
	require.NoError(t, json.Unmarshal(data, &fixtures))
	values := make([]IdentityVaultKey, 2)
	for i, id := range []string{vaultKeyTestFirst, vaultKeyTestSecond} {
		record, err := json.Marshal(map[string]any{
			"id": id, "directory_id": vaultKeyTestDirectory, "name": "nonunique-name",
			"public_key": fixtures[i].PublicKey, "kid": fixtures[i].Kid, "algorithm": "RSA", "key_size": 2048,
			"created_at": "2026-10-31T12:00:00Z", "updated_at": "2026-10-31T12:00:00Z",
		})
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(record, &values[i]))
	}
	return values
}

func vaultKeyHTTP(t *testing.T, value any) *http.Response {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data)))}
}

func vaultKeyPage(values []IdentityVaultKey, next any) map[string]any {
	return map[string]any{"data": values, "meta": map[string]any{"page": map[string]any{"size": 1, "next": next}}}
}

func TestVaultKeysCompleteFingerprintLookupWithDuplicateNames(t *testing.T) {
	values := vaultKeyTestValues(t)
	calls, gets := 0, 0
	api := vaultKeysReaderStub{
		list: func(request kkOps.ListVaultKeysRequest) (*kkOps.ListVaultKeysResponse, error) {
			calls++
			require.Equal(t, vaultKeyTestDirectory, request.DirectoryID)
			require.EqualValues(t, 1, *request.Page.Size)
			require.Nil(t, request.Sort)
			if calls == 1 {
				require.Nil(t, request.Page.After)
				return &kkOps.ListVaultKeysResponse{RawResponse: vaultKeyHTTP(t, vaultKeyPage(values[:1],
					"/v2/directories/"+request.DirectoryID+"/vault/keys?page%5Bafter%5D=cursor%2B%2F%3D%3D"))}, nil
			}
			require.Equal(t, "cursor+/==", *request.Page.After)
			return &kkOps.ListVaultKeysResponse{RawResponse: vaultKeyHTTP(t, vaultKeyPage(values[1:], nil))}, nil
		},
		get: func(directory, id string) (*kkOps.GetVaultKeyResponse, error) {
			gets++
			require.Equal(t, vaultKeyTestDirectory, directory)
			require.Equal(t, vaultKeyTestSecond, id)
			return &kkOps.GetVaultKeyResponse{RawResponse: vaultKeyHTTP(t, values[1])}, nil
		},
	}
	value, err := ResolveIdentityVaultKeyKid(t.Context(), api, vaultKeyTestDirectory, values[1].Kid, 1)
	require.NoError(t, err)
	require.Equal(t, values[1], *value)
	require.Equal(t, 2, calls)
	require.Equal(t, 1, gets)
}

func TestVaultKeyInventoriesRejectInvalidOrAmbiguousObservations(t *testing.T) {
	values := vaultKeyTestValues(t)
	valid, err := json.Marshal(values[0])
	require.NoError(t, err)
	for _, field := range []string{
		"id", "directory_id", "name", "public_key", "kid", "algorithm", "key_size",
		"created_at", "updated_at",
	} {
		t.Run("missing-"+field, func(t *testing.T) {
			var missing map[string]any
			require.NoError(t, json.Unmarshal(valid, &missing))
			delete(missing, field)
			response := map[string]any{"data": []any{missing}, "meta": vaultKeyPage(nil, nil)["meta"]}
			api := vaultKeysReaderStub{list: func(kkOps.ListVaultKeysRequest) (*kkOps.ListVaultKeysResponse, error) {
				return &kkOps.ListVaultKeysResponse{RawResponse: vaultKeyHTTP(t, response)}, nil
			}}
			got, err := ListIdentityVaultKeys(t.Context(), api, vaultKeyTestDirectory, 1)
			require.Error(t, err)
			require.Nil(t, got)
		})
	}
	for _, field := range []string{"id", "directory_id", "kid", "algorithm", "key_size"} {
		t.Run("inconsistent-"+field, func(t *testing.T) {
			var altered map[string]any
			require.NoError(t, json.Unmarshal(valid, &altered))
			altered[field] = "incorrect"
			if field == "directory_id" {
				altered[field] = vaultKeyTestSecond
			}
			api := vaultKeysReaderStub{list: func(kkOps.ListVaultKeysRequest) (*kkOps.ListVaultKeysResponse, error) {
				return &kkOps.ListVaultKeysResponse{RawResponse: vaultKeyHTTP(t, map[string]any{
					"data": []any{altered}, "meta": vaultKeyPage(nil, nil)["meta"],
				})}, nil
			}}
			got, err := ListIdentityVaultKeys(t.Context(), api, vaultKeyTestDirectory, 1)
			require.Error(t, err)
			require.Nil(t, got)
		})
	}
	for _, response := range []any{
		map[string]any{},
		map[string]any{"data": []any{}},
		vaultKeyPage(nil, nil),
		map[string]any{"data": []any{}, "meta": map[string]any{"page": map[string]any{"size": 1}}},
		map[string]any{"data": []any{}, "meta": map[string]any{"page": map[string]any{"size": 1.5, "next": nil}}},
		map[string]any{"data": []any{}, "meta": map[string]any{"page": map[string]any{"size": 0, "next": nil}}},
		vaultKeyPage(values[:1], 42), vaultKeyPage([]IdentityVaultKey{values[0], values[0]}, nil),
	} {
		api := vaultKeysReaderStub{list: func(kkOps.ListVaultKeysRequest) (*kkOps.ListVaultKeysResponse, error) {
			return &kkOps.ListVaultKeysResponse{RawResponse: vaultKeyHTTP(t, response)}, nil
		}}
		got, err := ListIdentityVaultKeys(t.Context(), api, vaultKeyTestDirectory, 1)
		require.Error(t, err)
		require.Nil(t, got)
	}
	// Different route IDs cannot make repeated key material unambiguous.
	duplicate := values[0]
	duplicate.ID = values[1].ID
	api := vaultKeysReaderStub{list: func(kkOps.ListVaultKeysRequest) (*kkOps.ListVaultKeysResponse, error) {
		return &kkOps.ListVaultKeysResponse{RawResponse: vaultKeyHTTP(t,
			vaultKeyPage([]IdentityVaultKey{values[0], duplicate}, nil))}, nil
	}}
	got, err := ListIdentityVaultKeys(t.Context(), api, vaultKeyTestDirectory, 1)
	require.ErrorContains(t, err, "repeated identity")
	require.Nil(t, got)
}

func TestVaultKeyReaderEmptyAndUnavailableScopes(t *testing.T) {
	api := vaultKeysReaderStub{
		list: func(kkOps.ListVaultKeysRequest) (*kkOps.ListVaultKeysResponse, error) {
			return &kkOps.ListVaultKeysResponse{
				RawResponse: vaultKeyHTTP(t, vaultKeyPage([]IdentityVaultKey{}, nil)),
			}, nil
		},
	}
	values, err := ListIdentityVaultKeys(t.Context(), api, vaultKeyTestDirectory, 0)
	require.NoError(t, err)
	require.NotNil(t, values)
	require.Empty(t, values)
	_, err = ResolveIdentityVaultKeyKid(t.Context(), api, vaultKeyTestDirectory, "missing", 1)
	require.ErrorContains(t, err, "not found")
	_, err = ResolveIdentityVaultKeyKid(t.Context(), api, vaultKeyTestDirectory, "", 1)
	require.ErrorContains(t, err, "must not be empty")
	_, err = ListIdentityVaultKeys(t.Context(), nil, vaultKeyTestDirectory, 1)
	require.ErrorContains(t, err, "not configured")
	_, err = ListIdentityVaultKeys(t.Context(), api, "invalid", 1)
	require.ErrorContains(t, err, "UUID")
	_, err = GetIdentityVaultKey(t.Context(), api, vaultKeyTestDirectory, "invalid")
	require.ErrorContains(t, err, "UUID")
	api.list = func(kkOps.ListVaultKeysRequest) (*kkOps.ListVaultKeysResponse, error) { return nil, nil }
	_, err = ListIdentityVaultKeys(t.Context(), api, vaultKeyTestDirectory, 1)
	require.ErrorContains(t, err, "missing")
	api.get = func(string, string) (*kkOps.GetVaultKeyResponse, error) { return nil, nil }
	_, err = GetIdentityVaultKey(t.Context(), api, vaultKeyTestDirectory, vaultKeyTestFirst)
	require.ErrorContains(t, err, "missing")
}

func TestVaultKeyLookupRejectsChangedDetailAndDoesNotReturnPartialCollections(t *testing.T) {
	values := vaultKeyTestValues(t)
	api := vaultKeysReaderStub{
		list: func(kkOps.ListVaultKeysRequest) (*kkOps.ListVaultKeysResponse, error) {
			return &kkOps.ListVaultKeysResponse{RawResponse: vaultKeyHTTP(t, vaultKeyPage(values[:1], nil))}, nil
		},
		get: func(string, string) (*kkOps.GetVaultKeyResponse, error) {
			changed := values[1]
			changed.ID = values[0].ID
			return &kkOps.GetVaultKeyResponse{RawResponse: vaultKeyHTTP(t, changed)}, nil
		},
	}
	got, err := ResolveIdentityVaultKeyKid(t.Context(), api, vaultKeyTestDirectory, values[0].Kid, 1)
	require.ErrorContains(t, err, "identity changed")
	require.Nil(t, got)
	api.get = func(string, string) (*kkOps.GetVaultKeyResponse, error) {
		return &kkOps.GetVaultKeyResponse{RawResponse: vaultKeyHTTP(t, values[1])}, nil
	}
	got, err = GetIdentityVaultKey(t.Context(), api, vaultKeyTestDirectory, values[0].ID)
	require.ErrorContains(t, err, "differs from the selected")
	require.Nil(t, got)
	calls := 0
	api.list = func(kkOps.ListVaultKeysRequest) (*kkOps.ListVaultKeysResponse, error) {
		calls++
		if calls == 1 {
			return &kkOps.ListVaultKeysResponse{RawResponse: vaultKeyHTTP(t, vaultKeyPage(values[:1], "cursor"))}, nil
		}
		return nil, &sdkerrors.SDKError{StatusCode: 403, Body: "sensitive-server-detail"}
	}
	list, err := ListIdentityVaultKeys(t.Context(), api, vaultKeyTestDirectory, 1)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "sensitive-server-detail")
	require.Nil(t, list)
	api.list = func(kkOps.ListVaultKeysRequest) (*kkOps.ListVaultKeysResponse, error) {
		return &kkOps.ListVaultKeysResponse{RawResponse: vaultKeyHTTP(t,
			vaultKeyPage([]IdentityVaultKey{}, "same-cursor"))}, nil
	}
	list, err = ListIdentityVaultKeys(t.Context(), api, vaultKeyTestDirectory, 1)
	require.ErrorContains(t, err, "paginate")
	require.Nil(t, list)
}
