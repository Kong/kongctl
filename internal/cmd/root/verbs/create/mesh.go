package create

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
// giving "kongctl create mesh ..." alongside the explicit
// "kongctl create konnect mesh ..." form.
//
// Under create, mesh serves token issuance only. Resources are written with
// `apply mesh -f`, because a write creates or replaces and that is what apply
// means; a create that silently replaced an existing resource was the reason
// the -f form is not offered here.
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

	meshCmd.Example = fmt.Sprintf(`  # Issue a token that proves a zone's identity to the global control plane
  %[1]s create mesh zone-token --zone zone1 --valid-for 720h --control-plane-id <id>

  # Issue a token that proves a dataplane's identity
  %[1]s create mesh dataplane-token --mesh default --name backend-01 --control-plane-id <id>

  # Mesh resources are written with apply, since a write creates or replaces
  %[1]s apply mesh -f policy.yaml --control-plane-id <id>`, meta.CLIName)

	return meshCmd, nil
}
