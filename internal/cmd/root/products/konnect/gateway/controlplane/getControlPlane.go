package controlplane

import (
	"context"
	"fmt"
	"regexp"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kong-cli/internal/cmd"
	"github.com/kong/kong-cli/internal/cmd/root/products/konnect/common"
	"github.com/kong/kong-cli/internal/config"
	"github.com/kong/kong-cli/internal/konnect/auth"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	RequestPageSizeFlagName = "page-size"
	DefaultRequestPageSize  = 10
)

var (
	requestPageSizeConfigPath = fmt.Sprintf("konnect.gateway.%s.%s", CommandName, RequestPageSizeFlagName)

	getControlPlanesShort = i18n.T("root.products.konnect.gateway.controlplane.getControlPlanesShort",
		"List or get Konnect Kong Gateway control planes")
	getControlPlanesLong = i18n.T("root.products.konnect.gateway.controlplane.getControlPlanesLong",
		`Use the get verb with the control-plane command to query Konnect Kong Gateway control planes.`)
	getControlPlanesExample = normalizers.Examples(
		i18n.T("root.products.konnect.gateway.gateway.controlplane.getControlPlaneExamples",
			fmt.Sprintf(`
	# List all the control planes for the authorized user
	%[1]s get konnect gateway control-planes
	# Get details for a control plane with a specific ID 
	%[1]s get konnect gateway control-plane 22cd8a0b-72e7-4212-9099-0764f8e9c5ac
	# Get details for a control plane with a specific name
	%[1]s get konnect gateway control-plane my-control-plane 
	# Get all the control planes for the authorized user using command aliases
	%[1]s get k gw cps
	`, meta.CLIName)))
)

type getControlPlaneCmd struct {
	*cobra.Command
}

func (c *getControlPlaneCmd) runListByName(ctx context.Context, name string, kkClient *kk.SDK, helper cmd.Helper,
	cfg config.Hook, printer cli.Printer,
) error {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(requestPageSizeConfigPath))

	var allData []kkComps.ControlPlane

	for {
		req := kkOps.ListControlPlanesRequest{
			PageSize:     kk.Int64(requestPageSize),
			PageNumber:   kk.Int64(pageNumber),
			FilterNameEq: kk.String(name),
		}

		res, err := kkClient.ControlPlanes.ListControlPlanes(ctx, req)
		if err != nil {
			helper.GetCmd().SilenceUsage = true
			helper.GetCmd().SilenceErrors = true
			return &cmd.ExecutionError{
				Err: err,
			}
		}

		allData = append(allData, res.GetListControlPlanesResponse().Data...)
		totalItems := res.GetListControlPlanesResponse().Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	if len(allData) == 1 {
		printer.Print(allData[0])
	} else {
		printer.Print(allData)
	}
	return nil
}

func (c *getControlPlaneCmd) runList(ctx context.Context, kkClient *kk.SDK, helper cmd.Helper,
	cfg config.Hook, printer cli.Printer,
) error {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(requestPageSizeConfigPath))

	var allData []kkComps.ControlPlane

	for {
		req := kkOps.ListControlPlanesRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := kkClient.ControlPlanes.ListControlPlanes(ctx, req)
		if err != nil {
			helper.GetCmd().SilenceUsage = true
			helper.GetCmd().SilenceErrors = true
			return &cmd.ExecutionError{
				Err: err,
			}
		}

		allData = append(allData, res.GetListControlPlanesResponse().Data...)
		totalItems := res.GetListControlPlanesResponse().Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	printer.Print(allData)

	return nil
}

func (c *getControlPlaneCmd) runGet(ctx context.Context, id string, kkClient *kk.SDK, helper cmd.Helper,
	printer cli.Printer,
) error {
	res, err := kkClient.ControlPlanes.GetControlPlane(ctx, id)
	if err != nil {
		// TODO: This needs to be generalized in some way. When an execution error occurs,
		//		don't show usage or let cobra print the error. Use the printer with it's configured
		//		output format to print the error.  This is done at the root command
		helper.GetCmd().SilenceUsage = true
		helper.GetCmd().SilenceErrors = true
		return &cmd.ExecutionError{
			Err: err,
		}
	}

	printer.Print(res.GetControlPlane())

	return nil
}

func (c *getControlPlaneCmd) preRunE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	f := c.Flags().Lookup(RequestPageSizeFlagName)
	e = cfg.BindFlag(requestPageSizeConfigPath, f)
	return e
}

func (c *getControlPlaneCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing control planes requires 0 or 1 arguments (name or ID)"),
		}
	}

	config, err := helper.GetConfig()
	if err != nil {
		return err
	}

	pageSize := config.GetInt(requestPageSizeConfigPath)
	if pageSize < 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("%s must be greater than 0", RequestPageSizeFlagName),
		}
	}
	return nil
}

func (c *getControlPlaneCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if e := c.validate(helper); e != nil {
		return e
	}

	outType, e := helper.GetOutputFormat()
	if e != nil {
		return e
	}
	printer, e := cli.Format(outType, helper.GetStreams().Out)
	if e != nil {
		return e
	}

	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	defer printer.Flush()

	kkClient, err := auth.GetAuthenticatedClient(
		cfg.GetProfile(),
		cfg.GetString(common.PATConfigPath),
		cfg.GetString(common.MachineClientIDConfigPath),
		cfg.GetString(common.BaseURLConfigPath)+cfg.GetString(common.RefreshPathConfigPath))
	if err != nil {
		return err
	}

	ctx := context.Background()

	// 'get konnect gateway cps' can be run like various ways:
	//	> get konnect gateway cps <id>    # Get by UUID
	//  > get konnect gateway cps <name>	# Get by name
	//  > get konnect gateway cps					# List all
	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		id := helper.GetArgs()[0]

		// TODO: Is capturing the following error necessary?
		isUUID, _ := regexp.MatchString(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`, id)

		if !isUUID {
			// If the ID is not a UUID, then it is a name
			// search for the control plane by name
			return c.runListByName(ctx, id, kkClient, helper, cfg, printer)
		}

		return c.runGet(ctx, id, kkClient, helper, printer)
	}

	return c.runList(ctx, kkClient, helper, cfg, printer)
}

func newGetControlPlaneCmd(baseCmd *cobra.Command) *getControlPlaneCmd {
	baseCmd.Flags().Int(
		RequestPageSizeFlagName,
		DefaultRequestPageSize,
		fmt.Sprintf(
			"Max number of results to include per response page from Control Plane API requests.\n (config path = '%s')",
			requestPageSizeConfigPath))

	rv := getControlPlaneCmd{
		Command: baseCmd,
	}

	baseCmd.Short = getControlPlanesShort
	baseCmd.Long = getControlPlanesLong
	baseCmd.Example = getControlPlanesExample
	baseCmd.PreRunE = rv.preRunE
	baseCmd.RunE = rv.runE

	return &rv
}
