package executor

import (
	"io"
	"net/http"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestPortalDomainCreatePreservesSSLMethodOnWire(t *testing.T) {
	for _, tc := range []struct {
		name string
		ssl  map[string]any
		want string
	}{
		{
			name: "http",
			ssl:  map[string]any{"domain_verification_method": "http"},
			want: `{"hostname":"portal.example.test","enabled":true,"ssl":{"domain_verification_method":"http"}}`,
		},
		{
			name: "custom_certificate",
			ssl: map[string]any{
				"domain_verification_method": "custom_certificate",
				"custom_certificate":         "fixture-certificate", "custom_private_key": "fixture-private-key",
				"skip_ca_check": true,
			},
			want: `{"hostname":"portal.example.test","enabled":true,"ssl":{` +
				`"domain_verification_method":"custom_certificate","custom_certificate":"fixture-certificate",` +
				`"custom_private_key":"fixture-private-key","skip_ca_check":true}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body []byte
			transport := customPolicyHTTPClient(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodPost, req.Method)
				require.Equal(t, "/v3/portals/portal-id/custom-domain", req.URL.Path)
				var err error
				body, err = io.ReadAll(req.Body)
				require.NoError(t, err)
				return &http.Response{
					StatusCode: http.StatusCreated, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{}`)), Request: req,
				}, nil
			})
			client := kkSDK.New(kkSDK.WithClient(transport), kkSDK.WithServerURL("https://example.test"))
			var request kkComps.CreatePortalCustomDomainRequest
			adapter := NewPortalDomainAdapter(nil)
			require.NoError(t, adapter.MapCreateFields(t.Context(), nil, map[string]any{
				planner.FieldHostname: "portal.example.test", planner.FieldEnabled: true, planner.FieldSSL: tc.ssl,
			}, &request))
			_, err := client.PortalCustomDomains.CreatePortalCustomDomain(t.Context(), "portal-id", request)
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(body))
		})
	}
}
