package get

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	konnectauditlogs "github.com/kong/kongctl/internal/cmd/root/products/konnect/auditlogs"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

// NewDirectAuditLogsCmd creates an audit-logs command that works at the root level (Konnect-first).
func NewDirectAuditLogsCmd() (*cobra.Command, error) {
	addFlags := func(verb verbs.VerbValue, cmdObj *cobra.Command) {
		cmdObj.Flags().String(common.BaseURLFlagName, "",
			fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
				common.BaseURLConfigPath, common.BaseURLDefault))

		cmdObj.Flags().String(
			common.RegionFlagName, "",
			fmt.Sprintf(`Konnect region identifier (for example "eu"). Used to construct the base URL when --%s is not provided.
- Config path: [ %s ]`,
				common.BaseURLFlagName, common.RegionConfigPath),
		)

		cmdObj.Flags().String(common.PATFlagName, "",
			fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI.
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
				common.PATConfigPath))

		if verb == verbs.Get || verb == verbs.List {
			cmdObj.Flags().Int(
				common.RequestPageSizeFlagName,
				konnectauditlogs.DefaultPullPageSize,
				fmt.Sprintf(`Maximum audit-log records requested per API page (1..1000).
- Config path: [ %s ]`,
					common.RequestPageSizeConfigPath),
			)
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
		return bindKonnectFlags(c, args)
	}

	auditLogsCmd, err := konnectauditlogs.NewAuditLogsCmd(Verb, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	auditLogsCmd.Example = `  # Retrieve the 50 most recent events
  kongctl get audit-logs

  # Retrieve and automatically paginate every event from the last 24 hours
  kongctl get audit-logs --since 24h --output jsonl

  # List audit-log destinations
  kongctl get audit-logs destinations

  # Get one destination by id or name
  kongctl get audit-logs destination <id|name>

  # Get regional webhook configuration
  kongctl get audit-logs webhook`

	return auditLogsCmd, nil
}
