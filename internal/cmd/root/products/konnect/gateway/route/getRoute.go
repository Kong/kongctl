package route

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	kkCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type textDisplayRecord struct {
	Name             string
	Methods          string
	Paths            string
	Protocols        string
	Tags             string
	LocalCreatedTime string
	LocalUpdatedTime string
	ID               string

	// Destinations          string
	// Headers               string
	// Hosts                 string
	// HTTPSRedirectStatusCode string
	// Service               string
	// PathHandling      string
	// PreserveHost      string
	// RegexPriority     string
	// RequestBuffering  string
	// ResponseBuffering string
	// Snis              string
	// Sources           string
	// StripPath         string
}

func routeToDisplayRecord(r *kkComps.Route) textDisplayRecord {
	missing := "n/a"

	name := missing
	if r.Name != nil {
		name = *r.Name
	}

	methods := missing
	if r.Methods != nil {
		methods = strings.Join(r.Methods, ", ")
	}

	paths := missing
	if r.Paths != nil {
		paths = strings.Join(r.Paths, ", ")
	}

	protocols := missing
	if r.Protocols != nil {
		protocolsArr := make([]string, len(r.Protocols))
		for i, protocol := range r.Protocols {
			protocolsArr[i] = string(protocol)
		}
		protocols = strings.Join(protocolsArr, ", ")
	}

	tags := missing
	if r.Tags != nil {
		tags = strings.Join(r.Tags, ", ")
	}

	createdAt := missing
	if r.CreatedAt != nil {
		createdAt = time.Unix(0, *r.CreatedAt*int64(time.Millisecond)).In(time.Local).Format("2006-01-02 15:04:05")
	}

	updatedAt := missing
	if r.UpdatedAt != nil {
		updatedAt = time.Unix(0, *r.UpdatedAt*int64(time.Millisecond)).In(time.Local).Format("2006-01-02 15:04:05")
	}

	id := missing
	if r.ID != nil {
		id = *r.ID
	}

	return textDisplayRecord{
		Name:             name,
		Methods:          methods,
		Paths:            paths,
		Protocols:        protocols,
		Tags:             tags,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
		ID:               id,
	}
}

type getRouteCmd struct {
	*cobra.Command
}

var (
	getRouteShort = i18n.T("root.products.konnect.gateway.route.getRouteShort",
		"List or get Konnect Kong Gateway Routes")
	getRouteLong = i18n.T("root.products.konnect.gateway.service.getServiceLong",
		`Use the get verb with the route command to query Konnect Kong Gateway Routes.`)
	getRouteExamples = normalizers.Examples(
		i18n.T("root.products.konnect.gateway.route.getRouteExamples",
			fmt.Sprintf(`
	# List all the Kong Gateway Routes for the a given Control Plane (by ID)
	%[1]s get konnect gateway routes --control-plane-id <id>
	# List all the Kong Gateway Routes for the a given Control Plane (by name)
	%[1]s get konnect gateway routes --control-plane-name <name>
	# Get a specific Kong Gateway Routes located on the given Control Plane (by name)
	%[1]s get konnect gateway route --control-plane-name <name> <route-name>
	`, meta.CLIName)))
)

func (c *getRouteCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing gateway routes requires 0 or 1 arguments (name or ID)"),
		}
	}
	return nil
}

func (c *getRouteCmd) runListByName(cpID string, name string,
	kkClient *kk.SDK, helper cmd.Helper, cfg config.Hook, printer cli.Printer, outputFormat cmdCommon.OutputFormat,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))

	allData, err := helpers.GetAllGatewayRoutes(helper.GetContext(), requestPageSize, cpID, kkClient)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list Gateway Routes", err, helper.GetCmd(), attrs...)
	}

	for _, route := range allData {
		if *route.GetName() == name {
			if outputFormat == cmdCommon.TEXT {
				printer.Print(routeToDisplayRecord(&route))
			} else {
				printer.Print(route)
			}
		}
	}

	return nil
}

func (c *getRouteCmd) runGet(cpID string, id string,
	kkClient *kk.SDK, helper cmd.Helper, printer cli.Printer, outputFormat cmdCommon.OutputFormat,
) error {
	res, err := kkClient.Routes.GetRoute(helper.GetContext(), cpID, id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get Gateway Route", err, helper.GetCmd(), attrs...)
	}

	if outputFormat == cmdCommon.TEXT {
		printer.Print(routeToDisplayRecord(res.GetRoute()))
	} else {
		printer.Print(res.GetRoute())
	}

	return nil
}

func (c *getRouteCmd) runList(cpID string,
	kkClient *kk.SDK, helper cmd.Helper, cfg config.Hook, printer cli.Printer, outputFormat cmdCommon.OutputFormat,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))

	allData, err := helpers.GetAllGatewayRoutes(helper.GetContext(), requestPageSize, cpID, kkClient)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list Gateway Routes", err, helper.GetCmd(), attrs...)
	}

	if outputFormat == cmdCommon.TEXT {
		var displayRecords []textDisplayRecord
		for _, route := range allData {
			displayRecords = append(displayRecords, routeToDisplayRecord(&route))
		}
		printer.Print(displayRecords)
	} else {
		printer.Print(allData)
	}

	return nil
}

func (c *getRouteCmd) runE(cobraCmd *cobra.Command, args []string) error {
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

	kkFactory := helper.GetKonnectSDKFactory()
	kkClient, err := kkFactory(cfg, logger)
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

	// 'get konnect gateway routes ' can be run like various ways:
	//	> get konnect gateway routes <id>    # Get by UUID
	//  > get konnect gateway routes <name>	# Get by name
	//  > get konnect gateway routes # List all
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

func newGetRouteCmd(baseCmd *cobra.Command) *getRouteCmd {
	rv := getRouteCmd{
		Command: baseCmd,
	}

	baseCmd.Short = getRouteShort
	baseCmd.Long = getRouteLong
	baseCmd.Example = getRouteExamples
	baseCmd.RunE = rv.runE

	return &rv
}
