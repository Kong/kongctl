package adopt

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	konnectadopt "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

func NewDirectPortalCmd() (*cobra.Command, error) {
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

	preRunE := func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, products.Product, konnect.Product)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactory())
		c.SetContext(ctx)
		return bindKonnectFlags(c, args)
	}

	cmd, err := konnectadopt.NewPortalCmd(Verb, &cobra.Command{}, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	cmd.Example = `  # Adopt a portal by name
  kongctl adopt portal my-portal --namespace team-alpha`

	return cmd, nil
}

func NewDirectControlPlaneCmd() (*cobra.Command, error) {
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

	preRunE := func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, products.Product, konnect.Product)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactory())
		c.SetContext(ctx)
		return bindKonnectFlags(c, args)
	}

	cmd, err := konnectadopt.NewControlPlaneCmd(Verb, &cobra.Command{}, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	cmd.Example = `  # Adopt a control plane by UUID
  kongctl adopt control-plane 22cd8a0b-72e7-4212-9099-0764f8e9c5ac --namespace platform`

	return cmd, nil
}

func NewDirectAPICmd() (*cobra.Command, error) {
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

	preRunE := func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, products.Product, konnect.Product)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactory())
		c.SetContext(ctx)
		return bindKonnectFlags(c, args)
	}

	cmd, err := konnectadopt.NewAPICmd(Verb, &cobra.Command{}, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	cmd.Example = `  # Adopt an API by name
  kongctl adopt api payments --namespace team-alpha`

	return cmd, nil
}
func NewDirectEventGatewayCmd() (*cobra.Command, error) {
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

	preRunE := func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, products.Product, konnect.Product)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactory())
		c.SetContext(ctx)
		return bindKonnectFlags(c, args)
	}

	cmd, err := konnectadopt.NewEventGatewayControlPlaneCmd(Verb, &cobra.Command{}, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	cmd.Example = `  # Adopt an Event Gateway by name
  kongctl adopt event-gateway my-egw --namespace team-alpha`

	return cmd, nil
}

func NewDirectAuthStrategyCmd() (*cobra.Command, error) {
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

	preRunE := func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, products.Product, konnect.Product)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactory())
		c.SetContext(ctx)
		return bindKonnectFlags(c, args)
	}

	cmd, err := konnectadopt.NewAuthStrategyCmd(Verb, &cobra.Command{}, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	cmd.Example = `  # Adopt an auth strategy by name
  kongctl adopt auth-strategy key-auth --namespace team-alpha`

	return cmd, nil
}

func bindKonnectFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(common.BaseURLFlagName); flag != nil {
		if err := cfg.BindFlag(common.BaseURLConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := c.Flags().Lookup(common.RegionFlagName); flag != nil {
		if err := cfg.BindFlag(common.RegionConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := c.Flags().Lookup(common.PATFlagName); flag != nil {
		if err := cfg.BindFlag(common.PATConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}
