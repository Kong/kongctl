package api

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	implementationsCommandName = "implementations"
)

type apiImplementationRecord struct {
	ImplementationID string
	ServiceID        string
	ControlPlaneID   string
	LocalCreatedTime string
	LocalUpdatedTime string
}

type apiImplementationFields struct {
	id             string
	apiID          string
	serviceID      string
	controlPlaneID string
	createdAt      time.Time
	updatedAt      time.Time
}

func (f apiImplementationFields) displayServiceID() string {
	if f.serviceID == "" {
		return "n/a"
	}
	return f.serviceID
}

func (f apiImplementationFields) displayControlPlaneID() string {
	if f.controlPlaneID == "" {
		return "n/a"
	}
	return f.controlPlaneID
}

var (
	implementationsUse = implementationsCommandName

	implementationsShort = i18n.T("root.products.konnect.api.implementationsShort",
		"Manage API implementations for a Konnect API")
	implementationsLong = normalizers.LongDesc(i18n.T("root.products.konnect.api.implementationsLong",
		`Use the implementations command to list API implementations for a specific Konnect API.`))
	implementationsExample = normalizers.Examples(
		i18n.T("root.products.konnect.api.implementationsExamples",
			fmt.Sprintf(`
# List implementations for an API by ID
%[1]s get api implementations --api-id <api-id>
# List implementations for an API by name
%[1]s get api implementations --api-name my-api
# Get a specific implementation by ID
%[1]s get api implementations --api-id <api-id> <implementation-id>
# Get an implementation by service ID
%[1]s get api implementations --api-id <api-id> <service-id>
`, meta.CLIName)),
	)
)

func newGetAPIImplementationsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     implementationsUse,
		Short:   implementationsShort,
		Long:    implementationsLong,
		Example: implementationsExample,
		Aliases: []string{"implementation", "impls", "impl"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindAPIChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := apiImplementationsHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addAPIChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type apiImplementationsHandler struct {
	cmd *cobra.Command
}

func (h apiImplementationsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"too many arguments. Listing API implementations requires 0 or 1 arguments (implementation or service ID)",
			),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
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

	apiID, apiName := getAPIIdentifiers(cfg)
	if apiID != "" && apiName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", apiIDFlagName, apiNameFlagName),
		}
	}

	if apiID == "" && apiName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("an API identifier is required. Provide --%s or --%s", apiIDFlagName, apiNameFlagName),
		}
	}

	if apiID == "" {
		apiID, err = resolveAPIIDByName(apiName, sdk.GetAPIAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	implementationAPI := sdk.GetAPIImplementationAPI()
	if implementationAPI == nil {
		return &cmd.ExecutionError{
			Msg: "API implementations client is not available",
			Err: fmt.Errorf("api implementations client not configured"),
		}
	}

	implementations, err := fetchImplementations(helper, implementationAPI, apiID, cfg)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		identifier := strings.TrimSpace(args[0])
		implementations = filterImplementations(implementations, identifier)
		if len(implementations) == 0 {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("implementation %q not found", identifier),
			}
		}
	}

	displayRecords := make([]apiImplementationRecord, 0, len(implementations))
	tableRows := make([]table.Row, 0, len(implementations))
	for i := range implementations {
		record := implementationToRecord(implementations[i])
		displayRecords = append(displayRecords, record)
		tableRows = append(tableRows, table.Row{record.ImplementationID, record.ServiceID, record.ControlPlaneID})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(implementations) {
			return ""
		}
		return implementationDetailView(&implementations[index])
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		displayRecords,
		implementations,
		"",
		tableview.WithTitle("Implementations"),
		apiImplementationTableOption(tableRows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailContext(common.ViewParentAPIImplementation, func(index int) any {
			if index < 0 || index >= len(implementations) {
				return nil
			}
			return &implementations[index]
		}),
		tableview.WithDetailHelper(helper),
	)
}

func apiImplementationTableOption(rows []table.Row) tableview.Option {
	return tableview.WithExactCustomTable([]string{"IMPLEMENTATION", "SERVICE", "CONTROL PLANE"}, rows)
}

