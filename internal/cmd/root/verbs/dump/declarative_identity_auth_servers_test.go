package dump

import (
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestAuthServerDumpRoundTrip(t *testing.T) {
	current := fixture.AuthServer("issuer", "auth-team", true)
	d := &declarativeDumpContext{stateClient: state.NewClient(state.ClientConfig{
		IdentityAuthServersAPI: &fixture.AuthServerAPI{Servers: []kkComps.AuthServer{current}},
	})}
	rs := &resources.ResourceSet{}
	require.NoError(t, identityAuthServerCollector().collect(t.Context(), d, rs))
	encoded, err := yaml.Marshal(rs)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "issuer:")
	require.NotContains(t, string(encoded), "KONGCTL-")
	file := filepath.Join(t.TempDir(), "dump.yaml")
	require.NoError(t, os.WriteFile(file, encoded, 0o600))
	loaded, err := loader.New().LoadFile(file)
	require.NoError(t, err)
	a := loaded.Identity.AuthServers[0]
	require.Equal(t, current.ID, a.Ref)
	require.Equal(t, current.Name, a.Name)
	require.Equal(t, current.Audience, a.Audience)
	require.Equal(t, current.TrustedOrigins, a.TrustedOrigins)
	require.Equal(t, *current.Dcr.DefaultAccessTokenDuration, *a.DCR.DefaultAccessTokenDuration)
	require.True(t, a.Protected())
	require.Equal(t, "auth-team", resources.GetNamespace(a.Kongctl))
}
