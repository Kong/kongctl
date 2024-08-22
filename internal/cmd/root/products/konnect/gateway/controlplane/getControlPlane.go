package controlplane

import (
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

var (
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

func (c *getControlPlaneCmd) runListByName(name string, kkClient *kk.SDK, helper cmd.Helper,
	cfg config.Hook, printer cli.Printer,
) error {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.ControlPlane

	for {
		req := kkOps.ListControlPlanesRequest{
			PageSize:     kk.Int64(requestPageSize),
			PageNumber:   kk.Int64(pageNumber),
			FilterNameEq: kk.String(name),
		}

		res, err := kkClient.ControlPlanes.ListControlPlanes(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return cmd.PrepareExecutionError("Failed to list Control Planes", err, helper.GetCmd(), attrs...)
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

func (c *getControlPlaneCmd) runList(kkClient *kk.SDK, helper cmd.Helper,
	cfg config.Hook, printer cli.Printer,
) error {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.ControlPlane

	for {
		req := kkOps.ListControlPlanesRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := kkClient.ControlPlanes.ListControlPlanes(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return cmd.PrepareExecutionError("Failed to list Control Planes", err, helper.GetCmd(), attrs...)
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

func (c *getControlPlaneCmd) runGet(id string, kkClient *kk.SDK, helper cmd.Helper,
	printer cli.Printer,
) error {
	res, err := kkClient.ControlPlanes.GetControlPlane(helper.GetContext(), id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get Control Plane", err, helper.GetCmd(), attrs...)
	}

	printer.Print(res.GetControlPlane())

	return nil
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

	pageSize := config.GetInt(common.RequestPageSizeConfigPath)
	if pageSize < 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("%s must be greater than 0", common.RequestPageSizeFlagName),
		}
	}
	return nil
}

func (c *getControlPlaneCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if e := c.validate(helper); e != nil {
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
	printer, e := cli.Format(outType, helper.GetStreams().Out)
	if e != nil {
		return e
	}

	defer printer.Flush()

	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	token, e := common.GetAccessToken(cfg, logger)
	if e != nil {
		return fmt.Errorf(
			`no access token available. Use "%s login konnect" to authenticate or provide a Konnect PAT using the --pat flag`,
			meta.CLIName)
	}

	kkClient, err := auth.GetAuthenticatedClient(token)
	if err != nil {
		return err
	}

	// 'get konnect gateway cps' can be run like various ways:
	//	> get konnect gateway cps <id>    # Get by UUID
	//  > get konnect gateway cps <name>	# Get by name
	//  > get konnect gateway cps					# List all
	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		id := helper.GetArgs()[0]

		isUUID, _ := regexp.MatchString(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`, id)
		// TODO: Is capturing the blanked error necessary?

		if !isUUID {
			// If the ID is not a UUID, then it is a name
			// search for the control plane by name
			return c.runListByName(id, kkClient, helper, cfg, printer)
		}

		return c.runGet(id, kkClient, helper, printer)
	}

	return c.runList(kkClient, helper, cfg, printer)
}

func newGetControlPlaneCmd(baseCmd *cobra.Command) *getControlPlaneCmd {
	rv := getControlPlaneCmd{
		Command: baseCmd,
	}

	baseCmd.Short = getControlPlanesShort
	baseCmd.Long = getControlPlanesLong
	baseCmd.Example = getControlPlanesExample
	baseCmd.RunE = rv.runE

	return &rv
}
