package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/charmbracelet/bubbles/table"
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

type getServiceCmd struct {
	*cobra.Command
}

// Represents a text display record for a Gateway Service
//
// Because the SDK provids pointers for optional value fields,
// the segmentio/cli printer prints the address instead of the value.
// This will require a decent amount of boilerplate code to convert
// the types to a format that prints how we want.
// TODO: Investigate if there is a way to handle this with less boilerplate
type textDisplayRecord struct {
	Name     string
	Enabled  string
	Host     string
	Path     string
	Port     string
	Protocol string
	Tags     string
	ID       string
}

func serviceToDisplayRecord(s *kkComps.ServiceOutput) textDisplayRecord {
	missing := "n/a"

	name := missing
	if s.Name != nil {
		name = *s.Name
	}

	id := missing
	if s.ID != nil {
		id = util.AbbreviateUUID(*s.ID)
	}

	enabled := missing
	if s.Enabled != nil {
		enabled = strconv.FormatBool(*s.Enabled)
	}

	path := missing
	if s.Path != nil {
		path = *s.Path
	}

	port := missing
	if s.Port != nil {
		port = strconv.FormatInt(*s.Port, 10)
	}

	protocol := missing
	if s.Protocol != nil {
		protocol = string(*s.Protocol)
	}

	tags := missing
	if s.Tags != nil {
		tags = strings.Join(s.Tags, ", ")
	}

	return textDisplayRecord{
		Name:     name,
		ID:       id,
		Enabled:  enabled,
		Host:     s.Host,
		Path:     path,
		Port:     port,
		Protocol: protocol,
		Tags:     tags,
	}
}

func serviceDetailView(s *kkComps.ServiceOutput) string {
	if s == nil {
		return ""
	}

	missing := "n/a"
	name := missing
	if s.Name != nil && *s.Name != "" {
		name = *s.Name
	}

	id := missing
	if s.ID != nil && *s.ID != "" {
		id = *s.ID
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\n", name)
	fmt.Fprintf(&b, "ID: %s\n", id)
	fmt.Fprintf(&b, "Host: %s\n", s.Host)

	port := missing
	if s.Port != nil {
		port = strconv.FormatInt(*s.Port, 10)
	}
	fmt.Fprintf(&b, "Port: %s\n", port)

	path := missing
	if s.Path != nil && *s.Path != "" {
		path = *s.Path
	}
	fmt.Fprintf(&b, "Path: %s\n", path)

	protocol := missing
	if s.Protocol != nil {
		protocol = string(*s.Protocol)
	}
	fmt.Fprintf(&b, "Protocol: %s\n", protocol)

	enabled := missing
	if s.Enabled != nil {
		enabled = strconv.FormatBool(*s.Enabled)
	}
	fmt.Fprintf(&b, "Enabled: %s\n", enabled)

	if len(s.Tags) > 0 {
		fmt.Fprintf(&b, "Tags: %s\n", strings.Join(s.Tags, ", "))
	} else {
		fmt.Fprintf(&b, "Tags: %s\n", missing)
	}

	if s.CreatedAt != nil {
		created := time.Unix(0, *s.CreatedAt*int64(time.Millisecond)).In(time.Local)
		fmt.Fprintf(&b, "Created: %s\n", created.Format("2006-01-02 15:04:05"))
	}
	if s.UpdatedAt != nil {
		updated := time.Unix(0, *s.UpdatedAt*int64(time.Millisecond)).In(time.Local)
		fmt.Fprintf(&b, "Updated: %s\n", updated.Format("2006-01-02 15:04:05"))
	}

	return b.String()
}

var (
	getServiceShort = i18n.T("root.products.konnect.gateway.service.getServiceShort",
		"List or get Konnect Kong Gateway Services")
	getServiceLong = i18n.T("root.products.konnect.gateway.service.getServiceLong",
		`Use the get verb with the service command to query Konnect Kong Gateway Services.`)
	getServiceExamples = normalizers.Examples(
		i18n.T("root.products.konnect.gateway.service.getServiceExamples",
			fmt.Sprintf(`
	# List all the Gateway Services for the a given control plane
	%[1]s get konnect gateway service --control-plane-id <id>
	# Get a specific Kong Gateway Services for the a given control plane
	%[1]s get konnect gateway service --control-plane-id <id> <service-name>
	`, meta.CLIName)))
)

func (c *getServiceCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing gateway services requires 0 or 1 arguments (name or ID)"),
		}
	}

	config, err := helper.GetConfig()
	if err != nil {
		return err
	}

	pageSize := config.GetInt(kkCommon.RequestPageSizeConfigPath)
	if pageSize < 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("%s must be greater than 0", kkCommon.RequestPageSizeFlagName),
		}
	}

	return nil
}

