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

type aiGatewayMCPServerRecord struct {
	ID               string
	Name             string
	DisplayName      string
	Type             string
	Enabled          string
	LocalUpdatedTime string
}

var (
	aiGatewayMCPServersUse   = "mcp-servers [mcp-server-id|mcp-server-name]"
	aiGatewayMCPServersShort = i18n.T(
		"root.products.konnect.ai-gateway.mcpServersShort",
		"List or get MCP Servers for a Konnect AI Gateway",
	)
	aiGatewayMCPServersLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.mcpServersLong",
		`Use the mcp-servers command to list or retrieve MCP Servers for a specific Konnect AI Gateway.`,
	))
	aiGatewayMCPServersExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.mcpServersExamples",
			fmt.Sprintf(`# List MCP Servers for an AI Gateway by display name
%[1]s get ai-gateway mcp-servers --gateway-name "Customer Support Gateway"
# List MCP Servers for an AI Gateway by ID
%[1]s get ai-gateway mcp-servers --gateway-id <gateway-id>
# Get an MCP Server by name
%[1]s get ai-gateway mcp-servers --gateway-name "Customer Support Gateway" customer-support-tools
# Get an MCP Server by ID
%[1]s get ai-gateway mcp-servers --gateway-id <gateway-id> --mcp-server-id <mcp-server-id>
`, meta.CLIName)),
	)
)

func newGetAIGatewayMCPServersCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayMCPServersUse,
		Short:   aiGatewayMCPServersShort,
		Long:    aiGatewayMCPServersLong,
		Example: aiGatewayMCPServersExample,
		Aliases: []string{"mcp-server"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayMCPServerFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayMCPServersHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayMCPServerFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayMCPServersHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayMCPServersHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateway MCP Servers requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		serverID, serverName := getAIGatewayMCPServerIdentifiers(cfg)
		if serverID != "" || serverName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayMCPServerIDFlagName,
					aiGatewayMCPServerNameFlagName,
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

	serverAPI := sdk.GetAIGatewayMCPServersAPI()
	if serverAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway MCP Servers client is not available",
			Err: fmt.Errorf("AI Gateway MCP Servers client not configured"),
		}
	}

	serverID, serverName := getAIGatewayMCPServerIdentifiers(cfg)
	if serverID != "" && serverName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayMCPServerIDFlagName,
				aiGatewayMCPServerNameFlagName,
			),
		}
	}

	identifier := ""
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else if serverID != "" {
		identifier = serverID
	} else if serverName != "" {
		identifier = serverName
	}

	if identifier != "" {
		return h.getSingleMCPServer(helper, serverAPI, gatewayID, identifier, outType, printer)
	}
	return h.listMCPServers(helper, serverAPI, gatewayID, outType, printer, cfg)
}

func (h aiGatewayMCPServersHandler) listMCPServers(
	helper cmd.Helper,
	serverAPI helpers.AIGatewayMCPServersAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	servers, err := fetchAIGatewayMCPServers(helper, serverAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayMCPServerRecord, 0, len(servers))
	tableRows := make([]table.Row, 0, len(servers))
	for _, server := range servers {
		record := aiGatewayMCPServerToRecord(server)
		records = append(records, record)
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.Type,
			record.Enabled,
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
		servers,
		"",
		tableview.WithCustomTable(
			[]string{
				aiGatewayHeaderID,
				aiGatewayHeaderName,
				aiGatewayHeaderDisplayName,
				aiGatewayHeaderType,
				"ENABLED",
				aiGatewayHeaderUpdated,
			},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index < 0 || index >= len(servers) {
				return ""
			}
			return aiGatewayMCPServerDetailView(servers[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayMCPServer, func(index int) any {
			if index < 0 || index >= len(servers) {
				return nil
			}
			return &servers[index]
		}),
	)
}

func (h aiGatewayMCPServersHandler) getSingleMCPServer(
	helper cmd.Helper,
	serverAPI helpers.AIGatewayMCPServersAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	res, err := serverAPI.GetAiGatewayMcpServer(helper.GetContext(), gatewayID, identifier)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway MCP Server", err, helper.GetCmd(), attrs...)
	}
	server := res.GetAIGatewayMCPServer()
	if server == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway MCP Server response was empty",
			Err: fmt.Errorf("no MCP Server returned for id or name %s", identifier),
		}
	}

	record := aiGatewayMCPServerToRecord(*server)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		server,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index != 0 {
				return ""
			}
			return aiGatewayMCPServerDetailView(*server)
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayMCPServer, func(index int) any {
			if index != 0 {
				return nil
			}
			return server
		}),
	)
}

