package root

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs/create"
	"github.com/kong/kongctl/internal/cmd/root/verbs/del"
	"github.com/kong/kongctl/internal/cmd/root/verbs/patch"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestClientSecretMutationRoutesSupportDirectAndExplicitKonnectForms(t *testing.T) {
	for name, build := range map[string]func() (*cobra.Command, error){
		"create": create.NewCreateCmd, "delete": del.NewDeleteCmd, "patch": patch.NewPatchCmd,
	} {
		for _, explicit := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "-direct", true: "-explicit"}[explicit], func(t *testing.T) {
				command, err := build()
				require.NoError(t, err)
				args := []string{"identity", "auth-server-client-secret"}
				if explicit {
					args = append([]string{"konnect"}, args...)
				}
				child, rest, err := command.Find(args)
				require.NoError(t, err)
				require.Empty(t, rest)
				require.Equal(t, "auth-server-client-secret", child.Name())
				require.NotNil(t, child.RunE)
				require.NotNil(t, child.Flags().Lookup("auth-server-id"))
				require.NotNil(t, child.Flags().Lookup("client-id"))
				if name == "create" {
					require.NotNil(t, child.Flags().Lookup("secret-output-file"))
				}
				if name == "patch" {
					require.NotNil(t, child.Flags().Lookup("enabled"))
				}
			})
		}
	}
}
