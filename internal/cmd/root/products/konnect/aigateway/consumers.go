package aigateway

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	declresources "github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type aiGatewayConsumerRecord struct {
	ID               string
	Name             string
	DisplayName      string
	Type             string
	PolicyCount      string
	LocalUpdatedTime string
}

var (
	aiGatewayConsumersUse   = "consumers [consumer-id|consumer-name]"
	aiGatewayConsumersShort = i18n.T(
		"root.products.konnect.ai-gateway.consumersShort",
		"List or get Consumers for a Konnect AI Gateway",
	)
	aiGatewayConsumersLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.consumersLong",
		`Use the consumers command to list or retrieve Consumers for a specific Konnect AI Gateway.`,
	))
	aiGatewayConsumersExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.consumersExamples",
			fmt.Sprintf(`# List Consumers for an AI Gateway by display name
%[1]s get ai-gateway consumers --gateway-name "Customer Support Gateway"
# List Consumers for an AI Gateway by ID
%[1]s get ai-gateway consumers --gateway-id <gateway-id>
# Get a Consumer by name
%[1]s get ai-gateway consumers --gateway-name "Customer Support Gateway" support-user
# Get a Consumer by ID
%[1]s get ai-gateway consumers --gateway-id <gateway-id> --consumer-id <consumer-id>
`, meta.CLIName)),
	)
)

func newGetAIGatewayConsumersCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayConsumersUse,
		Short:   aiGatewayConsumersShort,
		Long:    aiGatewayConsumersLong,
		Example: aiGatewayConsumersExample,
		Aliases: []string{"consumer"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayConsumerFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayConsumersHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayConsumerFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayConsumersHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayConsumersHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateway Consumers requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		consumerID, consumerName := getAIGatewayConsumerIdentifiers(cfg)
		if consumerID != "" || consumerName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayConsumerIDFlagName,
					aiGatewayConsumerNameFlagName,
				),
			}
		}
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	gatewayID, gatewayName := getAIGatewayIdentifiers(cfg)
	if gatewayID != "" && gatewayName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", aiGatewayIDFlagName, aiGatewayNameFlagName),
		}
	}
	if gatewayID == "" && gatewayName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"an AI Gateway identifier is required. Provide --%s or --%s",
				aiGatewayIDFlagName,
				aiGatewayNameFlagName,
			),
		}
	}
	if gatewayID == "" {
		gatewayID, err = resolveAIGatewayIDByDisplayName(gatewayName, sdk.GetAIGatewayAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	consumerAPI := sdk.GetAIGatewayConsumersAPI()
	if consumerAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Consumers client is not available",
			Err: fmt.Errorf("AI Gateway Consumers client not configured"),
		}
	}

	consumerID, consumerName := getAIGatewayConsumerIdentifiers(cfg)
	if consumerID != "" && consumerName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayConsumerIDFlagName,
				aiGatewayConsumerNameFlagName,
			),
		}
	}

	identifier := ""
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else if consumerID != "" {
		identifier = consumerID
	} else if consumerName != "" {
		identifier = consumerName
	}

	if identifier != "" {
		return h.getSingleConsumer(helper, consumerAPI, gatewayID, identifier, outType, printer)
	}
	return h.listConsumers(helper, consumerAPI, gatewayID, outType, printer, cfg)
}

func (h aiGatewayConsumersHandler) listConsumers(
	helper cmd.Helper,
	consumerAPI helpers.AIGatewayConsumersAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	consumers, err := fetchAIGatewayConsumers(helper, consumerAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayConsumerRecord, 0, len(consumers))
	tableRows := make([]table.Row, 0, len(consumers))
	for _, consumer := range consumers {
		record := aiGatewayConsumerToRecord(consumer)
		records = append(records, record)
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.Type,
			record.PolicyCount,
			record.LocalUpdatedTime,
		})
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		consumers,
		"",
		tableview.WithCustomTable(
			[]string{
				aiGatewayHeaderID,
				aiGatewayHeaderName,
				aiGatewayHeaderDisplayName,
				aiGatewayHeaderType,
				aiGatewayHeaderPolicies,
				aiGatewayHeaderUpdated,
			},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index < 0 || index >= len(consumers) {
				return ""
			}
			return aiGatewayConsumerDetailView(consumers[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayConsumer, func(index int) any {
			if index < 0 || index >= len(consumers) {
				return nil
			}
			return &consumers[index]
		}),
	)
}

