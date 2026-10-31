package adopt

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestAdoptAuthServerPreservesSettingsAndRequiresExplicitTakeover(t *testing.T) {
	current := fixture.AuthServer("issuer", "another-team", true)
	api := &fixture.AuthServerAPI{Servers: []kkComps.AuthServer{current}}
	_, err := adoptIdentityAuthServer(t.Context(), api, "issuer", "auth-team", false)
	require.ErrorContains(t, err, "overwrite-namespace")
	require.Empty(t, api.Writes)
	result, err := adoptIdentityAuthServer(t.Context(), api, current.ID, "auth-team", true)
	require.NoError(t, err)
	require.Equal(t, current.ID, result.ID)
	require.Equal(t, "auth-team", api.Servers[0].Labels[labels.NamespaceKey])
	require.Equal(t, "true", api.Servers[0].Labels[labels.ProtectedKey])
	require.Equal(t, "test", api.Servers[0].Labels["owner"])
	require.Len(t, api.Writes, 1)
	write := api.Writes[0]
	require.Nil(t, write.Name)
	require.Nil(t, write.Audience)
	require.Nil(t, write.Description)
	require.Nil(t, write.SigningAlgorithm)
	require.Nil(t, write.TrustedOrigins)
	require.Nil(t, write.Dcr)
}