func fetchAIGatewayMCPServers(
	helper cmd.Helper,
	serverAPI helpers.AIGatewayMCPServersAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayMCPServer, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayMCPServer

	for {
		req := kkOps.ListAiGatewayMcpServersRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := serverAPI.ListAiGatewayMcpServers(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list AI Gateway MCP Servers", err, helper.GetCmd(), attrs...)
		}
		if res.GetListAIGatewayMCPServersResponse() == nil {
			break
		}

		allData = append(allData, res.GetListAIGatewayMCPServersResponse().Data...)
		nextCursor := pagination.ExtractPageAfterCursor(res.GetListAIGatewayMCPServersResponse().Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func aiGatewayMCPServerToRecord(server kkComps.AIGatewayMCPServer) aiGatewayMCPServerRecord {
	record := aiGatewayMCPServerRecord{
		ID:               aiGatewayMissingValue,
		Name:             valueOrMissing(declresources.AIGatewayMCPServerName(server)),
		DisplayName:      valueOrMissing(declresources.AIGatewayMCPServerDisplayName(server)),
		Type:             valueOrMissing(declresources.AIGatewayMCPServerType(server)),
		Enabled:          aiGatewayMissingValue,
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if id := declresources.AIGatewayMCPServerID(server); id != "" {
		record.ID = util.AbbreviateUUID(id)
	}
	if enabled := declresources.AIGatewayMCPServerEnabled(server); enabled != nil {
		record.Enabled = fmt.Sprintf("%t", *enabled)
	}
	if updatedAt := declresources.AIGatewayMCPServerUpdatedAt(server); !updatedAt.IsZero() {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func aiGatewayMCPServerDetailView(server kkComps.AIGatewayMCPServer) string {
	payload := make(map[string]any)
	data, err := json.Marshal(server)
	if err == nil {
		// Detail views are best-effort; leave missing fields as n/a if SDK union data cannot round-trip.
		_ = json.Unmarshal(data, &payload)
	}

	order := []string{
		"id",
		"name",
		"display_name",
		"type",
		"enabled",
		"acl_attribute_type",
		"acls",
		"default_tool_acls",
		"config",
		"tools",
		"policies",
		aiGatewayFieldLabels,
		aiGatewayFieldManagedBy,
		aiGatewayFieldCreatedAt,
		aiGatewayFieldUpdatedAt,
	}

	var b strings.Builder
	for _, field := range order {
		fmt.Fprintf(&b, "%s: %s\n", field, formatAIGatewayModelDetailValue(payload[field]))
	}
	return strings.TrimRight(b.String(), "\n")
}

func buildAIGatewayMCPServerChildView(servers []kkComps.AIGatewayMCPServer) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(servers))
	for i := range servers {
		record := aiGatewayMCPServerToRecord(servers[i])
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.Type,
			record.Enabled,
			record.LocalUpdatedTime,
		})
	}

	return tableview.ChildView{
		Headers: []string{
			aiGatewayHeaderID,
			aiGatewayHeaderName,
			aiGatewayHeaderDisplayName,
			aiGatewayHeaderType,
			aiGatewayHeaderEnabled,
			aiGatewayHeaderUpdated,
		},
		Rows: tableRows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(servers) {
				return ""
			}
			return aiGatewayMCPServerDetailView(servers[index])
		},
		Title:      "AI Gateway MCP Servers",
		ParentType: common.ViewParentAIGatewayMCPServer,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(servers) {
				return nil
			}
			return &servers[index]
		},
	}
}
