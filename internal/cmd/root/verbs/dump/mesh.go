package dump

import (
	"context"
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/spf13/cobra"
)

// newMeshCmd creates the mesh export command, giving "kongctl dump mesh".
//
// The export writes a YAML stream rather than a table, so it does not take the
// output format flag the other mesh commands accept — matching the dump verb,
// which rejects that flag for every subcommand.
func newMeshCmd() (*cobra.Command, error) {
	addFlags := func(_ verbs.VerbValue, cmdObj *cobra.Command) {
		cmdObj.Flags().String(konnectCommon.BaseURLFlagName, "",
			fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
				konnectCommon.BaseURLConfigPath, konnectCommon.BaseURLDefault))

		cmdObj.Flags().String(konnectCommon.PATFlagName, "",
			fmt.Sprintf(`Konnect Personal Access Token.
- Config path: [ %s ]`, konnectCommon.PATConfigPath))
	}

	preRunE := func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		// products.ProductKonnect rather than konnect.Product: the konnect
		// package cannot be imported here without an import cycle, since the
		// navigator it pulls in imports this one.
		ctx = context.WithValue(ctx, products.Product, products.ProductKonnect)
		ctx = context.WithValue(ctx,
			helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(konnectCommon.KonnectSDKFactory))
		c.SetContext(ctx)

		helper := cmdpkg.BuildHelper(c, args)
		cfg, err := helper.GetConfig()
		if err != nil {
			return err
		}

		bindings := []struct{ flag, configPath string }{
			{konnectCommon.BaseURLFlagName, konnectCommon.BaseURLConfigPath},
			{konnectCommon.PATFlagName, konnectCommon.PATConfigPath},
		}
		for _, b := range bindings {
			if err := verbs.BindFlag(cfg, c.Flags(), b.flag, b.configPath); err != nil {
				return err
			}
		}

		return meshcommon.BindFlags(cfg, c.Flags())
	}

	meshCmd, err := mesh.NewMeshCmd(Verb, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	meshCmd.Example = fmt.Sprintf(`  # Export what a new global control plane needs
  %[1]s dump mesh --control-plane-id <id> > mesh.yaml

  # Include the targetRef policies
  %[1]s dump mesh --profile federation-with-policies --control-plane-id <id>

  # Export everything except dataplanes
  %[1]s dump mesh --profile no-dataplanes --control-plane-id <id>

  # Apply an export back to another control plane
  %[1]s create mesh -f mesh.yaml --control-plane-id <other>`, meta.CLIName)

	return meshCmd, nil
}
