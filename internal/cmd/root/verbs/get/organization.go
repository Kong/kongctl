package get

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	orgcmd "github.com/kong/kongctl/internal/cmd/root/products/konnect/organization"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

// NewDirectOrganizationCmd creates an organization command that works at the root level (Konnect-first)
func NewDirectOrganizationCmd() (*cobra.Command, error) {
	addFlags := func(_ verbs.VerbValue, cmd *cobra.Command) {
		cmd.Flags().String(common.BaseURLFlagName, common.BaseURLDefault,
			fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]`,
				common.BaseURLConfigPath))

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
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
		c.SetContext(ctx)

		return bindOrganizationFlags(c, args)
	}

	orgCmd, err := orgcmd.NewOrganizationCmd(Verb, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	orgCmd.Example = `  # Get current organization information
  kongctl get organization`

	return orgCmd, nil
}

func bindOrganizationFlags(c *cobra.Command, args []string) error {
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

	if f := c.Flags().Lookup(common.PATFlagName); f != nil {
		if err := cfg.BindFlag(common.PATConfigPath, f); err != nil {
			return err
		}
	}

	return nil
}
