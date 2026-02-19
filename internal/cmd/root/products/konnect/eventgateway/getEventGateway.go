package eventgateway

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
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
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

var (
	getEventGatewayControlPlanesShort = i18n.T(
		"root.products.konnect.event-gateway.control-plane.getEventGatewayControlPlanesShort",
		"List or get Konnect Event Gateways")
	getEventGatewayControlPlanesLong = i18n.T(
		"root.products.konnect.event-gateway.control-plane.getEventGatewayControlPlanesLong",
		`Use the get verb with the event-gateway command to query Konnect Event Gateways.`)
	getEventGatewayControlPlanesExample = normalizers.Examples(
		i18n.T("root.products.konnect.event-gateway.control-plane.getEventGatewayControlPlanesExamples",
			fmt.Sprintf(`# List all the Event Gateways for the organization
%[1]s get event-gateway
# Get details for an Event Gateway with a specific ID
%[1]s get event-gateway 22cd8a0b-72e7-4212-9099-0764f8e9c5ac
# Get details for an Event Gateway with a specific name
%[1]s get event-gateway my-eventgateway
# Get all the Event Gateways using command aliases
%[1]s get egw
`, meta.CLIName)))
)

type textDisplayRecord struct {
	ID               string
	Name             string
	Description      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

func eventGatewayControlPlaneToDisplayRecord(e *kkComps.EventGatewayInfo) textDisplayRecord {
	missing := "n/a"

	var id, name string
	if e.ID != "" {
		id = util.AbbreviateUUID(e.ID)
	} else {
		id = missing
	}

	if e.Name != "" {
		name = e.Name
	} else {
		name = missing
	}

	description := missing
	if e.Description != nil && *e.Description != "" {
		description = *e.Description
	}

	createdAt := e.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := e.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	return textDisplayRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

type getEventGatewayControlPlaneCmd struct {
	*cobra.Command
}

// runListByName retrieves an Event Gateway Control Plane by its name
// TODO: Since the API does not support filtering by name, we fetch all and filter locally
func runListByName(name string, kkClient helpers.EGWControlPlaneAPI, helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.EventGatewayInfo, error) {
	allEventGateways, err := runList(kkClient, helper, cfg)
	if err != nil {
		return nil, err
	}

	for _, eventGateway := range allEventGateways {
		if eventGateway.Name == name {
			return &eventGateway, nil
		}
	}

	return nil, cmd.PrepareExecutionErrorMsg(helper,
		fmt.Sprintf("Event Gateway Control Plane with name %s not found", name))
}

func runList(kkClient helpers.EGWControlPlaneAPI, helper cmd.Helper,
	cfg config.Hook,
) ([]kkComps.EventGatewayInfo, error) {
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.EventGatewayInfo
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewaysRequest{
			PageSize: kk.Int64(requestPageSize),
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := kkClient.ListEGWControlPlanes(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list Event Gateways", err, helper.GetCmd(), attrs...)
		}

		allData = append(allData, res.ListEventGatewaysResponse.Data...)

		if res.ListEventGatewaysResponse.Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.ListEventGatewaysResponse.Meta.Page.Next)
		if err != nil {
			return nil, cmd.PrepareExecutionError("Failed to list Event Gateways: invalid cursor", err, helper.GetCmd())
		}

		values := u.Query()
		pageAfter = kk.String(values.Get("page[after]"))
	}

	return allData, nil
}

func runGet(id string, kkClient helpers.EGWControlPlaneAPI, helper cmd.Helper,
) (*kkComps.EventGatewayInfo, error) {
	res, err := kkClient.FetchEGWControlPlane(helper.GetContext(), id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError(
			"Failed to get Event Gateway Control Plane",
			err,
			helper.GetCmd(),
			attrs...)
	}

	return res.GetEventGatewayInfo(), nil
}

func (c *getEventGatewayControlPlaneCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing Event Gateways requires 0 or 1 arguments (name or ID)"),
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

func (c *getEventGatewayControlPlaneCmd) runE(cobraCmd *cobra.Command, args []string) error {
	var e error
	helper := cmd.BuildHelper(cobraCmd, args)
	if e = c.validate(helper); e != nil {
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

	var printer cli.PrintFlusher

	printer, e = cli.Format(outType.String(), helper.GetStreams().Out)
	if e != nil {
		return e
	}
	defer printer.Flush()

	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	sdk, e := helper.GetKonnectSDK(cfg, logger)
	if e != nil {
		return e
	}

	// 'get event-gateway' can be run in various ways:
	//    > get event-gateway <id>    # Get by UUID
	//  > get event-gateway <name>  # Get by name
	//  > get event-gateway         # List all
	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		id := strings.TrimSpace(helper.GetArgs()[0])

		isUUID := util.IsValidUUID(id)

		if !isUUID {
			// If the ID is not a UUID, then it is a name
			eventGatewayControlPlane, e := runListByName(id, sdk.GetEventGatewayControlPlaneAPI(), helper, cfg)
			if e != nil {
				return e
			}
			return tableview.RenderForFormat(helper,
				false,
				outType,
				printer,
				helper.GetStreams(),
				eventGatewayControlPlaneToDisplayRecord(eventGatewayControlPlane),
				eventGatewayControlPlane,
				"",
				tableview.WithRootLabel(helper.GetCmd().Name()),
				tableview.WithDetailHelper(helper),
				tableview.WithDetailContext("eventGatewayControlPlane", func(index int) any {
					if index != 0 {
						return nil
					}
					return eventGatewayControlPlane
				}),
			)
		}

		eventGatewayControlPlane, e := runGet(id, sdk.GetEventGatewayControlPlaneAPI(), helper)
		if e != nil {
			return e
		}

		return tableview.RenderForFormat(helper,
			false,
			outType,
			printer,
			helper.GetStreams(),
			eventGatewayControlPlaneToDisplayRecord(eventGatewayControlPlane),
			eventGatewayControlPlane,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
			tableview.WithDetailHelper(helper),
			tableview.WithDetailContext("eventGatewayControlPlane", func(index int) any {
				if index != 0 {
					return nil
				}
				return eventGatewayControlPlane
			}),
		)
	}

	eventGatewayControlPlanes, e := runList(sdk.GetEventGatewayControlPlaneAPI(), helper, cfg)
	if e != nil {
		return e
	}

	return renderEventGatewayControlPlaneList(
		helper,
		helper.GetCmd().Name(),
		false,
		outType,
		printer,
		eventGatewayControlPlanes,
	)
}

func renderEventGatewayControlPlaneList(
	helper cmd.Helper,
	rootLabel string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	eventGatewayControlPlanes []kkComps.EventGatewayInfo,
) error {
	displayRecords := make([]textDisplayRecord, 0, len(eventGatewayControlPlanes))
	for i := range eventGatewayControlPlanes {
		displayRecords = append(displayRecords, eventGatewayControlPlaneToDisplayRecord(&eventGatewayControlPlanes[i]))
	}

	options := []tableview.Option{
		tableview.WithRootLabel(rootLabel),
		tableview.WithDetailHelper(helper),
	}

	return tableview.RenderForFormat(helper,
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		displayRecords,
		eventGatewayControlPlanes,
		"",
		options...,
	)
}

func buildEventGatewayChildView(eventGatewayControlPlanes []kkComps.EventGatewayInfo) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(eventGatewayControlPlanes))
	for i := range eventGatewayControlPlanes {
		record := eventGatewayControlPlaneToDisplayRecord(&eventGatewayControlPlanes[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(eventGatewayControlPlanes) {
			return ""
		}
		return eventGatewayDetailView(&eventGatewayControlPlanes[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Event Gateways",
		ParentType:     "event-gateway",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(eventGatewayControlPlanes) {
				return nil
			}
			return &eventGatewayControlPlanes[index]
		},
	}
}

func eventGatewayDetailView(gateway *kkComps.EventGatewayInfo) string {
	if gateway == nil {
		return ""
	}

	id := strings.TrimSpace(gateway.ID)
	if id == "" {
		id = valueNA
	}

	name := strings.TrimSpace(gateway.Name)
	if name == "" {
		name = valueNA
	}

	description := valueNA
	if gateway.Description != nil && strings.TrimSpace(*gateway.Description) != "" {
		description = strings.TrimSpace(*gateway.Description)
	}

	labels := formatLabelPairs(gateway.Labels)

	createdAt := gateway.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := gateway.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", description)
	fmt.Fprintf(&b, "labels: %s\n", labels)

	if value, ok := optionalFieldValue(gateway, "Version"); ok {
		fmt.Fprintf(&b, "version: %s\n", value)
	}
	if value, ok := optionalFieldValue(gateway, "MinRuntimeVersion"); ok {
		fmt.Fprintf(&b, "min_runtime_version: %s\n", value)
	}
	if value, ok := optionalFieldValue(gateway, "NodesTotal"); ok {
		fmt.Fprintf(&b, "nodes_total: %s\n", value)
	}
	if value, ok := optionalFieldValue(gateway, "VirtualClustersTotal"); ok {
		fmt.Fprintf(&b, "virtual_clusters_total: %s\n", value)
	}

	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)

	return strings.TrimRight(b.String(), "\n")
}

func optionalFieldValue(source any, fieldName string) (string, bool) {
	if source == nil || strings.TrimSpace(fieldName) == "" {
		return "", false
	}

	value := reflect.ValueOf(source)
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return "", false
	}

	field := value.FieldByName(fieldName)
	if !field.IsValid() {
		return "", false
	}

	for field.Kind() == reflect.Pointer {
		if field.IsNil() {
			return "", false
		}
		field = field.Elem()
	}

	switch field.Kind() {
	case reflect.Invalid:
		return "", false
	case reflect.String:
		val := strings.TrimSpace(field.String())
		if val == "" {
			return "", false
		}
		return val, true
	case reflect.Bool:
		return fmt.Sprintf("%t", field.Bool()), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", field.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return fmt.Sprintf("%d", field.Uint()), true
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%v", field.Float()), true
	case reflect.Complex64, reflect.Complex128:
		return fmt.Sprint(field.Complex()), true
	case reflect.Array, reflect.Slice:
		if field.Len() == 0 {
			return "", false
		}
		return fmt.Sprint(field.Interface()), true
	case reflect.Map:
		if field.Len() == 0 {
			return "", false
		}
		return fmt.Sprint(field.Interface()), true
	case reflect.Struct:
		return fmt.Sprint(field.Interface()), true
	case reflect.Interface, reflect.Pointer:
		if field.IsNil() {
			return "", false
		}
		return fmt.Sprint(field.Interface()), true
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return fmt.Sprint(field.Interface()), true
	}

	return "", false
}

func newGetEventGatewayControlPlaneCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getEventGatewayControlPlaneCmd {
	rv := getEventGatewayControlPlaneCmd{
		Command: baseCmd,
	}

	rv.Short = getEventGatewayControlPlanesShort
	rv.Long = getEventGatewayControlPlanesLong
	rv.Example = getEventGatewayControlPlanesExample
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	// Ensure parent-level flags are available on this command
	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	// Add child commands
	backendClustersCmd := newGetEventGatewayBackendClustersCmd(verb, addParentFlags, parentPreRun)
	if backendClustersCmd != nil {
		rv.AddCommand(backendClustersCmd)
	}

	virtualClustersCmd := newGetEventGatewayVirtualClustersCmd(verb, addParentFlags, parentPreRun)
	if virtualClustersCmd != nil {
		rv.AddCommand(virtualClustersCmd)
	}

	return &rv
}
