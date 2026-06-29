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
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type aiGatewayNodeRecord struct {
	ID               string
	Hostname         string
	Type             string
	Version          string
	ConfigVersion    string
	State            string
	LocalUpdatedTime string
}

var (
	aiGatewayNodesUse   = "nodes [node-id]"
	aiGatewayNodesShort = i18n.T(
		"root.products.konnect.ai-gateway.nodesShort",
		"List or get data plane Nodes for a Konnect AI Gateway",
	)
	aiGatewayNodesLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.nodesLong",
		`Use the nodes command to list or retrieve data plane Nodes for a specific Konnect AI Gateway.`,
	))
	aiGatewayNodesExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.nodesExamples",
			fmt.Sprintf(`# List Nodes for an AI Gateway by display name
%[1]s get ai-gateway nodes --gateway-name "Customer Support Gateway"
# List Nodes for an AI Gateway by ID
%[1]s get ai-gateway nodes --gateway-id <gateway-id>
# Get a Node by ID
%[1]s get ai-gateway nodes --gateway-name "Customer Support Gateway" <node-id>
# Get a Node by ID flag
%[1]s get ai-gateway nodes --gateway-id <gateway-id> --node-id <node-id>
`, meta.CLIName)),
	)
)

func newGetAIGatewayNodesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayNodesUse,
		Short:   aiGatewayNodesShort,
		Long:    aiGatewayNodesLong,
		Example: aiGatewayNodesExample,
		Aliases: []string{"node", "data-plane-nodes", "data-plane-node"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayNodeFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayNodesHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayNodeFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayNodesHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayNodesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateway Nodes requires 0 or 1 arguments (ID)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 && getAIGatewayNodeIdentifier(cfg) != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("cannot specify both positional argument and --%s flag", aiGatewayNodeIDFlagName),
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

	nodeAPI := sdk.GetAIGatewayNodesAPI()
	if nodeAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Nodes client is not available",
			Err: fmt.Errorf("AI Gateway Nodes client not configured"),
		}
	}

	var identifier string
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else {
		identifier = getAIGatewayNodeIdentifier(cfg)
	}

	if identifier != "" {
		return h.getSingleNode(helper, nodeAPI, gatewayID, identifier, outType, printer)
	}
	return h.listNodes(helper, nodeAPI, gatewayID, outType, printer, cfg)
}

func (h aiGatewayNodesHandler) listNodes(
	helper cmd.Helper,
	nodeAPI helpers.AIGatewayNodesAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	nodes, err := fetchAIGatewayNodes(helper, nodeAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayNodeRecord, 0, len(nodes))
	tableRows := make([]table.Row, 0, len(nodes))
	for _, node := range nodes {
		record := aiGatewayNodeToRecord(node)
		records = append(records, record)
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Hostname,
			record.Type,
			record.Version,
			record.ConfigVersion,
			record.State,
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
		nodes,
		"",
		tableview.WithCustomTable(
			[]string{
				aiGatewayHeaderID,
				"HOSTNAME",
				aiGatewayHeaderType,
				"VERSION",
				"CONFIG",
				"STATE",
				aiGatewayHeaderUpdated,
			},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index < 0 || index >= len(nodes) {
				return ""
			}
			return aiGatewayNodeDetailView(nodes[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayNode, func(index int) any {
			if index < 0 || index >= len(nodes) {
				return nil
			}
			return &nodes[index]
		}),
	)
}

func (h aiGatewayNodesHandler) getSingleNode(
	helper cmd.Helper,
	nodeAPI helpers.AIGatewayNodesAPI,
	gatewayID string,
	nodeID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	res, err := nodeAPI.GetAiGatewayNode(helper.GetContext(), gatewayID, nodeID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway Node", err, helper.GetCmd(), attrs...)
	}
	node := res.GetAIGatewayDataPlaneNode()
	if node == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Node response was empty",
			Err: fmt.Errorf("no Node returned for id %s", nodeID),
		}
	}

	record := aiGatewayNodeToRecord(*node)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		node,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index != 0 {
				return ""
			}
			return aiGatewayNodeDetailView(*node)
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayNode, func(index int) any {
			if index != 0 {
				return nil
			}
			return node
		}),
	)
}

