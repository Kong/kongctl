package gateway

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
)

// AddGatewayFlags registers standard Konnect gateway flags on cmd.
// Pass the current verb so the page-size flag is added only for Get/List.
func AddGatewayFlags(verb verbs.VerbValue, c *cobra.Command) {
	c.Flags().String(common.BaseURLFlagName, "",
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
			common.BaseURLConfigPath, common.BaseURLDefault))

	c.Flags().String(common.RegionFlagName, "",
		fmt.Sprintf(`Konnect region identifier (for example "eu"). Used to construct the base URL when --%s is not provided.
- Config path: [ %s ]`,
			common.BaseURLFlagName, common.RegionConfigPath))

	c.Flags().String(common.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI. 
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			common.PATConfigPath))

	if verb == verbs.Get || verb == verbs.List {
		c.Flags().Int(
			common.RequestPageSizeFlagName,
			common.DefaultRequestPageSize,
			fmt.Sprintf(`Max number of results to include per response page for get and list operations.
- Config path: [ %s ]`,
				common.RequestPageSizeConfigPath))
	}
}

// BindGatewayFlags binds registered gateway flags to their config paths.
func BindGatewayFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	bindings := []struct{ flag, config string }{
		{common.BaseURLFlagName, common.BaseURLConfigPath},
		{common.RegionFlagName, common.RegionConfigPath},
		{common.PATFlagName, common.PATConfigPath},
		{common.RequestPageSizeFlagName, common.RequestPageSizeConfigPath},
	}

	for _, b := range bindings {
		if f := c.Flags().Lookup(b.flag); f != nil {
			if err = cfg.BindFlag(b.config, f); err != nil {
				return err
			}
		}
	}
	return nil
}
