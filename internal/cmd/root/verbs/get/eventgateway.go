package get

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/eventgateway"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

// NewDirectEventGatewayCmd creates an Event Gateway Control Plane command that works at the root level (Konnect-first)
func NewDirectEventGatewayCmd() (*cobra.Command, error) {
	// Define the addFlags function to add Konnect-specific flags
	addFlags := func(verb verbs.VerbValue, cmd *cobra.Command) {
		cmd.Flags().String(common.BaseURLFlagName, "",
			fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
				common.BaseURLConfigPath, common.BaseURLDefault))

		cmd.Flags().String(common.RegionFlagName, "",
			fmt.Sprintf(`Konnect region identifier (for example "eu"). Used to construct the base URL when --%s is not provided.
- Config path: [ %s ]`,
				common.BaseURLFlagName, common.RegionConfigPath),
		)

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
		return bindEventGatewayFlags(c, args)
	}

	// Create the Event Gateway Control Plane command using the existing api package
	eventGatewayCmd, err := eventgateway.NewEventGatewayCmd(verbs.Get, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	// Override the example to show direct usage without "konnect"
	eventGatewayCmd.Example = `  # List all the Event Gateways for the organization
  kongctl get event-gateways
  # Get a specific Event Gateway
  kongctl get event-gateways <id|name>
  `

	return eventGatewayCmd, nil
}

// bindEventGatewayFlags binds Konnect-specific flags to configuration
func bindEventGatewayFlags(c *cobra.Command, args []string) error {
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

	f = c.Flags().Lookup(common.RequestPageSizeFlagName)
	if f != nil {
		err = cfg.BindFlag(common.RequestPageSizeConfigPath, f)
		if err != nil {
			return err
		}
	}

	return nil
}
