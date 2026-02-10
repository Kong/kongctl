package consumer

import (
	"fmt"
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

type getConsumerCmd struct {
	*cobra.Command
}

type consumerDisplayRecord struct {
	ID               string
	Username         string
	CustomID         string
	TagCount         int
	LocalCreatedTime string
	LocalUpdatedTime string
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

	// 'get konnect gateway consumers ' can be run like various ways:
	//	> get konnect gateway consumers <id>				# Get by UUID
	//  > get konnect gateway consumers <username>	# Get by uname
	//  > get konnect gateway consumers							# List all
	internalSDK := kkClient.(*helpers.KonnectSDK).SDK

	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		identifier := strings.TrimSpace(helper.GetArgs()[0])

		var consumer *kkComps.Consumer
		if util.IsValidUUID(identifier) {
			consumer, e = fetchConsumerByID(helper, internalSDK, cpID, identifier)
		} else {
			consumer, e = findConsumerByUsername(helper, cfg, internalSDK, cpID, identifier)
		}
		if e != nil {
			return e
		}

		return renderConsumers(helper, outType, printer, []kkComps.Consumer{*consumer})
	}

	consumers, err := fetchAllConsumers(helper, cfg, internalSDK, cpID)
	if err != nil {
		return err
	}

	return renderConsumers(helper, outType, printer, consumers)
}

func newGetConsumerCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getConsumerCmd {
	rv := getConsumerCmd{
		Command: baseCmd,
	}

	baseCmd.Short = getConsumerShort
	baseCmd.Long = getConsumerLong
	baseCmd.Example = getConsumerExamples

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

func fetchAllConsumers(helper cmd.Helper, cfg config.Hook, sdk *kk.SDK, cpID string) ([]kkComps.Consumer, error) {
	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))
	return helpers.GetAllGatewayConsumers(helper.GetContext(), requestPageSize, cpID, sdk)
}

func findConsumerByUsername(
	helper cmd.Helper,
	cfg config.Hook,
	sdk *kk.SDK,
	cpID string,
	username string,
) (*kkComps.Consumer, error) {
	consumers, err := fetchAllConsumers(helper, cfg, sdk, cpID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to list Gateway Consumers", err, helper.GetCmd(), attrs...)
	}

	lowered := strings.ToLower(username)
	for i := range consumers {
		if u := consumers[i].GetUsername(); u != nil && strings.ToLower(*u) == lowered {
			return &consumers[i], nil
		}
	}

	return nil, &cmd.ConfigurationError{
		Err: fmt.Errorf("gateway consumer %q not found", username),
	}
}

func fetchConsumerByID(helper cmd.Helper, sdk *kk.SDK, cpID, id string) (*kkComps.Consumer, error) {
	res, err := sdk.Consumers.GetConsumer(helper.GetContext(), cpID, id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get Gateway Consumer", err, helper.GetCmd(), attrs...)
	}

	consumer := res.GetConsumer()
	if consumer == nil {
		return nil, &cmd.ExecutionError{
			Msg: "Gateway consumer response was empty",
			Err: fmt.Errorf("no consumer returned for id %s", id),
		}
	}

	return consumer, nil
}

func renderConsumers(
	helper cmd.Helper, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	consumers []kkComps.Consumer,
) error {
	records := make([]consumerDisplayRecord, 0, len(consumers))
	rows := make([]table.Row, 0, len(consumers))
	for i := range consumers {
		record := consumerToDisplayRecord(&consumers[i])
		records = append(records, record)
		rows = append(rows, table.Row{util.AbbreviateUUID(record.ID), record.Username})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(consumers) {
			return ""
		}
		return ConsumerDetailView(&consumers[index])
	}

	var raw any
	if len(consumers) == 1 {
		raw = consumers[0]
	} else {
		raw = consumers
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		raw,
		"",
		tableview.WithCustomTable([]string{"ID", "USERNAME"}, rows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func consumerToDisplayRecord(consumer *kkComps.Consumer) consumerDisplayRecord {
	const missing = "n/a"

	id := missing
	if consumer.GetID() != nil && *consumer.GetID() != "" {
		id = *consumer.GetID()
	}

	username := missing
	if consumer.GetUsername() != nil && *consumer.GetUsername() != "" {
		username = *consumer.GetUsername()
	}

	customID := missing
	if consumer.GetCustomID() != nil && *consumer.GetCustomID() != "" {
		customID = *consumer.GetCustomID()
	}

	created := missing
	if ts := consumer.GetCreatedAt(); ts != nil {
		created = time.Unix(0, *ts*int64(time.Millisecond)).In(time.Local).Format("2006-01-02 15:04:05")
	}

	updated := missing
	if ts := consumer.GetUpdatedAt(); ts != nil {
		updated = time.Unix(0, *ts*int64(time.Millisecond)).In(time.Local).Format("2006-01-02 15:04:05")
	}

	tags := consumer.GetTags()

	return consumerDisplayRecord{
		ID:               util.AbbreviateUUID(id),
		Username:         username,
		CustomID:         customID,
		TagCount:         len(tags),
		LocalCreatedTime: created,
		LocalUpdatedTime: updated,
	}
}

func ConsumerDetailView(consumer *kkComps.Consumer) string {
	if consumer == nil {
		return ""
	}

	const missing = "n/a"

	id := missing
	if consumer.GetID() != nil && *consumer.GetID() != "" {
		id = *consumer.GetID()
	}

	username := missing
	if consumer.GetUsername() != nil && *consumer.GetUsername() != "" {
		username = *consumer.GetUsername()
	}

	customID := missing
	if consumer.GetCustomID() != nil && *consumer.GetCustomID() != "" {
		customID = *consumer.GetCustomID()
	}

	created := missing
	if ts := consumer.GetCreatedAt(); ts != nil {
		created = time.Unix(0, *ts*int64(time.Millisecond)).In(time.Local).Format("2006-01-02 15:04:05")
	}

	updated := missing
	if ts := consumer.GetUpdatedAt(); ts != nil {
		updated = time.Unix(0, *ts*int64(time.Millisecond)).In(time.Local).Format("2006-01-02 15:04:05")
	}

	tags := consumer.GetTags()
	tagsLine := missing
	if len(tags) > 0 {
		tagsLine = strings.Join(tags, ", ")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "username: %s\n", username)
	fmt.Fprintf(&b, "custom_id: %s\n", customID)
	fmt.Fprintf(&b, "tags: %s\n", tagsLine)
	fmt.Fprintf(&b, "created_at: %s\n", created)
	fmt.Fprintf(&b, "updated_at: %s\n", updated)

	return b.String()
}
