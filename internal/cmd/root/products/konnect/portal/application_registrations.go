package portal

import (
	"fmt"
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
	applicationIDFlagName   = "application-id"
	applicationNameFlagName = "application-name"
	developerIDFlagName     = "developer-id"
	statusFlagName          = "status"
)

type portalApplicationRegistrationSummaryRecord struct {
	ID               string
	Status           string
	Application      string
	API              string
	LocalCreatedTime string
	LocalUpdatedTime string
}

type portalApplicationRegistrationDetailRecord struct {
	ID               string
	Status           string
	Application      string
	ApplicationID    string
	API              string
	APIEntityType    string
	LocalCreatedTime string
	LocalUpdatedTime string
}

var (
	registrationsUse = "application-registrations"

	registrationsShort = i18n.T("root.products.konnect.portal.registrationsShort",
		"Manage portal application registrations for a Konnect portal")
	registrationsLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.registrationsLong",
		`Use the registrations command to list or retrieve application registrations for a specific Konnect portal.`))
	registrationsExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.registrationsExamples",
			fmt.Sprintf(`
# List registrations for a portal by ID
%[1]s get portal application registrations --portal-id <portal-id>
# List registrations for a portal by name
%[1]s get portal application registrations --portal-name my-portal
# List registrations for an application by name
%[1]s get portal application registrations --portal-name my-portal --application-name checkout-app
# Get a specific registration by ID
%[1]s get portal application registrations --portal-name my-portal <registration-id>
`, meta.CLIName)))
)

func newGetPortalApplicationRegistrationsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     registrationsUse,
		Short:   registrationsShort,
		Long:    registrationsLong,
		Example: registrationsExample,
		Aliases: []string{
			"registration",
			"registrations",
			"application-registration",
			"application-registrations",
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			if err := bindPortalChildFlags(cmd, args); err != nil {
				return err
			}
			return bindRegistrationFilterFlags(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := portalApplicationRegistrationsHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addPortalChildFlags(cmd)
	addRegistrationFilterFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type portalApplicationRegistrationsHandler struct {
	cmd *cobra.Command
}

func (h portalApplicationRegistrationsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"too many arguments. Listing portal application registrations requires 0 or 1 arguments (registration ID)",
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

	portalID, portalName := getPortalIdentifiers(cfg)
	if portalID != "" && portalName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", portalIDFlagName, portalNameFlagName),
		}
	}

	if portalID == "" && portalName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"a portal identifier is required. Provide --%s or --%s",
				portalIDFlagName,
				portalNameFlagName,
			),
		}
	}

	if portalID == "" {
		portalID, err = resolvePortalIDByName(portalName, sdk.GetPortalAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	regAPI := sdk.GetPortalApplicationRegistrationAPI()
	if regAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal application registrations client is not available",
			Err: fmt.Errorf("portal application registrations client not configured"),
		}
	}

	filters := registrationFiltersFromFlags(h.cmd)

	if len(args) == 1 {
		registrationID := strings.TrimSpace(args[0])
		return h.getSingleRegistration(
			helper,
			regAPI,
			portalID,
			registrationID,
			outType,
			printer,
			cfg,
			filters,
		)
	}

	return h.listRegistrations(helper, regAPI, portalID, outType, printer, cfg, filters)
}

func (h portalApplicationRegistrationsHandler) listRegistrations(
	helper cmd.Helper,
	regAPI helpers.PortalApplicationRegistrationAPI,
	portalID string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
	filters registrationFilters,
) error {
	regs, err := fetchPortalApplicationRegistrations(helper, regAPI, portalID, cfg, filters)
	if err != nil {
		return err
	}

	records := make([]portalApplicationRegistrationSummaryRecord, 0, len(regs))
	for _, reg := range regs {
		records = append(records, portalApplicationRegistrationSummaryToRecord(reg))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Status, record.Application, record.API})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(regs) {
			return ""
		}
		return portalApplicationRegistrationDetailView(&regs[index])
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		regs,
		"",
		tableview.WithCustomTable([]string{"ID", "STATUS", "APPLICATION", "API"}, tableRows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h portalApplicationRegistrationsHandler) getSingleRegistration(
	helper cmd.Helper,
	regAPI helpers.PortalApplicationRegistrationAPI,
	portalID string,
	registrationID string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
	filters registrationFilters,
) error {
	registrationID = strings.TrimSpace(registrationID)
	if registrationID == "" {
		return &cmd.ConfigurationError{Err: fmt.Errorf("registration identifier is required")}
	}

	applicationID := strings.TrimSpace(filters.ApplicationID)
	var cached *kkComps.ApplicationRegistration
	if applicationID == "" {
		all, err := fetchPortalApplicationRegistrations(helper, regAPI, portalID, cfg, registrationFilters{})
		if err != nil {
			return err
		}
		cached = findRegistrationByID(all, registrationID)
		if cached == nil {
			return &cmd.ConfigurationError{Err: fmt.Errorf("registration %q not found", registrationID)}
		}
		cachedApp := cached.GetApplication()
		applicationID = cachedApp.ID
	}

	if applicationID == "" {
		return &cmd.ExecutionError{
			Msg: "Application identifier for registration could not be determined",
			Err: fmt.Errorf("missing application id for registration %s", registrationID),
		}
	}

	req := kkOps.GetApplicationRegistrationRequest{
		PortalID:       portalID,
		ApplicationID:  applicationID,
		RegistrationID: registrationID,
	}

	res, err := regAPI.GetApplicationRegistration(helper.GetContext(), req)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get portal application registration", err, helper.GetCmd(), attrs...)
	}

	record := portalApplicationRegistrationDetailRecordFromResponse(res.GetGetApplicationRegistrationResponse())
	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		[]portalApplicationRegistrationDetailRecord{record},
		[]*kkOps.GetApplicationRegistrationResponse{res},
		"",
	)
}

