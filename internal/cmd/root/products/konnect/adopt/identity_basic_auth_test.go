package adopt

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestBasicAuthAdoptionPreservesUsernamePasswordsAndProtection(t *testing.T) {
	const directory, principal = "4078f559-063b-4f10-9098-9e6553b21d9b", "5078f559-063b-4f10-9098-9e6553b21d9b"
	current := fixture.BasicAuth(principal, "old", true)
	key := directory + "/" + principal
	api := &fixture.BasicAuthAPI{
		Parents:   map[string][]helpers.IdentityBasicAuth{key: {current}},
		Passwords: map[string][]helpers.IdentityPassword{key + "/" + current.ID: {{ID: principal}}},
	}
	_, err := adoptIdentityBasicAuth(t.Context(), api, directory, principal, current.ID, "", "new", false)
	require.ErrorContains(t, err, "overwrite-namespace")
	require.Empty(t, api.Writes)
	result, err := adoptIdentityBasicAuth(t.Context(), api, directory, principal, "", principal, "new", true)
	require.NoError(t, err)
	require.Equal(t, current.ID, result.ID)
	require.Equal(t, current.Username, api.Writes[0].Username)
	require.Len(t, api.Writes[0].ManagedBy, 4)
	require.Equal(t, "true", api.Writes[0].ManagedBy[resources.ChildProtectedKey])
	require.Len(t, api.Passwords[key+"/"+current.ID], 1)
}

func TestBasicAuthAdoptCommandRequiresExplicitIdentity(t *testing.T) {
	command, err := NewIdentityBasicAuthCmd(verbs.Adopt, nil, nil)
	require.NoError(t, err)
	require.ErrorContains(t, command.Args(command, nil), "principal-id")
	require.NoError(t, command.Flags().Set("principal-id", "4078f559-063b-4f10-9098-9e6553b21d9b"))
	require.ErrorContains(t, command.Args(command, nil), "exactly one")
	require.NoError(t, command.Flags().Set("username", "4078f559-063b-4f10-9098-9e6553b21d9b"))
	require.NoError(t, command.Args(command, nil))
	require.Error(t, command.Args(command, []string{"4078f559-063b-4f10-9098-9e6553b21d9b"}))
}
