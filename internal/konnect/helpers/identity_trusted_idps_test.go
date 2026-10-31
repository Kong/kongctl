package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/stretchr/testify/require"
)

const (
	trustedIDPTestDirectory = "731559d3-39bd-48c7-b33c-eb192847eb20"
	trustedIDPTestFirst     = "731559d3-39bd-48c7-b33c-eb192847eb21"
	trustedIDPTestSecond    = "731559d3-39bd-48c7-b33c-eb192847eb22"
	trustedIDPTestIssuer    = "https://issuer.example.test"
)

type trustedIDPReaderStub struct {
	list       func(string, *kkComps.CursorPageParameters) (*kkOps.ListTrustedIdpsResponse, error)
	get        func(string, string) (*kkOps.GetTrustedIdpResponse, error)
	principals func(kkOps.ListTrustedIdpPrincipalsRequest) (*kkOps.ListTrustedIdpPrincipalsResponse, error)
}

func (s trustedIDPReaderStub) ListTrustedIdps(_ context.Context, directory string,
	page *kkComps.CursorPageParameters, _ ...kkOps.Option,
) (*kkOps.ListTrustedIdpsResponse, error) {
	return s.list(directory, page)
}

func (s trustedIDPReaderStub) GetTrustedIdp(_ context.Context, directory, id string, _ ...kkOps.Option,
) (*kkOps.GetTrustedIdpResponse, error) {
	return s.get(directory, id)
}

func (s trustedIDPReaderStub) ListTrustedIdpPrincipals(_ context.Context,
	request kkOps.ListTrustedIdpPrincipalsRequest, _ ...kkOps.Option,
) (*kkOps.ListTrustedIdpPrincipalsResponse, error) {
	return s.principals(request)
}

func trustedIDPHTTP(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}
}

func trustedIDPRecord(id, issuer string) string {
	return fmt.Sprintf(`{"id":%q,"issuer_url":%q,"jwks_uri":null,`+
		`"display_name":"Shared name","jit_provisioning_enabled":false,"vault_access_enabled":true,`+
		`"jit":{"mapping":{"display_name":"name","metadata":{"team":"team"}}}}`, id, issuer)
}

func trustedIDPPage(data, next string) string {
	return `{"data":[` + data + `],"meta":{"page":{"size":1,"next":` + next + `}}}`
}

func TestIdentityTrustedIDPCompleteLookupPreservesConfigurationAndRejectsAmbiguity(t *testing.T) {
	calls, gets := 0, 0
	api := trustedIDPReaderStub{
		list: func(directory string, page *kkComps.CursorPageParameters) (*kkOps.ListTrustedIdpsResponse, error) {
			calls++
			require.Equal(t, trustedIDPTestDirectory, directory)
			require.EqualValues(t, 1, *page.Size)
			if calls == 1 {
				require.Nil(t, page.After)
				return &kkOps.ListTrustedIdpsResponse{RawResponse: trustedIDPHTTP(trustedIDPPage(
					trustedIDPRecord(trustedIDPTestFirst, trustedIDPTestIssuer+"/other"),
					`"/v2/directories/`+directory+`/trusted-idps?page%5Bafter%5D=cursor%2B%2F%3D%3D"`,
				))}, nil
			}
			require.Equal(t, "cursor+/==", *page.After)
			return &kkOps.ListTrustedIdpsResponse{RawResponse: trustedIDPHTTP(trustedIDPPage(
				trustedIDPRecord(trustedIDPTestSecond, trustedIDPTestIssuer), "null",
			))}, nil
		},
		get: func(directory, id string) (*kkOps.GetTrustedIdpResponse, error) {
			gets++
			require.Equal(t, trustedIDPTestDirectory, directory)
			require.Equal(t, trustedIDPTestSecond, id)
			return &kkOps.GetTrustedIdpResponse{
				RawResponse: trustedIDPHTTP(trustedIDPRecord(id, trustedIDPTestIssuer)),
			}, nil
		},
	}
	value, err := ResolveIdentityTrustedIDPIssuer(t.Context(), api, trustedIDPTestDirectory, trustedIDPTestIssuer, 1)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, 1, gets)
	require.Equal(t, trustedIDPTestSecond, value.ID)
	require.False(t, *value.JitProvisioningEnabled)
	require.True(t, *value.VaultAccessEnabled)
	require.Nil(t, value.JwksURI)
	require.Equal(t, "name", *value.Jit.Mapping.DisplayName)
	require.Equal(t, "team", value.Jit.Mapping.Metadata["team"])

	api.list = func(string, *kkComps.CursorPageParameters) (*kkOps.ListTrustedIdpsResponse, error) {
		return &kkOps.ListTrustedIdpsResponse{RawResponse: trustedIDPHTTP(trustedIDPPage(
			trustedIDPRecord(trustedIDPTestFirst, trustedIDPTestIssuer)+","+
				trustedIDPRecord(trustedIDPTestSecond, trustedIDPTestIssuer), "null",
		))}, nil
	}
	value, err = ResolveIdentityTrustedIDPIssuer(t.Context(), api, trustedIDPTestDirectory, trustedIDPTestIssuer, 1)
	require.ErrorContains(t, err, "multiple trusted IdPs")
	require.Nil(t, value)
	require.Equal(t, 1, gets, "ambiguous lookup must not select a detail record")
}

