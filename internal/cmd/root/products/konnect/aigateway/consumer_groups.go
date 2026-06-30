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

type aiGatewayConsumerGroupRecord struct {
	ID               string
	Name             string
	DisplayName      string
	PolicyCount      string
	LocalUpdatedTime string
}

var (
	aiGatewayConsumerGroupsUse   = "consumer-groups [consumer-group-id|consumer-group-name]"
	aiGatewayConsumerGroupsShort = i18n.T(
		"root.products.konnect.ai-gateway.consumer-groupsShort",
		"List or get Consumer Groups for a Konnect AI Gateway",
	)
	aiGatewayConsumerGroupsLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.consumer-groupsLong",
		`Use the consumer-groups command to list or retrieve Consumer Groups for a specific Konnect AI Gateway.`,
	))
	aiGatewayConsumerGroupsExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.consumer-groupsExamples",
			fmt.Sprintf(`# List Consumer Groups for an AI Gateway by display name
%[1]s get ai-gateway consumer-groups --gateway-name "Customer Support Gateway"
# List Consumer Groups for an AI Gateway by ID
%[1]s get ai-gateway consumer-groups --gateway-id <gateway-id>
# Get a Consumer Group by name
%[1]s get ai-gateway consumer-groups --gateway-name "Customer Support Gateway" premium-users
# Get a Consumer Group by ID
%[1]s get ai-gateway consumer-groups --gateway-id <gateway-id> --consumer-group-id <consumer-group-id>
`, meta.CLIName)),
	)
)

func newGetAIGatewayConsumerGroupsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayConsumerGroupsUse,
		Short:   aiGatewayConsumerGroupsShort,
		Long:    aiGatewayConsumerGroupsLong,
		Example: aiGatewayConsumerGroupsExample,
		Aliases: []string{"consumer-group"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayConsumerGroupFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayConsumerGroupsHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayConsumerGroupFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayConsumerGroupsHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayConsumerGroupsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateway Consumer Groups requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		groupID, groupName := getAIGatewayConsumerGroupIdentifiers(cfg)
		if groupID != "" || groupName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayConsumerGroupIDFlagName,
					aiGatewayConsumerGroupNameFlagName,
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

	groupAPI := sdk.GetAIGatewayConsumerGroupsAPI()
	if groupAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Consumer Groups client is not available",
			Err: fmt.Errorf("AI Gateway Consumer Groups client not configured"),
		}
	}

	groupID, groupName := getAIGatewayConsumerGroupIdentifiers(cfg)
	if groupID != "" && groupName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayConsumerGroupIDFlagName,
				aiGatewayConsumerGroupNameFlagName,
			),
		}
	}

	identifier := ""
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else if groupID != "" {
		identifier = groupID
	} else if groupName != "" {
		identifier = groupName
	}

	if identifier != "" {
		return h.getSingleConsumerGroup(helper, groupAPI, gatewayID, identifier, outType, printer)
	}
	return h.listConsumerGroups(helper, groupAPI, gatewayID, outType, printer, cfg)
}