func (c *getServiceCmd) runListByName(
	cpID string,
	name string,
	kkClient *kk.SDK,
	helper cmd.Helper,
	cfg config.Hook,
	interactive bool,
	printer cli.PrintFlusher,
	outputFormat cmdCommon.OutputFormat,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))

	allData, err := helpers.GetAllGatewayServices(helper.GetContext(), requestPageSize, cpID, kkClient)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list Gateway Services", err, helper.GetCmd(), attrs...)
	}

	for _, service := range allData {
		if *service.GetName() == name {
			return tableview.RenderForFormat(
				interactive,
				outputFormat,
				printer,
				helper.GetStreams(),
				serviceToDisplayRecord(&service),
				service,
				"Gateway Service",
				tableview.WithRootLabel(helper.GetCmd().Name()),
			)
		}
	}

	return nil
}

func (c *getServiceCmd) runGet(
	cpID string,
	id string,
	kkClient *kk.SDK,
	helper cmd.Helper,
	interactive bool,
	printer cli.PrintFlusher,
	outputFormat cmdCommon.OutputFormat,
) error {
	res, err := kkClient.Services.GetService(helper.GetContext(), cpID, id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get Gateway Service", err, helper.GetCmd(), attrs...)
	}

	return tableview.RenderForFormat(
		interactive,
		outputFormat,
		printer,
		helper.GetStreams(),
		serviceToDisplayRecord(res.GetService()),
		res.GetService(),
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (c *getServiceCmd) runList(
	cpID string,
	kkClient *kk.SDK,
	helper cmd.Helper,
	cfg config.Hook,
	interactive bool,
	printer cli.PrintFlusher,
	outputFormat cmdCommon.OutputFormat,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))

	// TODO: Explore streaming of data. We can expect some large data sets, especially for GW entities.
	//		   Right now these functions are loading all data into memory before printing.
	allData, err := helpers.GetAllGatewayServices(helper.GetContext(), requestPageSize, cpID, kkClient)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list Gateway Services", err, helper.GetCmd(), attrs...)
	}

	displayRecords := make([]textDisplayRecord, 0, len(allData))
	for i := range allData {
		displayRecords = append(displayRecords, serviceToDisplayRecord(&allData[i]))
	}

	tableRows := make([]table.Row, 0, len(displayRecords))
	for _, record := range displayRecords {
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(allData) {
			return ""
		}
		return serviceDetailView(&allData[index])
	}

	return tableview.RenderForFormat(
		interactive,
		outputFormat,
		printer,
		helper.GetStreams(),
		displayRecords,
		allData,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME"}, tableRows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (c *getServiceCmd) runE(cobraCmd *cobra.Command, args []string) error {
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

	interactive, e := helper.IsInteractive()
	if e != nil {
		return e
	}

	var printer cli.PrintFlusher
	if !interactive {
		printer, e = cli.Format(outType.String(), helper.GetStreams().Out)
		if e != nil {
			return e
		}
		defer printer.Flush()
	}

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

	// TODO!: Fix up the below casting to Konnect SDKs, as it will fail in testing once that is written.
	//         A service API needs to be added to our internal SDK API interfaces

	// 'get konnect gateway services' can be run like various ways:
	//	> get konnect gateway services <id>    # Get by UUID
	//  > get konnect gateway services <name>	# Get by name
	//  > get konnect gateway services # List all
	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		id := helper.GetArgs()[0]

		isUUID := util.IsValidUUID(id)
		// TODO: Is capturing the blanked error necessary?

		if !isUUID {
			// If the ID is not a UUID, then it is a name
			// search for the control plane by name
			return c.runListByName(
				cpID,
				id,
				kkClient.(*helpers.KonnectSDK).SDK,
				helper,
				cfg,
				interactive,
				printer,
				outType,
			)
		}

		return c.runGet(cpID, id, kkClient.(*helpers.KonnectSDK).SDK, helper, interactive, printer, outType)
	}

	return c.runList(cpID, kkClient.(*helpers.KonnectSDK).SDK, helper, cfg, interactive, printer, outType)
}

func newGetServiceCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getServiceCmd {
	rv := getServiceCmd{
		Command: baseCmd,
	}

	baseCmd.Short = getServiceShort
	baseCmd.Long = getServiceLong
	baseCmd.Example = getServiceExamples

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
