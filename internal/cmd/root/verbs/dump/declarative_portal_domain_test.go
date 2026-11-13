package dump

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	declresources "github.com/kong/kongctl/internal/declarative/resources"
	declstate "github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestPortalCustomDomainDumpReload(t *testing.T) {
	transport := portalChildDumpHTTPClient(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "/v3/portals/portal-id/custom-domain", req.URL.Path)
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"hostname":"portal.example.test","enabled":true,` +
				`"ssl":{"domain_verification_method":"http"}}`)), Request: req,
		}, nil
	})
	sdk := &helpers.KonnectSDK{SDK: kkSDK.New(
		kkSDK.WithServerURL("https://example.test"), kkSDK.WithClient(transport),
	)}
	client := declstate.NewClient(declstate.ClientConfig{PortalCustomDomainAPI: sdk.GetPortalCustomDomainAPI()})
	domain, err := buildPortalCustomDomain(t.Context(), slog.Default(), client, "portal-id", "Portal")
	require.NoError(t, err)
	require.NotNil(t, domain)
	manifest := declresources.ResourceSet{Portals: []declresources.PortalResource{{
		BaseResource: declresources.BaseResource{Ref: "portal"},
		CreatePortal: kkComps.CreatePortal{Name: "Portal"},
		CustomDomain: domain,
	}}}
	data, err := yaml.Marshal(manifest)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "dump.yaml")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	loaded, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	require.Len(t, loaded.Portals, 1)
	require.Len(t, loaded.PortalCustomDomains, 1)
	actual := loaded.PortalCustomDomains[0]
	require.Equal(t, "portal.example.test", actual.Hostname)
	require.True(t, actual.Enabled)
	require.NotNil(t, actual.Ssl.HTTP)
	require.Equal(t, kkComps.CreatePortalCustomDomainSSLStandardDomainVerificationMethodHTTP,
		actual.Ssl.HTTP.GetDomainVerificationMethod())
}
