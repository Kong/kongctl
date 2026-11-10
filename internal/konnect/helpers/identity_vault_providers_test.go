package helpers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

const (
	providerTestDirectory = "731559d3-39bd-48c7-b33c-eb192847eb20"
	providerTestFirst     = "731559d3-39bd-48c7-b33c-eb192847eb21"
	providerTestSecond    = "731559d3-39bd-48c7-b33c-eb192847eb22"
)

type vaultProvidersReaderStub struct {
	list      func(kkOps.ListVaultProvidersRequest) (*kkOps.ListVaultProvidersResponse, error)
	get       func(string, string) (*kkOps.GetVaultProviderResponse, error)
	templates func(*kkComps.CursorPageParameters) (*kkOps.ListCredentialProviderTemplatesResponse, error)
	template  func(kkComps.ProviderTemplateName) (*kkOps.GetCredentialProviderTemplateResponse, error)
}

func (s vaultProvidersReaderStub) ListVaultProviders(_ context.Context, request kkOps.ListVaultProvidersRequest,
	_ ...kkOps.Option,
) (*kkOps.ListVaultProvidersResponse, error) {
	return s.list(request)
}

func (s vaultProvidersReaderStub) GetVaultProvider(_ context.Context, directory, id string, _ ...kkOps.Option,
) (*kkOps.GetVaultProviderResponse, error) {
	return s.get(directory, id)
}

func (s vaultProvidersReaderStub) ListCredentialProviderTemplates(_ context.Context, page *kkComps.CursorPageParameters,
	_ ...kkOps.Option,
) (*kkOps.ListCredentialProviderTemplatesResponse, error) {
	return s.templates(page)
}

func (s vaultProvidersReaderStub) GetCredentialProviderTemplate(_ context.Context, name kkComps.ProviderTemplateName,
	_ ...kkOps.Option,
) (*kkOps.GetCredentialProviderTemplateResponse, error) {
	return s.template(name)
}

func providerTestRecord(id, name string) map[string]any {
	const hidden = "must-not-appear"
	return map[string]any{ // #nosec G101 -- synthetic response-only fields verify secret exclusion.
		"id": id, "directory_id": providerTestDirectory, "name": name, "template_name": "custom-oauth",
		"bearer_methods_supported": []string{"header"}, "enabled": false, "client_id": nil,
		"scopes": []string{"read"}, "token_endpoint_effective": "https://provider.example.test/token",
		"authorization_endpoint_effective": "https://provider.example.test/authorize",
		"registration_endpoint_effective":  "https://provider.example.test/register",
		"dcr_enabled":                      true, "credential_type": "user", "pkce_enabled": true,
		"refresh_buffer_seconds": 120,
		"created_at":             "2026-10-31T12:00:00Z", "updated_at": "2026-10-31T12:00:01Z",
		"client_secret": hidden, "enrollment_url": "https://private.example.test/?token=" + hidden,
		"credentials": []any{map[string]any{"secret": hidden}},
	}
}

func providerTemplateRecord(name string) map[string]any {
	return map[string]any{
		"name": name, "display_name": "Provider template", "secret_type": "authorization_code",
		"bearer_methods_supported": []string{"header", "form"}, "default_scopes": []string{},
		"pkce_required": false, "token_endpoint": nil, "authorization_endpoint": nil, "registration_endpoint": nil,
	}
}

func providerHTTP(t *testing.T, value any) *http.Response {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data)))}
}

func providerPage(data []any, next any) map[string]any {
	return map[string]any{"data": data, "meta": map[string]any{"page": map[string]any{"size": 1, "next": next}}}
}

func TestVaultProviderCompleteNameLookupAndPublicProjection(t *testing.T) {
	calls, gets := 0, 0
	api := vaultProvidersReaderStub{
		list: func(request kkOps.ListVaultProvidersRequest) (*kkOps.ListVaultProvidersResponse, error) {
			calls++
			require.Equal(t, providerTestDirectory, request.DirectoryID)
			require.EqualValues(t, 1, *request.Page.Size)
			require.Nil(t, request.Sort)
			require.Nil(t, request.Filter)
			if calls == 1 {
				require.Nil(t, request.Page.After)
				return &kkOps.ListVaultProvidersResponse{RawResponse: providerHTTP(t, providerPage(
					[]any{providerTestRecord(providerTestFirst, "first")},
					"/v2/directories/"+providerTestDirectory+"/vault/providers?page%5Bafter%5D=cursor%2B%2F%3D%3D",
				))}, nil
			}
			require.Equal(t, "cursor+/==", *request.Page.After)
			return &kkOps.ListVaultProvidersResponse{RawResponse: providerHTTP(t, providerPage(
				[]any{providerTestRecord(providerTestSecond, "target")}, nil,
			))}, nil
		},
		get: func(directory, id string) (*kkOps.GetVaultProviderResponse, error) {
			gets++
			require.Equal(t, providerTestDirectory, directory)
			require.Equal(t, providerTestSecond, id)
			return &kkOps.GetVaultProviderResponse{RawResponse: providerHTTP(t, providerTestRecord(id, "target"))}, nil
		},
	}
	value, err := ResolveIdentityVaultProviderName(t.Context(), api, providerTestDirectory, "target", 1)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, 1, gets)
	require.False(t, *value.Enabled)
	require.True(t, *value.DCREnabled)
	require.True(t, *value.PKCEEnabled)
	require.EqualValues(t, 120, *value.RefreshBufferSeconds)
	require.Nil(t, value.ClientID)
	require.Equal(t, "https://provider.example.test/register", *value.RegistrationEndpointEffective)
	for _, marshal := range []func(any) ([]byte, error){json.Marshal, yaml.Marshal} {
		encoded, err := marshal(value)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "must-not-appear")
		require.NotContains(t, string(encoded), "client_secret")
		require.NotContains(t, string(encoded), "enrollment_url")
		require.Contains(t, string(encoded), "enabled")
		require.Contains(t, string(encoded), "false")
	}
}

