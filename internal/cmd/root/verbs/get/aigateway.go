package get

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/aigateway"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

// NewDirectAIGatewayCmd creates an AI Gateway command that works at the root level.
func NewDirectAIGatewayCmd() (*cobra.Command, error) {
	addFlags := addAIGatewayFlags
	preRunE := func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, products.Product, konnect.Product)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
		c.SetContext(ctx)

		return bindAIGatewayFlags(c, args)
	}

	aiGatewayCmd, err := aigateway.NewAIGatewayCmd(Verb, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	aiGatewayCmd.Example = `  # List all AI Gateways
  kongctl get ai-gateways
  # Get a specific AI Gateway by ID or display name
  kongctl get ai-gateway <id|display-name>
  # List AI Gateways using aliases
  kongctl get aigw`

	return aiGatewayCmd, nil
}

func addAIGatewayFlags(verb verbs.VerbValue, cmd *cobra.Command) {
	cmd.Flags().String(common.BaseURLFlagName, "",
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
			common.BaseURLConfigPath, common.BaseURLDefault))

	cmd.Flags().String(
		common.RegionFlagName, "",
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
				common.RequestPageSizeConfigPath),
		)
	}
}

func bindAIGatewayFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if f := c.Flags().Lookup(common.BaseURLFlagName); f != nil {
		if err := cfg.BindFlag(common.BaseURLConfigPath, f); err != nil {
			return err
		}
	}

	if f := c.Flags().Lookup(common.RegionFlagName); f != nil {
		if err := cfg.BindFlag(common.RegionConfigPath, f); err != nil {
			return err
		}
	}

	if f := c.Flags().Lookup(common.PATFlagName); f != nil {
		if err := cfg.BindFlag(common.PATConfigPath, f); err != nil {
			return err
		}
	}

	if f := c.Flags().Lookup(common.RequestPageSizeFlagName); f != nil {
		if err := cfg.BindFlag(common.RequestPageSizeConfigPath, f); err != nil {
			return err
		}
	}

	return nil
}