type registrationFilters struct {
	ApplicationID   string
	ApplicationName string
	DeveloperID     string
	Status          string
}

func addRegistrationFilterFlags(cmd *cobra.Command) {
	cmd.Flags().String(applicationIDFlagName, "",
		"Scope to a specific application by ID (optional for list, required for get/delete if registration lookup fails)")
	cmd.Flags().String(applicationNameFlagName, "", "Scope to a specific application by name")
	cmd.Flags().String(developerIDFlagName, "", "Filter registrations by developer ID")
	cmd.Flags().String(statusFlagName, "", "Filter registrations by status (approved, pending, revoked, rejected)")
	cmd.MarkFlagsMutuallyExclusive(applicationIDFlagName, applicationNameFlagName)
}

func bindRegistrationFilterFlags(c *cobra.Command) error {
	status := strings.TrimSpace(c.Flag(statusFlagName).Value.String())
	if status != "" {
		statusLower := strings.ToLower(status)
		allowed := map[string]struct{}{
			string(kkComps.ApplicationRegistrationStatusApproved): {},
			string(kkComps.ApplicationRegistrationStatusPending):  {},
			string(kkComps.ApplicationRegistrationStatusRejected): {},
			string(kkComps.ApplicationRegistrationStatusRevoked):  {},
		}
		if _, ok := allowed[statusLower]; !ok {
			return &cmd.ConfigurationError{Err: fmt.Errorf(
				"invalid status %q; allowed values: approved, pending, revoked, rejected",
				status,
			)}
		}
		if err := c.Flags().Set(statusFlagName, statusLower); err != nil {
			return err
		}
	}

	return nil
}

func registrationFiltersFromFlags(c *cobra.Command) registrationFilters {
	filters := registrationFilters{}

	if c.Flags().Changed(applicationIDFlagName) {
		filters.ApplicationID = strings.TrimSpace(c.Flag(applicationIDFlagName).Value.String())
	}
	if c.Flags().Changed(applicationNameFlagName) {
		filters.ApplicationName = strings.TrimSpace(c.Flag(applicationNameFlagName).Value.String())
	}
	if c.Flags().Changed(developerIDFlagName) {
		filters.DeveloperID = strings.TrimSpace(c.Flag(developerIDFlagName).Value.String())
	}
	if c.Flags().Changed(statusFlagName) {
		filters.Status = strings.TrimSpace(c.Flag(statusFlagName).Value.String())
	}

	return filters
}

func fetchPortalApplicationRegistrations(
	helper cmd.Helper,
	regAPI helpers.PortalApplicationRegistrationAPI,
	portalID string,
	cfg config.Hook,
	filters registrationFilters,
) ([]kkComps.ApplicationRegistration, error) {
	var pageNumber int64 = 1
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}

	var all []kkComps.ApplicationRegistration

	for {
		req := kkOps.ListRegistrationsRequest{
			PortalID:   portalID,
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		filter := buildRegistrationFilter(filters)
		if filter != nil {
			req.Filter = filter
		}

		res, err := regAPI.ListRegistrations(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError(
				"Failed to list portal application registrations",
				err,
				helper.GetCmd(),
				attrs...,
			)
		}

		if res.GetListApplicationRegistrationsResponse() == nil {
			break
		}

		data := res.GetListApplicationRegistrationsResponse().GetData()
		all = append(all, data...)

		total := int(res.GetListApplicationRegistrationsResponse().GetMeta().Page.Total)
		if total == 0 || len(all) >= total || len(data) == 0 {
			break
		}

		pageNumber++
	}

	if filters.ApplicationName != "" {
		uniqueApps := make(map[string]struct{})
		for _, reg := range all {
			app := reg.GetApplication()
			id := strings.TrimSpace(app.ID)
			if id != "" {
				uniqueApps[id] = struct{}{}
			}
		}
		if len(uniqueApps) > 1 {
			return nil, &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"multiple applications match name %q; please specify --%s",
					filters.ApplicationName,
					applicationIDFlagName,
				),
			}
		}
	}

	if filters.ApplicationID != "" {
		filtered := make([]kkComps.ApplicationRegistration, 0, len(all))
		for _, reg := range all {
			app := reg.GetApplication()
			if strings.EqualFold(app.ID, filters.ApplicationID) {
				filtered = append(filtered, reg)
			}
		}
		return filtered, nil
	}

	return all, nil
}