func (h aiGatewayConsumersHandler) getSingleConsumer(
	helper cmd.Helper,
	consumerAPI helpers.AIGatewayConsumersAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	res, err := consumerAPI.GetAiGatewayConsumer(helper.GetContext(), gatewayID, identifier)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway Consumer", err, helper.GetCmd(), attrs...)
	}
	if res == nil || res.AIGatewayConsumer == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Consumer response was empty",
			Err: fmt.Errorf("no Consumer returned for id or name %s", identifier),
		}
	}
	consumer := res.AIGatewayConsumer

	record := aiGatewayConsumerToRecord(*consumer)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		consumer,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index != 0 {
				return ""
			}
			return aiGatewayConsumerDetailView(*consumer)
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayConsumer, func(index int) any {
			if index != 0 {
				return nil
			}
			return consumer
		}),
	)
}

func fetchAIGatewayConsumers(
	helper cmd.Helper,
	consumerAPI helpers.AIGatewayConsumersAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayConsumer, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayConsumer

	for {
		req := kkOps.ListAiGatewayConsumersRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := consumerAPI.ListAiGatewayConsumers(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list AI Gateway Consumers", err, helper.GetCmd(), attrs...)
		}
		if res == nil || res.ListAIGatewayConsumersResponse == nil {
			break
		}

		allData = append(allData, res.ListAIGatewayConsumersResponse.Data...)
		nextCursor := pagination.ExtractPageAfterCursor(res.ListAIGatewayConsumersResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func aiGatewayConsumerToRecord(consumer kkComps.AIGatewayConsumer) aiGatewayConsumerRecord {
	record := aiGatewayConsumerRecord{
		ID:               aiGatewayMissingValue,
		Name:             valueOrMissing(declresources.AIGatewayConsumerName(consumer)),
		DisplayName:      valueOrMissing(declresources.AIGatewayConsumerDisplayName(consumer)),
		Type:             valueOrMissing(string(consumer.Type)),
		PolicyCount:      fmt.Sprintf("%d", len(consumer.Policies)),
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if id := declresources.AIGatewayConsumerID(consumer); id != "" {
		record.ID = util.AbbreviateUUID(id)
	}
	if updatedAt := declresources.AIGatewayConsumerUpdatedAt(consumer); !updatedAt.IsZero() {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func aiGatewayConsumerDetailView(consumer kkComps.AIGatewayConsumer) string {
	payload := make(map[string]any)
	data, err := json.Marshal(consumer)
	if err == nil {
		_ = json.Unmarshal(data, &payload)
	}

	order := []string{
		"id",
		"name",
		"type",
		"display_name",
		"custom_id",
		"policies",
		"labels",
		"managed_by",
		aiGatewayFieldCreatedAt,
		aiGatewayFieldUpdatedAt,
	}

	var b strings.Builder
	for _, field := range order {
		fmt.Fprintf(&b, "%s: %s\n", field, formatAIGatewayModelDetailValue(payload[field]))
	}
	return strings.TrimRight(b.String(), "\n")
}

func buildAIGatewayConsumerChildView(consumers []kkComps.AIGatewayConsumer) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(consumers))
	for i := range consumers {
		record := aiGatewayConsumerToRecord(consumers[i])
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.Type,
			record.PolicyCount,
		})
	}

	return tableview.ChildView{
		Headers: []string{
			aiGatewayHeaderID,
			aiGatewayHeaderName,
			aiGatewayHeaderDisplayName,
			aiGatewayHeaderType,
			aiGatewayHeaderPolicies,
		},
		Rows: tableRows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(consumers) {
				return ""
			}
			return aiGatewayConsumerDetailView(consumers[index])
		},
		Title:      "AI Gateway Consumers",
		ParentType: common.ViewParentAIGatewayConsumer,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(consumers) {
				return nil
			}
			return &consumers[index]
		},
	}
}
