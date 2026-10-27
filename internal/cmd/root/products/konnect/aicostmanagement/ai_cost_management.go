package aicostmanagement

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

// NewCommand exposes the regional organization-wide cost management configuration.
func NewCommand(
	verb verbs.VerbValue,
	addFlags func(verbs.VerbValue, *cobra.Command),
	preRun func(*cobra.Command, []string) error,
) *cobra.Command {
	root := &cobra.Command{Use: "ai-cost-management", Short: "Manage regional AI cost capture"}
	if verb != verbs.Get && verb != verbs.List {
		return root
	}
	child := &cobra.Command{
		Use: "tag-capture", Short: "Get organization-wide cost-tag capture in the selected region",
		Args: cobra.NoArgs, PreRunE: preRun,
		RunE: func(c *cobra.Command, args []string) error {
			helper := cmd.BuildHelper(c, args)
			cfg, err := helper.GetConfig()
			if err != nil {
				return err
			}
			logger, err := helper.GetLogger()
			if err != nil {
				return err
			}
			sdk, err := helper.GetKonnectSDK(cfg, logger)
			if err != nil {
				return err
			}
			api := sdk.GetAICostManagementTagsAPI()
			if api == nil {
				return fmt.Errorf("AI cost management tags client is not configured")
			}
			response, err := api.GetCostManagementTags(helper.GetContext())
			if err != nil {
				return err
			}
			if response == nil || response.CostManagementTags == nil {
				return fmt.Errorf("AI cost management tags response missing data")
			}
			format, err := helper.GetOutputFormat()
			if err != nil {
				return err
			}
			printer, err := cli.Format(format.String(), helper.GetStreams().Out)
			if err != nil {
				return err
			}
			defer printer.Flush()
			value := response.CostManagementTags
			return tableview.RenderForFormat(helper, false, format, printer, helper.GetStreams(),
				struct {
					Headers  int `json:"header_count"`
					Prefixes int `json:"prefix_count"`
				}{len(value.Headers), len(value.Prefixes)}, value, "",
				tableview.WithRootLabel("tag-capture"))
		},
	}
	if addFlags != nil {
		addFlags(verb, child)
	}
	root.AddCommand(child)
	return root
}
