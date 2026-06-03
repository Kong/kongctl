//go:build integration

package integration

import (
	"testing"

	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	rootadopt "github.com/kong/kongctl/internal/cmd/root/verbs/adopt"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestAdoptCommandPersistentNamespaceFlags(t *testing.T) {
	testCases := []struct {
		name      string
		path      []string
		args      []string
		overwrite bool
	}{
		{
			name: "direct portal",
			path: []string{"portal"},
			args: []string{"portal", "portal-id", "--namespace", "team-alpha"},
		},
		{
			name:      "explicit konnect api",
			path:      []string{"konnect", "api"},
			args:      []string{"--namespace", "team-alpha", "konnect", "api", "api-id", "--overwrite-namespace"},
			overwrite: true,
		},
		{
			name:      "nested organization team",
			path:      []string{"organization", "team"},
			args:      []string{"--namespace", "team-alpha", "--overwrite-namespace", "org", "team", "team-id"},
			overwrite: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := rootadopt.NewAdoptCmd()
			require.NoError(t, err)
			clearAdoptLifecycleHooks(cmd)

			leaf := findAdoptCommand(t, cmd, tt.path...)
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

func findAdoptCommand(t *testing.T, root *cobra.Command, path ...string) *cobra.Command {
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

func clearAdoptLifecycleHooks(cmd *cobra.Command) {
	cmd.PersistentPreRun = nil
	cmd.PersistentPreRunE = nil
	cmd.PreRun = nil
	cmd.PreRunE = nil

	for _, child := range cmd.Commands() {
		clearAdoptLifecycleHooks(child)
	}
}
