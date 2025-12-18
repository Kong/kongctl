package get

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/catalog"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

// NewDirectCatalogCmd creates a catalog command that works at the root level (Konnect-first)
func NewDirectCatalogCmd() (*cobra.Command, error) {
	addFlags := func(verb verbs.VerbValue, cmd *cobra.Command) {
		cmd.Flags().String(common.BaseURLFlagName, "",
			fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
				common.BaseURLConfigPath, common.BaseURLDefault))

		cmd.Flags().String(common.RegionFlagName, "",
			fmt.Sprintf(`Konnect region identifier (for example "eu"). Used to construct the base URL when --%s is not provided.
- Config path: [ %s ]`,
				common.BaseURLFlagName, common.RegionConfigPath))

		cmd.Flags().String(common.PATFlagName, "",
			fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI. 
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
				common.PATConfigPath))

		if verb == verbs.Get || verb == verbs.List {
			cmd.Flags().Int(
				common.RequestPageSizeFlagName,
				common.DefaultRequestPageSize,
				fmt.Sprintf(`Max number of results to include per response page for get and list operations.
- Config path: [ %s ]`,
					common.RequestPageSizeConfigPath))
		}
	}

	preRunE := func(c *cobra.Command, _ []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, products.Product, konnect.Product)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
		c.SetContext(ctx)

		return nil
	}

	catalogCmd, err := catalog.NewCatalogCmd(Verb, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	catalogCmd.Example = `  # List catalog services
  kongctl get catalog services
  # Get a catalog service by name
  kongctl get catalog service payments`

	return catalogCmd, nil
}
