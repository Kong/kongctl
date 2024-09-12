package service

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	kkCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/err"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
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

func serviceToDisplayRecord(s *kkComps.Service) textDisplayRecord {
	missing := "n/a"

	name := missing
	if s.Name != nil {
		name = *s.Name
	}

	id := missing
	if s.ID != nil {
		id = *s.ID
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
		return &err.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing gateway services requires 0 or 1 arguments (name or ID)"),
		}
	}

	config, e := helper.GetConfig()
	if e != nil {
		return e
	}

	pageSize := config.GetInt(kkCommon.RequestPageSizeConfigPath)
	if pageSize < 1 {
		return &err.ConfigurationError{
			Err: fmt.Errorf("%s must be greater than 0", kkCommon.RequestPageSizeFlagName),
		}
	}

	return nil
}

func (c *getServiceCmd) runListByName(cpID string, name string,
	kkClient *kk.SDK, helper cmd.Helper, cfg config.Hook, printer cli.Printer, outputFormat cmdCommon.OutputFormat,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))

	allData, e := helpers.GetAllGatewayServices(helper.GetContext(), requestPageSize, cpID, kkClient)
	if e != nil {
		attrs := err.TryConvertErrorToAttrs(e)
		return cmd.PrepareExecutionError("Failed to list Gateway Services", e, helper.GetCmd(), attrs...)
	}

	for _, service := range allData {
		if *service.GetName() == name {
			if outputFormat == cmdCommon.TEXT {
				printer.Print(serviceToDisplayRecord(&service))
			} else {
				printer.Print(service)
			}
		}
	}

	return nil
}

func (c *getServiceCmd) runGet(cpID string, id string,
	kkClient *kk.SDK, helper cmd.Helper, printer cli.Printer, outputFormat cmdCommon.OutputFormat,
) error {
	res, e := kkClient.Services.GetService(helper.GetContext(), cpID, id)
	if e != nil {
		attrs := err.TryConvertErrorToAttrs(e)
		return cmd.PrepareExecutionError("Failed to get Gateway Service", e, helper.GetCmd(), attrs...)
	}

	if outputFormat == cmdCommon.TEXT {
		printer.Print(serviceToDisplayRecord(res.GetService()))
	} else {
		printer.Print(res.GetService())
	}

	return nil
}

func (c *getServiceCmd) runList(cpID string,
	kkClient *kk.SDK, helper cmd.Helper, cfg config.Hook, printer cli.Printer, outputFormat cmdCommon.OutputFormat,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))

	// TODO: Explore streaming of data. We can expect some large data sets, especially for GW entities.
	//		   Right now these functions are loading all data into memory before printing.
	allData, e := helpers.GetAllGatewayServices(helper.GetContext(), requestPageSize, cpID, kkClient)
	if e != nil {
		attrs := err.TryConvertErrorToAttrs(e)
		return cmd.PrepareExecutionError("Failed to list Gateway Services", e, helper.GetCmd(), attrs...)
	}

	if outputFormat == cmdCommon.TEXT {
		var displayRecords []textDisplayRecord
		for _, service := range allData {
			displayRecords = append(displayRecords, serviceToDisplayRecord(&service))
		}
		printer.Print(displayRecords)
	} else {
		printer.Print(allData)
	}

	return nil
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

	printer, e := cli.Format(outType.String(), helper.GetStreams().Out)
	if e != nil {
		return e
	}

	defer printer.Flush()

	kkClient, e := helper.GetKonnectSDK(cfg, logger)
	if e != nil {
		return e
	}

	cpID, e := helpers.GetControlPlaneIDByNameIfNecessary(helper.GetContext(), kkClient.GetControlPlaneAPI(),
		cfg.GetString(common.ControlPlaneIDConfigPath),
		cfg.GetString(common.ControlPlaneNameConfigPath))

	if e != nil {
		var ee *err.ExecutionError
		if errors.As(e, &ee) {
			return cmd.PrepareExecutionError("Failed to get Control Plane ID", ee.Err, helper.GetCmd(), ee.Attrs)
		}
		return e
	}

	// TODO!: Fix up the below casting to Konnect SDKs, as it will fail in testing once that is written.
	//         A service API needs to be added to our internal SDK API interfaces

	// 'get konnect gateway services' can be run like various ways:
	//	> get konnect gateway services <id>    # Get by UUID
	//  > get konnect gateway services <name>	# Get by name
	//  > get konnect gateway services # List all
	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		id := helper.GetArgs()[0]

		isUUID, _ := regexp.MatchString(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`, id)
		// TODO: Is capturing the blanked error necessary?

		if !isUUID {
			// If the ID is not a UUID, then it is a name
			// search for the control plane by name
			return c.runListByName(cpID, id, kkClient.(*helpers.KonnectSDK).SDK, helper, cfg, printer, outType)
		}

		return c.runGet(cpID, id, kkClient.(*helpers.KonnectSDK).SDK, helper, printer, outType)
	}

	return c.runList(cpID, kkClient.(*helpers.KonnectSDK).SDK, helper, cfg, printer, outType)
}

func newGetServiceCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getServiceCmd {
	rv := getServiceCmd{
		Command: baseCmd,
	}

	rv.Short = getServiceShort
	rv.Long = getServiceLong
	rv.Example = getServiceExamples

	if addParentFlags != nil {
		addParentFlags(verb, baseCmd)
	}

	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	return &rv
}
