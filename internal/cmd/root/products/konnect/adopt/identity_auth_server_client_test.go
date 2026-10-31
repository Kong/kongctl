package adopt

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/labels"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestAdoptIdentityClientPreservesAuthenticationAndPublicFields(t *testing.T) {
	parent := "12345678-1234-4123-8123-123456789012"
	method := kkComps.AuthServerTokenEndpointAuthMethod("client_secret_basic")
	api := &fixture.AuthServerChildrenAPI{Clients: map[string][]kkComps.AuthServerClient{parent: {{
		ID: "opaque-client", Name: "worker", TokenEndpointAuthMethod: &method,
		Labels: map[string]string{"team": "platform", labels.ProtectedKey: "true"}, AllowScopes: []string{"keep"},
	}}}}
	result, err := adoptIdentityAuthServerClient(t.Context(), api, parent, "worker", true, "client-team", false)
	require.NoError(t, err)
	require.Equal(t, "opaque-client", result.ID)
	require.Len(t, api.ClientUpdates, 1)
	request := api.ClientUpdates[0].UpdateClient
	require.Nil(t, request.ClientSecret)
	require.Nil(t, request.AllowScopes)
	require.Equal(t, method, *request.TokenEndpointAuthMethod)
	require.True(t, labels.IsProtectedResource(api.Clients[parent][0].Labels))
	require.Equal(t, "platform", api.Clients[parent][0].Labels["team"])
	require.Equal(t, []string{"keep"}, api.Clients[parent][0].AllowScopes)
	_, err = adoptIdentityAuthServerClient(t.Context(), api, parent, "opaque-client", false, "other", false)
	require.ErrorContains(t, err, "overwrite-namespace")
	_, err = adoptIdentityAuthServerClient(t.Context(), api, parent, "opaque-client", false, "other", true)
	require.NoError(t, err)
	_, err = adoptIdentityAuthServerClient(t.Context(), api, "bad", "opaque-client", false, "other", true)
	require.Error(t, err)
}

func TestIdentityClientAdoptCommandSelectors(t *testing.T) {
	cmd, err := NewIdentityAuthServerClientCmd(verbs.Adopt, nil, nil)
	require.NoError(t, err)
	require.Error(t, cmd.Args(cmd, nil))
	require.NoError(t, cmd.Args(cmd, []string{"opaque-client"}))
	require.NoError(t, cmd.Flags().Set("client-name", "worker"))
	require.NoError(t, cmd.Args(cmd, nil))
	require.Error(t, cmd.Args(cmd, []string{"opaque-client"}))
	require.NotNil(t, cmd.Flags().Lookup("auth-server-id"))
}
