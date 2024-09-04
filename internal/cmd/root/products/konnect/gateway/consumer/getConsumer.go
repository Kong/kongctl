package consumer

import (
	"fmt"
	"regexp"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	"github.com/kong/kongctl/internal/cmd"
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

type getConsumerCmd struct {
	*cobra.Command
}

var (
	getConsumerShort = i18n.T("root.products.konnect.gateway.consumer.getConsumerShort",
		"List or get Konnect Kong Gateway Consumers")
	getConsumerLong = i18n.T("root.products.konnect.gateway.service.getServiceLong",
		`Use the get verb with the consumer command to query Konnect Kong Gateway Consumers.`)
	getConsumerExamples = normalizers.Examples(
		i18n.T("root.products.konnect.gateway.consumer.getConsumerExamples",
			fmt.Sprintf(`
	# List all the Kong Gateway Consumers for the a given Control Plane (by ID)
	%[1]s get konnect gateway consumers --control-plane-id <id>
	# List all the Kong Gateway Consumers for the a given Control Plane (by name)
	%[1]s get konnect gateway consumers --control-plane-name <name>
	# Get a specific Kong Gateway Consumers located on the given Control Plane (by name)
	%[1]s get konnect gateway consumer --control-plane-name <name> <consumer-name>
	`, meta.CLIName)))
)

func (c *getConsumerCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing gateway consumers requires 0 or 1 arguments (name or ID)"),
		}
	}
	return nil
}

func (c *getConsumerCmd) runListByUsername(cpID string, username string,
	kkClient *kk.SDK, helper cmd.Helper, cfg config.Hook, printer cli.Printer,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))

	allData, err := helpers.GetAllGatewayConsumers(helper.GetContext(), requestPageSize, cpID, kkClient)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list Gateway Consumers", err, helper.GetCmd(), attrs...)
	}

	for _, consumer := range allData {
		if *consumer.GetUsername() == username {
			printer.Print(consumer)
		}
	}

	return nil
}

func (c *getConsumerCmd) runGet(cpID string, id string,
	kkClient *kk.SDK, helper cmd.Helper, printer cli.Printer,
) error {
	res, err := kkClient.Consumers.GetConsumer(helper.GetContext(), cpID, id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get Gateway Consumer", err, helper.GetCmd(), attrs...)
	}

	printer.Print(res.GetConsumer())

	return nil
}

func (c *getConsumerCmd) runList(cpID string,
	kkClient *kk.SDK, helper cmd.Helper, cfg config.Hook, printer cli.Printer,
) error {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))

	allData, err := helpers.GetAllGatewayConsumers(helper.GetContext(), requestPageSize, cpID, kkClient)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list Gateway Consumers", err, helper.GetCmd(), attrs...)
	}

	printer.Print(allData)

	return nil
}

func (c *getConsumerCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if e := c.validate(helper); e != nil {
		return e
	}

	logger, e := helper.GetLogger()
	if e != nil {
		return e
	}

	cfg, e := helper.GetConfig()
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

	// 'get konnect gateway consumers ' can be run like various ways:
	//	> get konnect gateway consumers <id>				# Get by UUID
	//  > get konnect gateway consumers <username>	# Get by uname
	//  > get konnect gateway consumers							# List all
	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		id := helper.GetArgs()[0]

		isUUID, _ := regexp.MatchString(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`, id)
		// TODO: Is capturing the previous blanked error advised?

		if !isUUID {
			// If the ID is not a UUID, then it is a name
			// search for the control plane by name
			return c.runListByUsername(cpID, id, kkClient.(*helpers.KonnectSDK).SDK, helper, cfg, printer)
		}

		return c.runGet(cpID, id, kkClient.(*helpers.KonnectSDK).SDK, helper, printer)
	}

	return c.runList(cpID, kkClient.(*helpers.KonnectSDK).SDK, helper, cfg, printer)
}

func newGetConsumerCmd(baseCmd *cobra.Command) *getConsumerCmd {
	rv := getConsumerCmd{
		Command: baseCmd,
	}

	baseCmd.Short = getConsumerShort
	baseCmd.Long = getConsumerLong
	baseCmd.Example = getConsumerExamples
	baseCmd.RunE = rv.runE

	return &rv
}
