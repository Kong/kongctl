package get

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/spf13/cobra"
)

// NewDirectMeshCmd creates a mesh command that works at the root level,
// giving "kongctl get mesh ..." alongside the explicit
// "kongctl get konnect mesh ..." form.
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

		helper := cmd.BuildHelper(c, args)
		cfg, err := helper.GetConfig()
		if err != nil {
			return err
		}
		return meshcommon.BindFlags(cfg, c.Flags())
	}

	meshCmd, err := mesh.NewMeshCmd(Verb, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	meshCmd.Example = fmt.Sprintf(`  # List the resource types a control plane serves
  %[1]s get mesh resource-types --control-plane-id <id>

  # List dataplanes without specifying the product
  %[1]s get mesh dataplanes --control-plane-id <id>`, meta.CLIName)

	return meshCmd, nil
}
