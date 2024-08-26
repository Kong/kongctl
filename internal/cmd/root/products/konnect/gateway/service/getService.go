package service

import (
	"fmt"
	"regexp"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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

type getServiceCmd struct {
	*cobra.Command
}

type textDisplayRecord struct {
	Name string
	ID   string
}

func serviceToDisplayRecord(s *kkComps.Service) textDisplayRecord {
	return textDisplayRecord{
		Name: *s.Name,
		ID:   *s.ID,
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

func (c *getServiceCmd) runListByName(cpID string, name string,
	kkClient *kk.SDK, helper cmd.Helper, cfg config.Hook, printer cli.Printer,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))

	allData, err := helpers.GetAllGatewayServices(helper.GetContext(), requestPageSize, cpID, kkClient)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list Gateway Services", err, helper.GetCmd(), attrs...)
	}

	for _, service := range allData {
		if *service.GetName() == name {
			printer.Print(service)
		}
	}

	return nil
}

func (c *getServiceCmd) runGet(cpID string, id string,
	kkClient *kk.SDK, helper cmd.Helper, printer cli.Printer,
) error {
	res, err := kkClient.Services.GetService(helper.GetContext(), cpID, id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get Gateway Service", err, helper.GetCmd(), attrs...)
	}

	printer.Print(res.GetService())

	return nil
}

func (c *getServiceCmd) runList(cpID string,
	kkClient *kk.SDK, helper cmd.Helper, cfg config.Hook, printer cli.Printer,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))

	allData, err := helpers.GetAllGatewayServices(helper.GetContext(), requestPageSize, cpID, kkClient)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list Gateway Services", err, helper.GetCmd(), attrs...)
	}

	printer.Print(allData)

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
			return c.runListByName(cpID, id, kkClient, helper, cfg, printer)
		}

		return c.runGet(cpID, id, kkClient, helper, printer)
	}

	return c.runList(cpID, kkClient, helper, cfg, printer)
}

func newGetServiceCmd(baseCmd *cobra.Command) *getServiceCmd {
	rv := getServiceCmd{
		Command: baseCmd,
	}

	baseCmd.Short = getServiceShort
	baseCmd.Long = getServiceLong
	baseCmd.Example = getServiceExamples
	baseCmd.RunE = rv.runE

	return &rv
}
