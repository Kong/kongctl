package identity

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/maturity"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

const fixtureCommandID = "731559d3-39bd-48c7-b33c-eb192847eb20"

func TestIdentityHelpTextFitsEightyColumns(t *testing.T) {
	for _, verb := range []verbs.VerbValue{
		verbs.Get, verbs.List, verbs.Adopt, verbs.Inspect, verbs.Import,
	} {
		root, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		var check func(*cobra.Command)
		check = func(command *cobra.Command) {
			for name, text := range map[string]string{
				"short": command.Short, "description": command.Long, "examples": command.Example,
			} {
				for line := range strings.SplitSeq(text, "\n") {
					require.LessOrEqual(t, len(line), 80, "%s %s %s: %s", verb, command.CommandPath(), name, line)
				}
			}
			for _, child := range command.Commands() {
				check(child)
			}
		}
		check(root)
	}
}

func TestIdentityAlias(t *testing.T) {
	for _, verb := range []verbs.VerbValue{
		verbs.Get, verbs.List, verbs.Adopt, verbs.Inspect,
	} {
		t.Run(string(verb), func(t *testing.T) {
			command, err := NewIdentityCmd(verb, nil, nil)
			require.NoError(t, err)
			root := &cobra.Command{Use: string(verb)}
			root.AddCommand(command)
			found, _, err := root.Find([]string{"id"})
			require.NoError(t, err)
			require.Same(t, command, found)
		})
	}
}

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

func TestIdentityClientReadCommandsRequireExplicitSelectors(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		command, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		client, _, err := command.Find([]string{"auth-server-clients"})
		require.NoError(t, err)
		require.NotNil(t, client.RunE)
		require.NoError(t, client.Args(client, []string{"opaque-client-id"}))
		require.NoError(t, client.Flags().Set("client-name", "machine"))
		require.ErrorContains(t, client.Args(client, []string{"opaque-id"}), "not both")
		require.NoError(t, client.Args(client, nil))
		secret, _, err := command.Find([]string{"auth-server-client-secrets"})
		require.NoError(t, err)
		require.NotNil(t, secret.RunE)
		require.ErrorContains(t, secret.Args(secret, nil), "exactly one")
		require.NoError(t, secret.Flags().Set("client-id", "opaque-id"))
		require.NoError(t, secret.Args(secret, nil))
		require.ErrorContains(t, secret.Args(secret, []string{"not-a-uuid"}), "UUID")
		require.NoError(t, secret.Args(secret, []string{"731559d3-39bd-48c7-b33c-eb192847eb20"}))
		require.NoError(t, secret.Flags().Set("client-name", "machine"))
		require.ErrorContains(t, secret.Args(secret, nil), "exactly one")
		require.Error(t, secret.Args(secret, []string{"one", "two"}))
	}
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

func TestIdentityPrincipalIdentityCommandsRequireExplicitParents(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		command, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		child, _, err := command.Find([]string{"principal-identities"})
		require.NoError(t, err)
		require.Equal(t, "principal-identity", child.Name())
		require.NotNil(t, child.RunE)
		require.NotNil(t, child.Flags().Lookup("directory-name"))
		require.ErrorContains(t, child.Args(child, nil), "principal-id")
		require.NoError(t, child.Flags().Set("principal-id", "731559d3-39bd-48c7-b33c-eb192847eb20"))
		require.NoError(t, child.Args(child, nil))
		require.NoError(t, child.Args(child, []string{"731559d3-39bd-48c7-b33c-eb192847eb21"}))
		require.ErrorContains(t, child.Args(child, []string{"display-name"}), "UUID")
		require.Error(t, child.Args(child, []string{"one", "two"}))
	}
}

func TestIdentityPrincipalKeyCommandsRequireExplicitParents(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		command, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		child, _, err := command.Find([]string{"principal-api-keys"})
		require.NoError(t, err)
		require.Equal(t, "principal-api-key", child.Name())
		require.NotNil(t, child.RunE)
		require.NotNil(t, child.Flags().Lookup("directory-id"))
		require.NotNil(t, child.Flags().Lookup("directory-name"))
		require.NoError(t, child.Flags().Set("directory-name", "default"))
		require.ErrorContains(t, child.Args(child, nil), "principal-id")
		require.NoError(t, child.Flags().Set("principal-id", "731559d3-39bd-48c7-b33c-eb192847eb20"))
		require.NoError(t, child.Args(child, nil))
		require.NoError(t, child.Args(child, []string{"731559d3-39bd-48c7-b33c-eb192847eb21"}))
		require.ErrorContains(t, child.Args(child, []string{"key-name"}), "UUID")
		require.Error(t, child.Args(child, []string{"one", "two"}))
	}
}

func TestIdentityCommandTreesAreGenerallyAvailable(t *testing.T) {
	for _, verb := range []verbs.VerbValue{
		verbs.Get, verbs.List, verbs.Adopt, verbs.Inspect, verbs.Import,
	} {
		t.Run(string(verb), func(t *testing.T) {
			root, err := NewIdentityCmd(verb, nil, nil)
			require.NoError(t, err)
			var check func(*cobra.Command)
			check = func(cmd *cobra.Command) {
				resolved, err := maturity.ResolveCommand(cmd)
				require.NoError(t, err)
				require.Equal(t, maturity.LevelGA, resolved.Effective.Level, cmd.CommandPath())
				require.Empty(t, resolved.Effective.Message)
				for _, child := range cmd.Commands() {
					check(child)
				}
			}
			check(root)
		})
	}
}

func TestIdentityRejectsImperativeResourceWrites(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Create, verbs.Patch, verbs.Delete} {
		cmd, err := NewIdentityCmd(verb, nil, nil)
		require.ErrorContains(t, err, "does not support imperative")
		require.Nil(t, cmd)
	}
}
