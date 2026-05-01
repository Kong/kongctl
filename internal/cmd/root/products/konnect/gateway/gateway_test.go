package gateway

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGatewayCoreEntityCommandsAreNestedUnderControlPlane(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		t.Run(verb.String(), func(t *testing.T) {
			cmd, err := NewGatewayCmd(verb, nil, nil)
			require.NoError(t, err)

			for _, name := range []string{"service", "services", "route", "routes", "consumer", "consumers"} {
				assert.False(t, hasDirectSubcommand(cmd, name), "gateway should not expose %q directly", name)
			}

			controlPlaneCmd := directSubcommand(cmd, "control-plane")
			require.NotNil(t, controlPlaneCmd)

			for _, name := range []string{"service", "services", "route", "routes", "consumer", "consumers"} {
				assert.True(t, hasDirectSubcommand(controlPlaneCmd, name), "control-plane should expose %q", name)
			}
		})
	}
}

func TestGatewayCoreEntitySiblingPathsFail(t *testing.T) {
	for _, name := range []string{"services", "routes", "consumers"} {
		t.Run(name, func(t *testing.T) {
			cmd, err := NewGatewayCmd(verbs.Get, nil, nil)
			require.NoError(t, err)

			cmd.SetArgs([]string{name})

			err = cmd.Execute()
			require.Error(t, err)
			assert.ErrorContains(t, err, "unknown command")
		})
	}
}

func directSubcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
		for _, alias := range child.Aliases {
			if alias == name {
				return child
			}
		}
	}
	return nil
}

func hasDirectSubcommand(parent *cobra.Command, name string) bool {
	return directSubcommand(parent, name) != nil
}