func TestIdentityTrustedIDPInvalidInventoriesReturnNoPartialResult(t *testing.T) {
	valid := trustedIDPRecord(trustedIDPTestSecond, trustedIDPTestIssuer)
	for _, body := range []string{
		`{}`, `{"data":[],"meta":{"page":{"size":1}}}`,
		`{"data":[],"meta":{"page":{"size":0,"next":null}}}`,
		`{"data":[],"meta":{"page":{"size":1.5,"next":null}}}`,
		`{"data":[],"meta":{"page":{"size":1,"next":42}}}`,
		trustedIDPPage(trustedIDPRecord("bad-id", trustedIDPTestIssuer), "null"),
		trustedIDPPage(trustedIDPRecord(trustedIDPTestFirst, trustedIDPTestIssuer), "null"),
		trustedIDPPage(valid, `"cycle"`),
	} {
		t.Run(body, func(t *testing.T) {
			calls := 0
			api := trustedIDPReaderStub{
				list: func(string, *kkComps.CursorPageParameters) (*kkOps.ListTrustedIdpsResponse, error) {
					calls++
					if calls == 1 {
						return &kkOps.ListTrustedIdpsResponse{RawResponse: trustedIDPHTTP(trustedIDPPage(
							trustedIDPRecord(trustedIDPTestFirst, trustedIDPTestIssuer), `"cycle"`,
						))}, nil
					}
					return &kkOps.ListTrustedIdpsResponse{RawResponse: trustedIDPHTTP(body)}, nil
				},
			}
			values, err := ListIdentityTrustedIDPs(t.Context(), api, trustedIDPTestDirectory, 1)
			require.Error(t, err)
			require.Nil(t, values)
			require.Equal(t, 2, calls)
		})
	}
	for _, outcome := range []string{"nil", "missing-body", "failed-page"} {
		t.Run(outcome, func(t *testing.T) {
			calls := 0
			api := trustedIDPReaderStub{
				list: func(string, *kkComps.CursorPageParameters) (*kkOps.ListTrustedIdpsResponse, error) {
					calls++
					if calls == 1 {
						return &kkOps.ListTrustedIdpsResponse{
							RawResponse: trustedIDPHTTP(trustedIDPPage(valid, `"next"`)),
						}, nil
					}
					switch outcome {
					case "nil":
						return nil, nil
					case "missing-body":
						return &kkOps.ListTrustedIdpsResponse{}, nil
					default:
						return nil, &sdkerrors.SDKError{StatusCode: 403, Body: "private-response-value"}
					}
				},
			}
			values, err := ListIdentityTrustedIDPs(t.Context(), api, trustedIDPTestDirectory, 1)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "private-response-value")
			require.Nil(t, values)
		})
	}
}

