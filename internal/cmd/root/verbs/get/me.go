package get

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/me"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

// NewDirectMeCmd creates a me command that works at the root level (Konnect-first)
func NewDirectMeCmd() (*cobra.Command, error) {
	// Define the addFlags function to add Konnect-specific flags
	addFlags := func(_ verbs.VerbValue, cmd *cobra.Command) {
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
	}

	// Define the preRunE function to set up Konnect context
	preRunE := func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, products.Product, konnect.Product)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
		c.SetContext(ctx)

		// Bind flags
		return bindMeFlags(c, args)
	}

	// Create the me command using the me package
	meCmd, err := me.NewMeCmd(Verb, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	// Set example for direct usage
	meCmd.Example = `  # Get current user information
  kongctl get me`

	return meCmd, nil
}

// bindMeFlags binds Konnect-specific flags to configuration
func bindMeFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	f := c.Flags().Lookup(common.BaseURLFlagName)
	if f != nil {
		err = cfg.BindFlag(common.BaseURLConfigPath, f)
		if err != nil {
			return err
		}
	}

	f = c.Flags().Lookup(common.RegionFlagName)
	if f != nil {
		err = cfg.BindFlag(common.RegionConfigPath, f)
		if err != nil {
			return err
		}
	}

	f = c.Flags().Lookup(common.PATFlagName)
	if f != nil {
		err = cfg.BindFlag(common.PATConfigPath, f)
		if err != nil {
			return err
		}
	}

	return nil
}
