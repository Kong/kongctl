package list

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/portal"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

// NewDirectPortalCmd creates a portal command that can be used directly under the list verb
// This enables the Konnect-first pattern: kongctl list portals
func NewDirectPortalCmd() (*cobra.Command, error) {
	// Create the portal command with the same logic as the konnect portal command
	baseCmd, err := portal.NewPortalCmd(verbs.List, addKonnectFlags, konnectPreRunE)
	if err != nil {
		return nil, err
	}

	// Override the example to show direct usage
	baseCmd.Example = `  # List all the Konnect portals for the organization
  kongctl list portals`

	return baseCmd, nil
}

// addKonnectFlags adds Konnect-specific flags to the portal command
func addKonnectFlags(_ verbs.VerbValue, cmd *cobra.Command) {
	cmd.Flags().String(common.BaseURLFlagName, common.BaseURLDefault,
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]`,
			common.BaseURLConfigPath))

	cmd.Flags().String(common.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI. 
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			common.PATConfigPath))

	cmd.Flags().Int(
		common.RequestPageSizeFlagName,
		common.DefaultRequestPageSize,
		fmt.Sprintf(`Max number of results to include per response page for get and list operations.
- Config path: [ %s ]`,
			common.RequestPageSizeConfigPath))
}

// konnectPreRunE sets up the Konnect context and binds flags
func konnectPreRunE(c *cobra.Command, args []string) error {
	ctx := c.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	
	// Set the Konnect product context
	ctx = context.WithValue(ctx, products.Product, konnect.Product)
	ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
	c.SetContext(ctx)
	
	// Bind flags
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	f := c.Flags().Lookup(common.BaseURLFlagName)
	if err := cfg.BindFlag(common.BaseURLConfigPath, f); err != nil {
		return err
	}

	f = c.Flags().Lookup(common.PATFlagName)
	if f != nil {
		if err := cfg.BindFlag(common.PATConfigPath, f); err != nil {
			return err
		}
	}

	f = c.Flags().Lookup(common.RequestPageSizeFlagName)
	if f != nil {
		if err := cfg.BindFlag(common.RequestPageSizeConfigPath, f); err != nil {
			return err
		}
	}

	return nil
}