func TestIdentityTrustedIDPRequiredConfigurationIsNotSynthesized(t *testing.T) {
	body := trustedIDPRecord(trustedIDPTestFirst, trustedIDPTestIssuer)
	for _, field := range []string{"id", "issuer_url", "jwks_uri", "jit_provisioning_enabled", "vault_access_enabled"} {
		t.Run(field, func(t *testing.T) {
			var raw map[string]any
			require.NoError(t, json.Unmarshal([]byte(body), &raw))
			delete(raw, field)
			encoded, err := json.Marshal(raw)
			require.NoError(t, err)
			var value IdentityTrustedIDP
			require.Error(t, json.Unmarshal(encoded, &value))
		})
	}
	api := trustedIDPReaderStub{get: func(string, string) (*kkOps.GetTrustedIdpResponse, error) {
		return &kkOps.GetTrustedIdpResponse{RawResponse: trustedIDPHTTP(strings.Replace(body,
			`"display_name":"Shared name"`, `"display_name":null,"secret":"unexpected-material"`, 1))}, nil
	}}
	value, err := GetIdentityTrustedIDP(t.Context(), api, trustedIDPTestDirectory, trustedIDPTestFirst)
	require.NoError(t, err)
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "unexpected-material")
	require.Contains(t, string(encoded), `"display_name":null`)
	require.Contains(t, string(encoded), `"jit_provisioning_enabled":false`)
	_, err = GetIdentityTrustedIDP(t.Context(), api, trustedIDPTestDirectory, trustedIDPTestSecond)
	require.ErrorContains(t, err, "differs from the selected ID")
}

func TestIdentityTrustedIDPPrincipalInventoryPreservesPublicState(t *testing.T) {
	calls := 0
	api := trustedIDPReaderStub{principals: func(request kkOps.ListTrustedIdpPrincipalsRequest) (
		*kkOps.ListTrustedIdpPrincipalsResponse, error,
	) {
		calls++
		require.Equal(t, trustedIDPTestDirectory, request.DirectoryID)
		require.Equal(t, trustedIDPTestFirst, request.TrustedIdpID)
		require.EqualValues(t, 1, *request.Page.Size)
		id, active, next := trustedIDPTestFirst, `,"active":false`, `"second"`
		if calls > 1 {
			require.Equal(t, "second", *request.Page.After)
			id, active, next = trustedIDPTestSecond, "", "null"
		}
		body := trustedIDPPage(`{"id":"`+id+`","display_name":"Test principal",`+
			`"type":"user","managed_by":{},"secret":"unexpected"`+active+`}`, next)
		return &kkOps.ListTrustedIdpPrincipalsResponse{RawResponse: trustedIDPHTTP(body)}, nil
	}}
	values, err := ListIdentityTrustedIDPPrincipals(t.Context(), api, trustedIDPTestDirectory, trustedIDPTestFirst, 1)
	require.NoError(t, err)
	require.Len(t, values, 2)
	require.False(t, *values[0].Active)
	require.Nil(t, values[1].Active)
	encoded, err := json.Marshal(values)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "unexpected")
	require.Equal(t, 2, calls)
}

func TestIdentityTrustedIDPReadsValidateScopeBeforeCallingAPI(t *testing.T) {
	api := trustedIDPReaderStub{}
	_, err := ListIdentityTrustedIDPs(t.Context(), nil, trustedIDPTestDirectory, 1)
	require.ErrorContains(t, err, "not configured")
	_, err = ListIdentityTrustedIDPs(t.Context(), api, "invalid", 1)
	require.ErrorContains(t, err, "directory ID")
	_, err = GetIdentityTrustedIDP(t.Context(), api, trustedIDPTestDirectory, "invalid")
	require.ErrorContains(t, err, "IDs must be UUIDs")
	_, err = ResolveIdentityTrustedIDPIssuer(t.Context(), api, trustedIDPTestDirectory, "", 1)
	require.ErrorContains(t, err, "must not be empty")
	_, err = ListIdentityTrustedIDPPrincipals(t.Context(), api, "invalid", trustedIDPTestFirst, 1)
	require.ErrorContains(t, err, "IDs must be UUIDs")
}
