package get

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/analytics"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

// NewDirectAnalyticsCmd creates an analytics command that works at the root level (Konnect-first).
func NewDirectAnalyticsCmd() (*cobra.Command, error) {
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

	preRunE := func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, products.Product, konnect.Product)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
		c.SetContext(ctx)

		return bindAnalyticsFlags(c, args)
	}

	analyticsCmd, err := analytics.NewAnalyticsCmd(Verb, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	analyticsCmd.Example = `  # List all analytics dashboards
  kongctl get analytics dashboards
  # Get a specific analytics dashboard by name
  kongctl get analytics dashboard "API Summary"
  # List analytics dashboards using aliases
  kongctl get analytic dashboards`

	return analyticsCmd, nil
}

func bindAnalyticsFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	bindings := []struct {
		flag   string
		config string
	}{
		{common.BaseURLFlagName, common.BaseURLConfigPath},
		{common.RegionFlagName, common.RegionConfigPath},
		{common.PATFlagName, common.PATConfigPath},
		{common.RequestPageSizeFlagName, common.RequestPageSizeConfigPath},
	}

	for _, binding := range bindings {
		if f := c.Flags().Lookup(binding.flag); f != nil {
			if err := cfg.BindFlag(binding.config, f); err != nil {
				return err
			}
		}
	}

	return nil
}
