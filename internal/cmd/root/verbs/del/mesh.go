package del

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/spf13/cobra"
)

// NewDirectMeshCmd creates a mesh command that works at the root level,
// giving "kongctl delete mesh ..." alongside the explicit
// "kongctl delete konnect mesh ..." form.
//
// Reaching Kong Mesh through one command path regardless of whether the control
// plane is Konnect hosted or self managed is deliberate: where a control plane
// runs is a connection detail, not a different command.
func NewDirectMeshCmd() (*cobra.Command, error) {
	addFlags := func(_ verbs.VerbValue, cmdObj *cobra.Command) {
		cmdObj.Flags().String(common.BaseURLFlagName, "",
			fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
				common.BaseURLConfigPath, common.BaseURLDefault))

		cmdObj.Flags().String(common.PATFlagName, "",
			fmt.Sprintf(`Konnect Personal Access Token.
- Config path: [ %s ]`, common.PATConfigPath))
	}

	preRunE := func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, products.Product, konnect.Product)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
		c.SetContext(ctx)

		if err := bindKonnectFlags(c, args); err != nil {
			return err
		}

		// Mesh flags are bound by the mesh command itself, which chains this
		// pre-run, so both command trees bind identically.
		return nil
	}

	meshCmd, err := mesh.NewMeshCmd(Verb, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	meshCmd.Example = fmt.Sprintf(`  # Delete a mesh scoped resource
  %[1]s delete mesh meshtrafficpermission allow-all --control-plane-id <id>

  # Delete a resource in a named mesh
  %[1]s delete mesh meshtimeout slow -m prod --control-plane-id <id>`, meta.CLIName)

	return meshCmd, nil
}
