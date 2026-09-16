package apply

import (
	"context"
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/jq"
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

// NewDirectMeshCmd creates a mesh command that works at the root level, giving
// "kongctl apply mesh ..." alongside the explicit
// "kongctl apply konnect mesh ..." form.
//
// Apply is the primary verb for sending mesh resources. A write addresses a
// resource by type and name and creates or replaces it, which is what kumactl
// calls apply and implements as an upsert, and it matches what apply already
// means in kongctl: create or update, without deleting anything. `create mesh`
// remains for the name this first shipped under.
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

		if err := bindMeshKonnectFlags(c, args); err != nil {
			return err
		}

		helper := cmdpkg.BuildHelper(c, args)
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

	meshCmd.Example = fmt.Sprintf(`  # Apply mesh resources from a file
  %[1]s apply mesh -f policy.yaml --control-plane-id <id>

  # Apply every resource in a directory
  %[1]s apply mesh -f ./policies --control-plane-id <id>

  # Apply from stdin, which is how an export is reapplied
  %[1]s dump mesh --control-plane-id <id> | %[1]s apply mesh -f - --control-plane-id <id>`, meta.CLIName)

	return meshCmd, nil
}

// bindMeshKonnectFlags binds the Konnect connection flags the mesh command
// carries. The apply verb's own tree is declarative and binds elsewhere, so
// this is scoped to the mesh command rather than shared.
func bindMeshKonnectFlags(c *cobra.Command, args []string) error {
	helper := cmdpkg.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	bindings := []struct{ flag, path string }{
		{common.BaseURLFlagName, common.BaseURLConfigPath},
		{common.RegionFlagName, common.RegionConfigPath},
		{common.PATFlagName, common.PATConfigPath},
	}
	for _, b := range bindings {
		if f := c.Flags().Lookup(b.flag); f != nil {
			if err := cfg.BindFlag(b.path, f); err != nil {
				return err
			}
		}
	}

	return jq.BindFlags(cfg, c.Flags())
}
