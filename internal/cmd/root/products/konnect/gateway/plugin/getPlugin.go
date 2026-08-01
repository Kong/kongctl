package plugin

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	kk "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	kkCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type getPluginCmd struct {
	*cobra.Command
}

var (
	getPluginShort = i18n.T("root.products.konnect.gateway.plugin.getPluginShort",
		"List or get Konnect Kong Gateway Plugins")
	getPluginLong = i18n.T("root.products.konnect.gateway.plugin.getPluginLong",
		`Use the get verb with the plugin command to query Konnect Kong Gateway Plugins.`)
	getPluginExamples = normalizers.Examples(
		i18n.T("root.products.konnect.gateway.plugin.getPluginExamples",
			fmt.Sprintf(`
	# List all Kong Gateway Plugins for a given control plane (by ID)
	%[1]s get konnect gateway control-plane plugins --control-plane-id <id>
	# List all Kong Gateway Plugins for a given control plane (by name)
	%[1]s get konnect gateway control-plane plugins --control-plane-name <name>
	# Get a specific Kong Gateway Plugin located on the given control plane (by name)
	%[1]s get konnect gateway control-plane plugin --control-plane-name <name> <plugin-name>
	`, meta.CLIName)),
	)
)

func (c *getPluginCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing gateway plugins requires 0 or 1 arguments (name or ID)"),
		}
	}
	return nil
}

func (c *getPluginCmd) runList(
	cpID string,
	kkClient *kk.SDK,
	helper cmd.Helper,
	cfg config.Hook,
	printer cli.PrintFlusher,
	outputFormat cmdCommon.OutputFormat,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))
	allData, err := helpers.GetAllGatewayPlugins(helper.GetContext(), requestPageSize, cpID, kkClient)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list Gateway Plugins", err, helper.GetCmd(), attrs...)
	}

	displayRecords := make([]pluginDisplayRecord, 0, len(allData))
	rows := make([]table.Row, 0, len(allData))
	for i := range allData {
		record := pluginToDisplayRecord(&allData[i])
		displayRecords = append(displayRecords, record)
		rows = append(rows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(allData) {
			return ""
		}
		return pluginDetailView(&allData[index])
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outputFormat,
		printer,
		helper.GetStreams(),
		displayRecords,
		allData,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME"}, rows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (c *getPluginCmd) runGet(
	cpID string,
	id string,
	kkClient *kk.SDK,
	helper cmd.Helper,
	printer cli.PrintFlusher,
	outputFormat cmdCommon.OutputFormat,
) error {
	res, err := kkClient.Plugins.GetPlugin(helper.GetContext(), kkOps.GetPluginRequest{
		ControlPlaneID: cpID,
		PluginID:       id,
	})
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get Gateway Plugin", err, helper.GetCmd(), attrs...)
	}

	plugin := res.GetPlugin()
	if plugin == nil {
		return &cmd.ExecutionError{
			Msg: "Gateway plugin response was empty",
			Err: fmt.Errorf("no plugin returned for id %s", id),
		}
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outputFormat,
		printer,
		helper.GetStreams(),
		pluginToDisplayRecord(plugin),
		plugin,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (c *getPluginCmd) runListByName(
	cpID string,
	name string,
	kkClient *kk.SDK,
	helper cmd.Helper,
	cfg config.Hook,
	printer cli.PrintFlusher,
	outputFormat cmdCommon.OutputFormat,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))
	allData, err := helpers.GetAllGatewayPlugins(helper.GetContext(), requestPageSize, cpID, kkClient)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list Gateway Plugins", err, helper.GetCmd(), attrs...)
	}

	for i := range allData {
		if strings.EqualFold(allData[i].GetName(), name) {
			return tableview.RenderForFormat(
				helper,
				false,
				outputFormat,
				printer,
				helper.GetStreams(),
				pluginToDisplayRecord(&allData[i]),
				allData[i],
				"",
				tableview.WithRootLabel(helper.GetCmd().Name()),
			)
		}
	}

	return &cmd.ConfigurationError{
		Err: fmt.Errorf("gateway plugin %q not found", name),
	}
}

func (c *getPluginCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if e := c.validate(helper); e != nil {
		return e
	}

	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	logger, e := helper.GetLogger()
	if e != nil {
		return e
	}

	outType, e := helper.GetOutputFormat()
	if e != nil {
		return e
	}

	printer, e := cli.Format(outType.String(), helper.GetStreams().Out)
	if e != nil {
		return e
	}
	defer printer.Flush()

	kkClient, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	cpID := cfg.GetString(common.ControlPlaneIDConfigPath)
	if cpID == "" {
		cpName := cfg.GetString(common.ControlPlaneNameConfigPath)
		if cpName == "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("control plane ID or name is required"),
			}
		}
		var err error
		cpID, err = helpers.GetControlPlaneID(helper.GetContext(), kkClient.GetControlPlaneAPI(), cpName)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return cmd.PrepareExecutionError("Failed to get Control Plane ID", err, helper.GetCmd(), attrs...)
		}
	}

	internalSDK := kkClient.(*helpers.KonnectSDK).SDK
	if len(helper.GetArgs()) == 1 {
		identifier := strings.TrimSpace(helper.GetArgs()[0])
		if util.IsValidUUID(identifier) {
			return c.runGet(cpID, identifier, internalSDK, helper, printer, outType)
		}
		return c.runListByName(cpID, identifier, internalSDK, helper, cfg, printer, outType)
	}

	return c.runList(cpID, internalSDK, helper, cfg, printer, outType)
}

func newGetPluginCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getPluginCmd {
	rv := getPluginCmd{
		Command: baseCmd,
	}

	baseCmd.Short = getPluginShort
	baseCmd.Long = getPluginLong
	baseCmd.Example = getPluginExamples

	if addParentFlags != nil {
		addParentFlags(verb, baseCmd)
	}

	originalPreRunE := baseCmd.PreRunE
	baseCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if parentPreRun != nil {
			if err := parentPreRun(cmd, args); err != nil {
				return err
			}
		}
		if originalPreRunE != nil {
			if err := originalPreRunE(cmd, args); err != nil {
				return err
			}
		}
		return nil
	}
	baseCmd.RunE = rv.runE

	return &rv
}
