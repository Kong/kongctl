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

type aiGatewayAgentRecord struct {
	ID               string
	Name             string
	DisplayName      string
	Type             string
	Enabled          string
	PolicyCount      string
	LocalUpdatedTime string
}

var (
	aiGatewayAgentsUse   = "agents [agent-id|agent-name]"
	aiGatewayAgentsShort = i18n.T(
		"root.products.konnect.ai-gateway.agentsShort",
		"List or get Agents for a Konnect AI Gateway",
	)
	aiGatewayAgentsLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.agentsLong",
		`Use the agents command to list or retrieve Agents for a specific Konnect AI Gateway.`,
	))
	aiGatewayAgentsExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.agentsExamples",
			fmt.Sprintf(`# List Agents for an AI Gateway by display name
%[1]s get ai-gateway agents --gateway-name "Customer Support Gateway"
# List Agents for an AI Gateway by ID
%[1]s get ai-gateway agents --gateway-id <gateway-id>
# Get an Agent by name
%[1]s get ai-gateway agents --gateway-name "Customer Support Gateway" booking-agent
# Get an Agent by ID
%[1]s get ai-gateway agents --gateway-id <gateway-id> --agent-id <agent-id>
`, meta.CLIName)),
	)
)

func newGetAIGatewayAgentsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayAgentsUse,
		Short:   aiGatewayAgentsShort,
		Long:    aiGatewayAgentsLong,
		Example: aiGatewayAgentsExample,
		Aliases: []string{"agent"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayAgentFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayAgentsHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayAgentFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayAgentsHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayAgentsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateway Agents requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		agentID, agentName := getAIGatewayAgentIdentifiers(cfg)
		if agentID != "" || agentName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayAgentIDFlagName,
					aiGatewayAgentNameFlagName,
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
		gatewayID, err = resolveAIGatewayIDByName(gatewayName, sdk.GetAIGatewayAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	agentAPI := sdk.GetAIGatewayAgentsAPI()
	if agentAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Agents client is not available",
			Err: fmt.Errorf("AI Gateway Agents client not configured"),
		}
	}

	agentID, agentName := getAIGatewayAgentIdentifiers(cfg)
	if agentID != "" && agentName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayAgentIDFlagName,
				aiGatewayAgentNameFlagName,
			),
		}
	}

	identifier := ""
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else if agentID != "" {
		identifier = agentID
	} else if agentName != "" {
		identifier = agentName
	}

	if identifier != "" {
		return h.getSingleAgent(helper, agentAPI, gatewayID, identifier, outType, printer)
	}
	return h.listAgents(helper, agentAPI, gatewayID, outType, printer, cfg)
}

func (h aiGatewayAgentsHandler) listAgents(
	helper cmd.Helper,
	agentAPI helpers.AIGatewayAgentsAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	agents, err := fetchAIGatewayAgents(helper, agentAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayAgentRecord, 0, len(agents))
	tableRows := make([]table.Row, 0, len(agents))
	for _, agent := range agents {
		record := aiGatewayAgentToRecord(agent)
		records = append(records, record)
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.Type,
			record.Enabled,
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
		agents,
		"",
		tableview.WithCustomTable(
			[]string{
				aiGatewayHeaderID,
				aiGatewayHeaderName,
				aiGatewayHeaderDisplayName,
				aiGatewayHeaderType,
				aiGatewayHeaderEnabled,
				aiGatewayHeaderPolicies,
				aiGatewayHeaderUpdated,
			},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index < 0 || index >= len(agents) {
				return ""
			}
			return aiGatewayAgentDetailView(agents[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayAgent, func(index int) any {
			if index < 0 || index >= len(agents) {
				return nil
			}
			return &agents[index]
		}),
	)
}

func (h aiGatewayAgentsHandler) getSingleAgent(
	helper cmd.Helper,
	agentAPI helpers.AIGatewayAgentsAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	res, err := agentAPI.GetAiGatewayAgent(helper.GetContext(), gatewayID, identifier)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway Agent", err, helper.GetCmd(), attrs...)
	}
	if res == nil || res.AIGatewayAgent == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Agent response was empty",
			Err: fmt.Errorf("no Agent returned for id or name %s", identifier),
		}
	}
	agent := res.AIGatewayAgent

	record := aiGatewayAgentToRecord(*agent)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		agent,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index != 0 {
				return ""
			}
			return aiGatewayAgentDetailView(*agent)
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayAgent, func(index int) any {
			if index != 0 {
				return nil
			}
			return agent
		}),
	)
}

func fetchAIGatewayAgents(
	helper cmd.Helper,
	agentAPI helpers.AIGatewayAgentsAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayAgent, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayAgent

	for {
		req := kkOps.ListAiGatewayAgentsRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := agentAPI.ListAiGatewayAgents(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list AI Gateway Agents", err, helper.GetCmd(), attrs...)
		}
		if res == nil || res.ListAIGatewayAgentsResponse == nil {
			break
		}

		allData = append(allData, res.ListAIGatewayAgentsResponse.Data...)
		nextCursor := pagination.ExtractPageAfterCursor(res.ListAIGatewayAgentsResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func aiGatewayAgentToRecord(agent kkComps.AIGatewayAgent) aiGatewayAgentRecord {
	record := aiGatewayAgentRecord{
		ID:               aiGatewayMissingValue,
		Name:             valueOrMissing(declresources.AIGatewayAgentName(agent)),
		DisplayName:      valueOrMissing(declresources.AIGatewayAgentDisplayName(agent)),
		Type:             valueOrMissing(string(agent.Type)),
		Enabled:          aiGatewayMissingValue,
		PolicyCount:      fmt.Sprintf("%d", len(agent.Policies)),
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if enabled := declresources.AIGatewayAgentEnabled(agent); enabled != nil {
		record.Enabled = fmt.Sprintf("%t", *enabled)
	}
	if id := declresources.AIGatewayAgentID(agent); id != "" {
		record.ID = util.AbbreviateUUID(id)
	}
	if updatedAt := declresources.AIGatewayAgentUpdatedAt(agent); !updatedAt.IsZero() {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func aiGatewayAgentDetailView(agent kkComps.AIGatewayAgent) string {
	payload := make(map[string]any)
	data, err := json.Marshal(agent)
	if err == nil {
		// Detail views are best-effort; leave missing fields as n/a if SDK union data cannot round-trip.
		_ = json.Unmarshal(data, &payload)
	}

	order := []string{
		"id",
		"name",
		"display_name",
		"enabled",
		"type",
		"policies",
		"acls",
		"config",
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

func buildAIGatewayAgentChildView(agents []kkComps.AIGatewayAgent) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(agents))
	for i := range agents {
		record := aiGatewayAgentToRecord(agents[i])
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.Type,
			record.Enabled,
			record.PolicyCount,
		})
	}

	return tableview.ChildView{
		Headers: []string{
			aiGatewayHeaderID,
			aiGatewayHeaderName,
			aiGatewayHeaderDisplayName,
			aiGatewayHeaderType,
			aiGatewayHeaderEnabled,
			aiGatewayHeaderPolicies,
		},
		Rows: tableRows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(agents) {
				return ""
			}
			return aiGatewayAgentDetailView(agents[index])
		},
		Title:      "AI Gateway Agents",
		ParentType: common.ViewParentAIGatewayAgent,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(agents) {
				return nil
			}
			return &agents[index]
		},
	}
}