func TestVaultProviderInventoryRejectsMissingAndAmbiguousObservations(t *testing.T) {
	for _, field := range []string{
		"id", "directory_id", "name", "template_name", "bearer_methods_supported",
		"scopes", "enabled", "dcr_enabled", "credential_type", "pkce_enabled", "refresh_buffer_seconds",
		"created_at", "updated_at",
	} {
		t.Run(field, func(t *testing.T) {
			record := providerTestRecord(providerTestFirst, "first")
			delete(record, field)
			api := vaultProvidersReaderStub{
				list: func(kkOps.ListVaultProvidersRequest) (*kkOps.ListVaultProvidersResponse, error) {
					return &kkOps.ListVaultProvidersResponse{
						RawResponse: providerHTTP(t, providerPage([]any{record}, nil)),
					}, nil
				},
			}
			values, err := ListIdentityVaultProviders(t.Context(), api, providerTestDirectory, 1)
			require.Error(t, err)
			require.Nil(t, values)
		})
	}
	for _, kind := range []string{"duplicate-name", "duplicate-id", "other-directory", "invalid-id", "negative-buffer"} {
		t.Run(kind, func(t *testing.T) {
			first, next := providerTestRecord(
				providerTestFirst,
				"first",
			), providerTestRecord(
				providerTestSecond,
				"second",
			)
			switch kind {
			case "duplicate-name":
				next["name"] = "first"
			case "duplicate-id":
				next["id"] = providerTestFirst
			case "other-directory":
				next["directory_id"] = providerTestSecond
			case "invalid-id":
				next["id"] = "invalid"
			case "negative-buffer":
				next["refresh_buffer_seconds"] = -1
			}
			api := vaultProvidersReaderStub{
				list: func(kkOps.ListVaultProvidersRequest) (*kkOps.ListVaultProvidersResponse, error) {
					return &kkOps.ListVaultProvidersResponse{
						RawResponse: providerHTTP(t, providerPage([]any{first, next}, nil)),
					}, nil
				},
			}
			values, err := ListIdentityVaultProviders(t.Context(), api, providerTestDirectory, 1)
			require.Error(t, err)
			require.Nil(t, values)
		})
	}
}

func TestVaultProviderPaginationDoesNotReturnPartialData(t *testing.T) {
	for _, response := range []any{
		map[string]any{},
		map[string]any{"data": []any{}},
		providerPage(nil, nil),
		map[string]any{"data": []any{}, "meta": map[string]any{"page": map[string]any{"size": 1.5, "next": nil}}},
		map[string]any{"data": []any{}, "meta": map[string]any{"page": map[string]any{"size": 0, "next": nil}}},
		map[string]any{"data": []any{}, "meta": map[string]any{"page": map[string]any{"size": 1}}},
		providerPage([]any{}, 42), providerPage([]any{}, "repeated-cursor"),
	} {
		api := vaultProvidersReaderStub{
			list: func(kkOps.ListVaultProvidersRequest) (*kkOps.ListVaultProvidersResponse, error) {
				return &kkOps.ListVaultProvidersResponse{RawResponse: providerHTTP(t, response)}, nil
			},
		}
		values, err := ListIdentityVaultProviders(t.Context(), api, providerTestDirectory, 1)
		require.Error(t, err)
		require.Nil(t, values)
	}
	calls := 0
	api := vaultProvidersReaderStub{
		list: func(kkOps.ListVaultProvidersRequest) (*kkOps.ListVaultProvidersResponse, error) {
			calls++
			if calls == 1 {
				return &kkOps.ListVaultProvidersResponse{RawResponse: providerHTTP(t, providerPage(
					[]any{providerTestRecord(providerTestFirst, "first")}, "next",
				))}, nil
			}
			return nil, &sdkerrors.SDKError{StatusCode: 403, Body: "sensitive-backend-body"}
		},
	}
	values, err := ListIdentityVaultProviders(t.Context(), api, providerTestDirectory, 1)
	require.ErrorContains(t, err, "HTTP 403")
	require.NotContains(t, err.Error(), "sensitive-backend-body")
	require.Nil(t, values)
}

