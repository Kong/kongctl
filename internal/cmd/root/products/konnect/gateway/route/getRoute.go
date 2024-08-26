package route

import (
	"fmt"
	"regexp"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	"github.com/kong/kong-cli/internal/cmd"
	kkCommon "github.com/kong/kong-cli/internal/cmd/root/products/konnect/common"
	"github.com/kong/kong-cli/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kong-cli/internal/config"
	"github.com/kong/kong-cli/internal/konnect/auth"
	"github.com/kong/kong-cli/internal/konnect/helpers"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

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
	kkClient *kk.SDK, helper cmd.Helper, cfg config.Hook, printer cli.Printer,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))

	allData, err := helpers.GetAllGatewayRoutes(helper.GetContext(), requestPageSize, cpID, kkClient)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list Gateway Routes", err, helper.GetCmd(), attrs...)
	}

	for _, route := range allData {
		if *route.GetName() == name {
			printer.Print(route)
		}
	}

	return nil
}

func (c *getRouteCmd) runGet(cpID string, id string,
	kkClient *kk.SDK, helper cmd.Helper, printer cli.Printer,
) error {
	res, err := kkClient.Routes.GetRoute(helper.GetContext(), cpID, id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get Gateway Route", err, helper.GetCmd(), attrs...)
	}

	printer.Print(res.GetRoute())

	return nil
}

func (c *getRouteCmd) runList(cpID string,
	kkClient *kk.SDK, helper cmd.Helper, cfg config.Hook, printer cli.Printer,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))

	allData, err := helpers.GetAllGatewayRoutes(helper.GetContext(), requestPageSize, cpID, kkClient)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list Gateway Routes", err, helper.GetCmd(), attrs...)
	}

	printer.Print(allData)

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

	token, e := kkCommon.GetAccessToken(cfg, logger)
	if e != nil {
		return fmt.Errorf(
			`no access token available. Use "%s login konnect" to authenticate or provide a Konnect PAT using the --pat flag`,
			meta.CLIName)
	}

	kkClient, err := auth.GetAuthenticatedClient(token)
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
		cpID, err = helpers.GetControlPlaneID(helper.GetContext(), kkClient, cpName)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return cmd.PrepareExecutionError("Failed to get Control Plane ID", err, helper.GetCmd(), attrs...)
		}
	}

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
			return c.runListByName(cpID, id, kkClient, helper, cfg, printer)
		}

		return c.runGet(cpID, id, kkClient, helper, printer)
	}

	return c.runList(cpID, kkClient, helper, cfg, printer)
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