func (h aiGatewayConsumerGroupsHandler) listConsumerGroups(
	helper cmd.Helper,
	groupAPI helpers.AIGatewayConsumerGroupsAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	groups, err := fetchAIGatewayConsumerGroups(helper, groupAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayConsumerGroupRecord, 0, len(groups))
	tableRows := make([]table.Row, 0, len(groups))
	for _, group := range groups {
		record := aiGatewayConsumerGroupToRecord(group)
		records = append(records, record)
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
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
		groups,
		"",
		tableview.WithCustomTable(
			[]string{
				aiGatewayHeaderID,
				aiGatewayHeaderName,
				aiGatewayHeaderDisplayName,
				aiGatewayHeaderPolicies,
				aiGatewayHeaderUpdated,
			},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index < 0 || index >= len(groups) {
				return ""
			}
			return aiGatewayConsumerGroupDetailView(groups[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayConsumerGroup, func(index int) any {
			if index < 0 || index >= len(groups) {
				return nil
			}
			return &groups[index]
		}),
	)
}

func (h aiGatewayConsumerGroupsHandler) getSingleConsumerGroup(
	helper cmd.Helper,
	groupAPI helpers.AIGatewayConsumerGroupsAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	res, err := groupAPI.GetAiGatewayConsumerGroup(helper.GetContext(), gatewayID, identifier)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway Consumer Group", err, helper.GetCmd(), attrs...)
	}
	if res == nil || res.AIGatewayConsumerGroup == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Consumer Group response was empty",
			Err: fmt.Errorf("no Consumer Group returned for id or name %s", identifier),
		}
	}
	group := res.AIGatewayConsumerGroup

	record := aiGatewayConsumerGroupToRecord(*group)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		group,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index != 0 {
				return ""
			}
			return aiGatewayConsumerGroupDetailView(*group)
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayConsumerGroup, func(index int) any {
			if index != 0 {
				return nil
			}
			return group
		}),
	)
}

func fetchAIGatewayConsumerGroups(
	helper cmd.Helper,
	groupAPI helpers.AIGatewayConsumerGroupsAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayConsumerGroup, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayConsumerGroup

	for {
		req := kkOps.ListAiGatewayConsumerGroupsRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := groupAPI.ListAiGatewayConsumerGroups(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError(
				"Failed to list AI Gateway Consumer Groups",
				err,
				helper.GetCmd(),
				attrs...,
			)
		}
		if res == nil || res.ListAIGatewayConsumerGroupsResponse == nil {
			break
		}

		allData = append(allData, res.ListAIGatewayConsumerGroupsResponse.Data...)
		nextCursor := pagination.ExtractPageAfterCursor(res.ListAIGatewayConsumerGroupsResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func aiGatewayConsumerGroupToRecord(group kkComps.AIGatewayConsumerGroup) aiGatewayConsumerGroupRecord {
	record := aiGatewayConsumerGroupRecord{
		ID:               aiGatewayMissingValue,
		Name:             valueOrMissing(declresources.AIGatewayConsumerGroupName(group)),
		DisplayName:      valueOrMissing(declresources.AIGatewayConsumerGroupDisplayName(group)),
		PolicyCount:      fmt.Sprintf("%d", len(group.Policies)),
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if id := declresources.AIGatewayConsumerGroupID(group); id != "" {
		record.ID = util.AbbreviateUUID(id)
	}
	if updatedAt := declresources.AIGatewayConsumerGroupUpdatedAt(group); !updatedAt.IsZero() {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func aiGatewayConsumerGroupDetailView(group kkComps.AIGatewayConsumerGroup) string {
	payload := make(map[string]any)
	data, err := json.Marshal(group)
	if err == nil {
		// Detail views are best-effort; leave missing fields as n/a if SDK union data cannot round-trip.
		_ = json.Unmarshal(data, &payload)
	}

	order := []string{
		"id",
		"name",
		"display_name",
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

func buildAIGatewayConsumerGroupChildView(groups []kkComps.AIGatewayConsumerGroup) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(groups))
	for i := range groups {
		record := aiGatewayConsumerGroupToRecord(groups[i])
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.PolicyCount,
		})
	}

	return tableview.ChildView{
		Headers: []string{
			aiGatewayHeaderID,
			aiGatewayHeaderName,
			aiGatewayHeaderDisplayName,
			aiGatewayHeaderPolicies,
		},
		Rows: tableRows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(groups) {
				return ""
			}
			return aiGatewayConsumerGroupDetailView(groups[index])
		},
		Title:      "AI Gateway Consumer Groups",
		ParentType: common.ViewParentAIGatewayConsumerGroup,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(groups) {
				return nil
			}
			return &groups[index]
		},
	}
}