func fetchImplementations(
	helper cmd.Helper,
	implementationAPI helpers.APIImplementationAPI,
	apiID string,
	cfg config.Hook,
) ([]kkComps.APIImplementationListItem, error) {
	var pageNumber int64 = 1
	pageSize := common.ResolveRequestPageSize(cfg)

	var all []kkComps.APIImplementationListItem

	filter := &kkComps.APIImplementationFilterParameters{
		APIID: &kkComps.UUIDFieldFilter{Eq: new(apiID)},
	}

	for {
		req := kkOps.ListAPIImplementationsRequest{
			PageSize:   new(pageSize),
			PageNumber: new(pageNumber),
			Filter:     filter,
		}

		res, err := implementationAPI.ListAPIImplementations(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list API implementations", err, helper.GetCmd(), attrs...)
		}

		if res.GetListAPIImplementationsResponse() == nil {
			break
		}

		data := res.GetListAPIImplementationsResponse().GetData()
		all = append(all, data...)

		total := int(res.GetListAPIImplementationsResponse().GetMeta().Page.Total)
		if !common.HasMorePageNumberResults(total, len(all), len(data)) {
			break
		}

		pageNumber++
	}

	return all, nil
}

func filterImplementations(
	implementations []kkComps.APIImplementationListItem,
	identifier string,
) []kkComps.APIImplementationListItem {
	matches := make([]kkComps.APIImplementationListItem, 0)
	for _, implementation := range implementations {
		fields, ok := getAPIImplementationFields(implementation)
		if ok && (strings.EqualFold(fields.id, identifier) ||
			(fields.serviceID != "" && strings.EqualFold(fields.serviceID, identifier))) {
			matches = append(matches, implementation)
		}
	}

	return matches
}

func implementationToRecord(implementation kkComps.APIImplementationListItem) apiImplementationRecord {
	fields, ok := getAPIImplementationFields(implementation)
	if !ok {
		return apiImplementationRecord{}
	}

	return apiImplementationRecord{
		ImplementationID: fields.id,
		ServiceID:        fields.displayServiceID(),
		ControlPlaneID:   fields.displayControlPlaneID(),
		LocalCreatedTime: fields.createdAt.In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: fields.updatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func implementationDetailView(implementation *kkComps.APIImplementationListItem) string {
	if implementation == nil {
		return ""
	}

	implementationFields, ok := getAPIImplementationFields(*implementation)
	if !ok {
		return ""
	}

	fields := map[string]string{
		"api_id":           implementationFields.apiID,
		"control_plane_id": implementationFields.displayControlPlaneID(),
		"created_at":       implementationFields.createdAt.In(time.Local).Format("2006-01-02 15:04:05"),
		"service_id":       implementationFields.displayServiceID(),
		"updated_at":       implementationFields.updatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
	}

	keys := slices.Sorted(maps.Keys(fields))

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", implementationFields.id)
	for _, key := range keys {
		fmt.Fprintf(&b, "%s: %s\n", key, fields[key])
	}

	return strings.TrimRight(b.String(), "\n")
}

func getAPIImplementationFields(
	implementation kkComps.APIImplementationListItem,
) (apiImplementationFields, bool) {
	if entity := implementation.APIImplementationListItemGatewayServiceEntity; entity != nil {
		fields := apiImplementationFields{
			id:        entity.GetID(),
			apiID:     entity.GetAPIID(),
			createdAt: entity.GetCreatedAt(),
			updatedAt: entity.GetUpdatedAt(),
		}
		if service := entity.GetService(); service != nil {
			fields.serviceID = service.GetID()
			fields.controlPlaneID = service.GetControlPlaneID()
		}
		return fields, true
	}

	if entity := implementation.APIImplementationListItemControlPlaneEntity; entity != nil {
		controlPlane := entity.GetControlPlane()
		return apiImplementationFields{
			id:             entity.GetID(),
			apiID:          entity.GetAPIID(),
			controlPlaneID: controlPlane.GetID(),
			createdAt:      entity.GetCreatedAt(),
			updatedAt:      entity.GetUpdatedAt(),
		}, true
	}

	return apiImplementationFields{}, false
}