func fetchAIGatewayNodes(
	helper cmd.Helper,
	nodeAPI helpers.AIGatewayNodesAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayDataPlaneNode, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayDataPlaneNode

	for {
		req := kkOps.ListAiGatewayNodesRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := nodeAPI.ListAiGatewayNodes(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list AI Gateway Nodes", err, helper.GetCmd(), attrs...)
		}
		if res.GetListAIGatewayDataPlaneNodesResponse() == nil {
			break
		}

		allData = append(allData, res.GetListAIGatewayDataPlaneNodesResponse().Data...)
		nextCursor := pagination.ExtractPageAfterCursor(res.GetListAIGatewayDataPlaneNodesResponse().Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func aiGatewayNodeToRecord(node kkComps.AIGatewayDataPlaneNode) aiGatewayNodeRecord {
	record := aiGatewayNodeRecord{
		ID:               aiGatewayMissingValue,
		Hostname:         valueOrMissing(aiGatewayNodeHostname(node)),
		Type:             valueOrMissing(aiGatewayNodeType(node)),
		Version:          valueOrMissing(aiGatewayNodeVersion(node)),
		ConfigVersion:    valueOrMissing(aiGatewayNodeConfigVersion(node)),
		State:            valueOrMissing(aiGatewayNodeCompatibilityState(node)),
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if id := aiGatewayNodeID(node); id != "" {
		record.ID = util.AbbreviateUUID(id)
	}
	if updatedAt := aiGatewayNodeUpdatedAt(node); !updatedAt.IsZero() {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func aiGatewayNodeID(node kkComps.AIGatewayDataPlaneNode) string {
	return aiGatewayNodeStringField(node, "id")
}

func aiGatewayNodeVersion(node kkComps.AIGatewayDataPlaneNode) string {
	return aiGatewayNodeStringField(node, "version")
}

func aiGatewayNodeHostname(node kkComps.AIGatewayDataPlaneNode) string {
	return aiGatewayNodeStringField(node, "hostname")
}

func aiGatewayNodeType(node kkComps.AIGatewayDataPlaneNode) string {
	return aiGatewayNodeStringField(node, "type")
}

func aiGatewayNodeConfigVersion(node kkComps.AIGatewayDataPlaneNode) string {
	return aiGatewayNodeStringField(node, "config_version")
}

func aiGatewayNodeUpdatedAt(node kkComps.AIGatewayDataPlaneNode) time.Time {
	value := aiGatewayNodeStringField(node, aiGatewayFieldUpdatedAt)
	if value == "" {
		return time.Time{}
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return updatedAt
}

func aiGatewayNodeStringField(node kkComps.AIGatewayDataPlaneNode, key string) string {
	payload := make(map[string]any)
	data, err := json.Marshal(node)
	if err != nil {
		return ""
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	if value, ok := payload[key].(string); ok {
		return value
	}
	return ""
}

func aiGatewayNodeCompatibilityState(node kkComps.AIGatewayDataPlaneNode) string {
	if node.CompatibilityStatus.State == nil {
		return ""
	}
	return *node.CompatibilityStatus.State
}

func aiGatewayNodeDetailView(node kkComps.AIGatewayDataPlaneNode) string {
	payload := make(map[string]any)
	data, err := json.Marshal(node)
	if err == nil {
		_ = json.Unmarshal(data, &payload)
	}

	order := []string{
		"id",
		"version",
		"hostname",
		"last_ping",
		"type",
		"config_version",
		"errors",
		"compatibility_status",
		aiGatewayFieldCreatedAt,
		aiGatewayFieldUpdatedAt,
	}

	var b strings.Builder
	for _, field := range order {
		fmt.Fprintf(&b, "%s: %s\n", field, formatAIGatewayModelDetailValue(payload[field]))
	}
	return strings.TrimRight(b.String(), "\n")
}
