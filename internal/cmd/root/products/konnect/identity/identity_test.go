package identity

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestIdentityDirectoryCommands(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List, verbs.Adopt} {
		command, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		child, _, err := command.Find([]string{"directories"})
		require.NoError(t, err)
		require.Equal(t, "directory", child.Name())
		require.NotNil(t, child.RunE)
		require.Error(t, child.Args(child, []string{"one", "two"}))
		if verb == verbs.Adopt {
			require.Error(t, child.Args(child, nil))
		}
	}
}

func TestIdentityAuthServerCommands(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List, verbs.Adopt} {
		command, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		child, _, err := command.Find([]string{"auth-servers"})
		require.NoError(t, err)
		require.Equal(t, "auth-server", child.Name())
		require.NotNil(t, child.RunE)
		require.Error(t, child.Args(child, []string{"one", "two"}))
	}
}

func TestIdentityAuthServerChildCommands(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		command, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		for _, alias := range []string{"auth-server-scopes", "auth-server-claims"} {
			child, _, err := command.Find([]string{alias})
			require.NoError(t, err)
			require.NotNil(t, child.RunE)
			require.NotNil(t, child.Flags().Lookup("auth-server-id"))
			require.NotNil(t, child.Flags().Lookup("auth-server-name"))
			require.Error(t, child.Args(child, []string{"one", "two"}))
			_, err = authServerIDFromFlags(t.Context(), child, nil, 100)
			require.ErrorContains(t, err, "exactly one")
			require.NoError(t, child.Flags().Set("auth-server-id", "invalid"))
			_, err = authServerIDFromFlags(t.Context(), child, nil, 100)
			require.ErrorContains(t, err, "UUID")
		}
	}
}

func TestPrincipalDisplayDoesNotInventActiveState(t *testing.T) {
	require.Equal(t, "unknown", principalDisplay(helpers.IdentityPrincipal{}).Active)
	require.Equal(t, "false", principalDisplay(helpers.IdentityPrincipal{Active: new(false)}).Active)
	require.Equal(t, "true", principalDisplay(helpers.IdentityPrincipal{Active: new(true)}).Active)
}

func TestIdentityPrincipalCommands(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		command, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		child, _, err := command.Find([]string{"principals"})
		require.NoError(t, err)
		require.Equal(t, "principal", child.Name())
		require.NotNil(t, child.RunE)
		require.NotNil(t, child.Flags().Lookup("directory-id"))
		require.NotNil(t, child.Flags().Lookup("directory-name"))
		require.Error(t, child.Args(child, []string{"one", "two"}))
	}
}
