package eventgateway

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"
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
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	listenersCommandName = "listeners"
)

type listenerSummaryRecord struct {
	ID               string
	Name             string
	Description      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

var (
	listenersUse = listenersCommandName

	listenersShort = i18n.T("root.products.konnect.eventgateway.listenersShort",
		"Manage listeners for an Event Gateway")
	listenersLong = normalizers.LongDesc(i18n.T("root.products.konnect.eventgateway.listenersLong",
		`Use the listeners command to list or retrieve listeners for a specific Event Gateway.`))
	listenersExample = normalizers.Examples(
		i18n.T("root.products.konnect.eventgateway.listenersExamples",
			fmt.Sprintf(`
# List listeners for an event gateway by ID
%[1]s get event-gateway listeners --gateway-id <gateway-id>
# List listeners for an event gateway by name
%[1]s get event-gateway listeners --gateway-name my-gateway
# Get a specific listener by ID (positional argument)
%[1]s get event-gateway listeners --gateway-id <gateway-id> <listener-id>
# Get a specific listener by name (positional argument)
%[1]s get event-gateway listeners --gateway-id <gateway-id> my-listener
# Get a specific listener by ID (flag)
%[1]s get event-gateway listeners --gateway-id <gateway-id> --listener-id <listener-id>
# Get a specific listener by name (flag)
%[1]s get event-gateway listeners --gateway-name my-gateway --listener-name my-listener
`, meta.CLIName)))
)

func newGetEventGatewayListenersCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     listenersUse,
		Short:   listenersShort,
		Long:    listenersLong,
		Example: listenersExample,
		Aliases: []string{"listener", "ln", "lns"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			if err := bindEventGatewayChildFlags(cmd, args); err != nil {
				return err
			}
			return bindListenerChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := listenersHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addEventGatewayChildFlags(cmd)
	addListenerChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type listenersHandler struct {
	cmd *cobra.Command
}

func (h listenersHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing listeners requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	// Check if positional arg and flags are both provided
	if len(args) == 1 {
		listenerID, listenerName := getListenerIdentifiers(cfg)
		if listenerID != "" || listenerName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					listenerIDFlagName,
					listenerNameFlagName,
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

	gatewayID, gatewayName := getEventGatewayIdentifiers(cfg)
	if gatewayID != "" && gatewayName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", gatewayIDFlagName, gatewayNameFlagName),
		}
	}

	if gatewayID == "" && gatewayName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"an event gateway identifier is required. Provide --%s or --%s",
				gatewayIDFlagName,
				gatewayNameFlagName,
			),
		}
	}

	if gatewayID == "" {
		gatewayID, err = resolveEventGatewayIDByName(gatewayName, sdk.GetEventGatewayControlPlaneAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	listenerAPI := sdk.GetEventGatewayListenerAPI()
	if listenerAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Listeners client is not available",
			Err: fmt.Errorf("listeners client not configured"),
		}
	}

	// Determine if we're getting a single listener or listing all
	listenerID, listenerName := getListenerIdentifiers(cfg)
	var listenerIdentifier string

	if len(args) == 1 {
		listenerIdentifier = strings.TrimSpace(args[0])
	} else if listenerID != "" {
		listenerIdentifier = listenerID
	} else if listenerName != "" {
		listenerIdentifier = listenerName
	}

	// Validate mutual exclusivity of listener ID and name flags
	if listenerID != "" && listenerName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				listenerIDFlagName,
				listenerNameFlagName,
			),
		}
	}

	if listenerIdentifier != "" {
		return h.getSingleListener(
			helper,
			listenerAPI,
			gatewayID,
			listenerIdentifier,
			outType,
			printer,
			cfg,
		)
	}

	return h.listListeners(helper, listenerAPI, gatewayID, outType, printer, cfg)
}

func (h listenersHandler) listListeners(
	helper cmd.Helper,
	listenerAPI helpers.EventGatewayListenerAPI,
	gatewayID string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	listeners, err := fetchListeners(helper, listenerAPI, gatewayID, cfg, "")
	if err != nil {
		return err
	}

	records := make([]listenerSummaryRecord, 0, len(listeners))
	for _, listener := range listeners {
		records = append(records, listenerToRecord(listener))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		listeners,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME"}, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h listenersHandler) getSingleListener(
	helper cmd.Helper,
	listenerAPI helpers.EventGatewayListenerAPI,
	gatewayID string,
	identifier string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	listenerID := identifier
	if !util.IsValidUUID(identifier) {
		// Use name filter to optimize the API query
		listeners, err := fetchListeners(helper, listenerAPI, gatewayID, cfg, identifier)
		if err != nil {
			return err
		}
		match := findListenerByName(listeners, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("listener %q not found", identifier),
			}
		}
		if match.ID != "" {
			listenerID = match.ID
		} else {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("listener %q does not have an ID", identifier),
			}
		}
	}

	res, err := listenerAPI.FetchEventGatewayListener(helper.GetContext(), gatewayID, listenerID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get listener", err, helper.GetCmd(), attrs...)
	}

	listener := res.GetEventGatewayListener()
	if listener == nil {
		return &cmd.ExecutionError{
			Msg: "Listener response was empty",
			Err: fmt.Errorf("no listener returned for id %s", listenerID),
		}
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		listenerToRecord(*listener),
		listener,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func fetchListeners(
	helper cmd.Helper,
	listenerAPI helpers.EventGatewayListenerAPI,
	gatewayID string,
	cfg config.Hook,
	nameFilter string,
) ([]kkComps.EventGatewayListener, error) {
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.EventGatewayListener
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewayListenersRequest{
			GatewayID: gatewayID,
			PageSize:  kk.Int64(requestPageSize),
		}

		// Apply name filter if provided
		if nameFilter != "" {
			req.Filter = &kkComps.EventGatewayCommonFilter{
				Name: &kkComps.StringFieldContainsFilter{
					Contains: nameFilter,
				},
			}
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := listenerAPI.ListEventGatewayListeners(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list listeners", err, helper.GetCmd(), attrs...)
		}

		if res.GetListEventGatewayListenersResponse() == nil {
			break
		}

		data := res.GetListEventGatewayListenersResponse().Data
		allData = append(allData, data...)

		if res.GetListEventGatewayListenersResponse().Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.GetListEventGatewayListenersResponse().Meta.Page.Next)
		if err != nil {
			return nil, cmd.PrepareExecutionError(
				"Failed to list listeners: invalid cursor",
				err,
				helper.GetCmd(),
			)
		}

		values := u.Query()
		pageAfter = kk.String(values.Get("page[after]"))
	}

	return allData, nil
}

func findListenerByName(listeners []kkComps.EventGatewayListener, name string) *kkComps.EventGatewayListener {
	lowered := strings.ToLower(name)
	for _, listener := range listeners {
		if listener.Name != "" && strings.ToLower(listener.Name) == lowered {
			listenerCopy := listener
			return &listenerCopy
		}
	}
	return nil
}

func listenerToRecord(listener kkComps.EventGatewayListener) listenerSummaryRecord {
	id := listener.ID
	if id != "" {
		id = util.AbbreviateUUID(id)
	} else {
		id = valueNA
	}

	name := listener.Name
	if name == "" {
		name = valueNA
	}

	description := valueNA
	if listener.Description != nil && *listener.Description != "" {
		description = *listener.Description
	}

	createdAt := listener.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	updatedAt := listener.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return listenerSummaryRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}