func TestVaultProviderLookupRequiresConsistentFreshIdentity(t *testing.T) {
	api := vaultProvidersReaderStub{
		list: func(kkOps.ListVaultProvidersRequest) (*kkOps.ListVaultProvidersResponse, error) {
			return &kkOps.ListVaultProvidersResponse{RawResponse: providerHTTP(t, providerPage(
				[]any{providerTestRecord(providerTestFirst, "target")}, nil,
			))}, nil
		},
		get: func(string, string) (*kkOps.GetVaultProviderResponse, error) {
			return &kkOps.GetVaultProviderResponse{
				RawResponse: providerHTTP(t, providerTestRecord(providerTestFirst, "renamed")),
			}, nil
		},
	}
	_, err := ResolveIdentityVaultProviderName(t.Context(), api, providerTestDirectory, "target", 1)
	require.ErrorContains(t, err, "name changed")
	api.get = func(string, string) (*kkOps.GetVaultProviderResponse, error) {
		return &kkOps.GetVaultProviderResponse{
			RawResponse: providerHTTP(t, providerTestRecord(providerTestSecond, "target")),
		}, nil
	}
	_, err = GetIdentityVaultProvider(t.Context(), api, providerTestDirectory, providerTestFirst)
	require.ErrorContains(t, err, "differs from the selected")
	_, err = ResolveIdentityVaultProviderName(t.Context(), api, providerTestDirectory, "missing", 1)
	require.ErrorContains(t, err, "not found")
	_, err = ListIdentityVaultProviders(t.Context(), nil, providerTestDirectory, 1)
	require.ErrorContains(t, err, "not configured")
	_, err = ListIdentityVaultProviders(t.Context(), api, "bad-id", 1)
	require.ErrorContains(t, err, "UUID")
}

func TestProviderTemplateCatalogNullableDefaultsAndCompletePagination(t *testing.T) {
	calls := 0
	api := vaultProvidersReaderStub{
		templates: func(page *kkComps.CursorPageParameters) (*kkOps.ListCredentialProviderTemplatesResponse, error) {
			calls++
			require.EqualValues(t, 1, *page.Size)
			next, name := any("cursor"), "first"
			if calls == 2 {
				next, name = nil, "second"
				require.Equal(t, "cursor", *page.After)
			}
			return &kkOps.ListCredentialProviderTemplatesResponse{RawResponse: providerHTTP(t,
				providerPage([]any{providerTemplateRecord(name)}, next))}, nil
		},
		template: func(name kkComps.ProviderTemplateName) (*kkOps.GetCredentialProviderTemplateResponse, error) {
			return &kkOps.GetCredentialProviderTemplateResponse{
				RawResponse: providerHTTP(t, providerTemplateRecord(string(name))),
			}, nil
		},
	}
	values, err := ListIdentityCredentialProviderTemplates(t.Context(), api, 1)
	require.NoError(t, err)
	require.Len(t, values, 2)
	require.Equal(t, 2, calls)
	require.False(t, *values[0].PKCERequired)
	require.NotNil(t, values[0].DefaultScopes)
	require.Empty(t, values[0].DefaultScopes)
	require.Nil(t, values[0].TokenEndpoint)
	value, err := GetIdentityCredentialProviderTemplate(t.Context(), api, "custom-oauth")
	require.NoError(t, err)
	require.Equal(t, "custom-oauth", value.Name)
	for _, field := range []string{
		"name", "display_name", "secret_type", "bearer_methods_supported", "default_scopes",
		"token_endpoint", "authorization_endpoint",
	} {
		record := providerTemplateRecord("template")
		delete(record, field)
		data, err := json.Marshal(record)
		require.NoError(t, err)
		var value IdentityCredentialProviderTemplate
		require.Error(t, json.Unmarshal(data, &value), field)
	}
	api.templates = func(*kkComps.CursorPageParameters) (*kkOps.ListCredentialProviderTemplatesResponse, error) {
		return &kkOps.ListCredentialProviderTemplatesResponse{RawResponse: providerHTTP(t,
			providerPage([]any{providerTemplateRecord("same"), providerTemplateRecord("same")}, nil))}, nil
	}
	_, err = ListIdentityCredentialProviderTemplates(t.Context(), api, 1)
	require.ErrorContains(t, err, "repeats")
	api.template = func(kkComps.ProviderTemplateName) (*kkOps.GetCredentialProviderTemplateResponse, error) {
		return &kkOps.GetCredentialProviderTemplateResponse{
			RawResponse: providerHTTP(t, providerTemplateRecord("other")),
		}, nil
	}
	_, err = GetIdentityCredentialProviderTemplate(t.Context(), api, "target")
	require.ErrorContains(t, err, "differs from the selected")
}
