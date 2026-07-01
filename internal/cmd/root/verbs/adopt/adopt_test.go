package adopt

import (
	"testing"

	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestAdoptFlagsArePersistentOnAdoptRoot(t *testing.T) {
	cmd, err := NewAdoptCmd()
	require.NoError(t, err)

	require.NotNil(t, cmd.PersistentFlags().Lookup(adoptCommon.NamespaceFlagName))
	require.NotNil(t, cmd.PersistentFlags().Lookup(adoptCommon.OverwriteNamespaceFlagName))
	require.Nil(t, findCommand(t, cmd, "portal").LocalNonPersistentFlags().Lookup(adoptCommon.NamespaceFlagName))
}

func TestAdoptPersistentFlagsAreInheritedByLeafCommands(t *testing.T) {
	testCases := []struct {
		name      string
		path      []string
		args      []string
		overwrite bool
	}{
		{
			name: "direct portal parent-position namespace",
			path: []string{"portal"},
			args: []string{"--namespace", "team-alpha", "portal", "portal-id"},
		},
		{
			name:      "direct portal leaf-position overwrite",
			path:      []string{"portal"},
			args:      []string{"portal", "portal-id", "--namespace", "team-alpha", "--overwrite-namespace"},
			overwrite: true,
		},
		{
			name:      "explicit konnect api",
			path:      []string{"konnect", "api"},
			args:      []string{"--namespace", "team-alpha", "konnect", "api", "api-id", "--overwrite-namespace"},
			overwrite: true,
		},
		{
			name: "nested analytics dashboard",
			path: []string{"analytics", "dashboard"},
			args: []string{"--namespace", "team-alpha", "analytics", "dashboard", "dashboard-id"},
		},
		{
			name: "nested organization team",
			path: []string{"organization", "team"},
			args: []string{"--namespace", "team-alpha", "org", "team", "22cd8a0b-72e7-4212-9099-0764f8e9c5ac"},
		},
		{
			name: "direct identity directory",
			path: []string{"identity", "directory"},
			args: []string{"--namespace", "team-alpha", "identity", "directory", "workforce"},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := NewAdoptCmd()
			require.NoError(t, err)
			clearLifecycleHooks(cmd)

			leaf := findCommand(t, cmd, tt.path...)
			leaf.RunE = func(c *cobra.Command, _ []string) error {
				flags, err := adoptCommon.ReadAdoptFlags(c)
				require.NoError(t, err)
				require.Equal(t, "team-alpha", flags.Namespace)
				require.Equal(t, tt.overwrite, flags.OverwriteNamespace)
				return nil
			}

			cmd.SetArgs(tt.args)
			require.NoError(t, cmd.Execute())
		})
	}
}

func findCommand(t *testing.T, root *cobra.Command, path ...string) *cobra.Command {
	t.Helper()

	cmd := root
	for _, name := range path {
		var next *cobra.Command
		for _, candidate := range cmd.Commands() {
			if candidate.Name() == name {
				next = candidate
				break
			}
		}
		require.NotNil(t, next, "missing command %q under %q", name, cmd.Name())
		cmd = next
	}
	return cmd
}

func clearLifecycleHooks(cmd *cobra.Command) {
	cmd.PersistentPreRun = nil
	cmd.PersistentPreRunE = nil
	cmd.PreRun = nil
	cmd.PreRunE = nil

	for _, child := range cmd.Commands() {
		clearLifecycleHooks(child)
	}
}