func buildRegistrationFilter(filters registrationFilters) *kkOps.ListRegistrationsQueryParamFilter {
	var out kkOps.ListRegistrationsQueryParamFilter
	var hasFilter bool

	if filters.ApplicationName != "" {
		out.ApplicationName = &kkComps.StringFieldFilter{Eq: kk.String(filters.ApplicationName)}
		hasFilter = true
	}

	if filters.DeveloperID != "" {
		out.DeveloperID = &kkComps.UUIDFieldFilter{Eq: kk.String(filters.DeveloperID)}
		hasFilter = true
	}

	if filters.Status != "" {
		out.Status = &kkComps.StringFieldFilter{Eq: kk.String(filters.Status)}
		hasFilter = true
	}

	if !hasFilter {
		return nil
	}

	return &out
}

func findRegistrationByID(regs []kkComps.ApplicationRegistration, identifier string) *kkComps.ApplicationRegistration {
	if identifier == "" {
		return nil
	}

	lowered := strings.ToLower(strings.TrimSpace(identifier))
	for i := range regs {
		if strings.ToLower(regs[i].GetID()) == lowered {
			return &regs[i]
		}
	}

	return nil
}

func portalApplicationRegistrationSummaryToRecord(
	reg kkComps.ApplicationRegistration,
) portalApplicationRegistrationSummaryRecord {
	createdAt := reg.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := reg.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05")

	app := reg.GetApplication()
	api := reg.GetAPI()

	application := fmt.Sprintf("%s (%s)", app.GetName(), util.AbbreviateUUID(app.GetID()))
	apiLabel := api.GetName()
	if version := api.GetVersion(); version != nil && strings.TrimSpace(*version) != "" {
		apiLabel = fmt.Sprintf("%s (%s)", apiLabel, strings.TrimSpace(*version))
	}

	return portalApplicationRegistrationSummaryRecord{
		ID:               util.AbbreviateUUID(reg.GetID()),
		Status:           string(reg.GetStatus()),
		Application:      application,
		API:              apiLabel,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

func portalApplicationRegistrationDetailRecordFromResponse(
	res *kkComps.GetApplicationRegistrationResponse,
) portalApplicationRegistrationDetailRecord {
	if res == nil {
		return portalApplicationRegistrationDetailRecord{}
	}

	api := res.GetAPI()
	app := res.GetApplication()

	apiLabel := api.GetName()
	if version := api.GetVersion(); version != nil && strings.TrimSpace(*version) != "" {
		apiLabel = fmt.Sprintf("%s (%s)", apiLabel, strings.TrimSpace(*version))
	}

	return portalApplicationRegistrationDetailRecord{
		ID:               util.AbbreviateUUID(res.GetID()),
		Status:           string(res.GetStatus()),
		Application:      app.GetName(),
		ApplicationID:    app.GetID(),
		API:              apiLabel,
		APIEntityType:    string(api.GetEntityType()),
		LocalCreatedTime: res.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: res.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func portalApplicationRegistrationDetailView(reg *kkComps.ApplicationRegistration) string {
	if reg == nil {
		return ""
	}

	api := reg.GetAPI()
	app := reg.GetApplication()

	b := &strings.Builder{}
	fmt.Fprintf(b, "ID: %s\n", util.AbbreviateUUID(reg.GetID()))
	fmt.Fprintf(b, "Status: %s\n", reg.GetStatus())
	fmt.Fprintf(b, "Application: %s (%s)\n", app.GetName(), app.GetID())
	fmt.Fprintf(b, "API: %s", api.GetName())
	if version := api.GetVersion(); version != nil && strings.TrimSpace(*version) != "" {
		fmt.Fprintf(b, " (%s)", strings.TrimSpace(*version))
	}
	fmt.Fprintf(b, "\nEntity Type: %s\n", api.GetEntityType())
	fmt.Fprintf(b, "Created: %s\n", reg.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05"))
	fmt.Fprintf(b, "Updated: %s\n", reg.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05"))

	return b.String()
}
