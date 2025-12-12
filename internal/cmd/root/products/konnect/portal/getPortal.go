package portal

import (
	"fmt"
	"sort"
	"strings"
	"time"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/navigator"
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
	getPortalsShort = i18n.T("root.products.konnect.portal.getPortalsShort",
		"List or get Konnect portals")
	getPortalsLong = i18n.T("root.products.konnect.portal.getPortalsLong",
		`Use the get verb with the portal command to query Konnect portals.`)
	getPortalsExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.getPortalExamples",
			fmt.Sprintf(`
	# List all the portals for the organization
	%[1]s get portals
	# Get details for a portal with a specific ID 
	%[1]s get portal 22cd8a0b-72e7-4212-9099-0764f8e9c5ac
	# Get details for a portal with a specific name
	%[1]s get portal my-portal 
	# Get all the portals using command aliases
	%[1]s get ps
	`, meta.CLIName)))
)

// Represents a text display record for a Portal
type textDisplayRecord struct {
	ID               string
	Name             string
	Description      string
	CustomDomain     string
	LocalCreatedTime string
	LocalUpdatedTime string
}

func portalToDisplayRecord(p *kkComps.ListPortalsResponsePortal) textDisplayRecord {
	missing := "n/a"

	var id, name string
	if p.GetID() != "" {
		id = util.AbbreviateUUID(p.GetID())
	} else {
		id = missing
	}

	if p.GetName() != "" {
		name = p.GetName()
	} else {
		name = missing
	}

	description := missing
	if desc := p.GetDescription(); desc != nil && *desc != "" {
		description = *desc
	}

	// CustomDomain field doesn't exist in current SDK
	customDomain := missing

	createdAt := p.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := p.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05")

	return textDisplayRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		CustomDomain:     customDomain,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

func portalResponseToDisplayRecord(p *kkComps.PortalResponse) textDisplayRecord {
	missing := "n/a"

	var id, name string
	if p.ID != "" {
		id = util.AbbreviateUUID(p.ID)
	} else {
		id = missing
	}

	if p.Name != "" {
		name = p.Name
	} else {
		name = missing
	}

	description := missing
	if p.Description != nil && *p.Description != "" {
		description = *p.Description
	}

	// Use CanonicalDomain from PortalResponse
	customDomain := missing
	if p.CanonicalDomain != "" {
		customDomain = p.CanonicalDomain
	}

	createdAt := p.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := p.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return textDisplayRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		CustomDomain:     customDomain,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

type portalDetailData struct {
	ID              string
	Name            string
	Description     *string
	CanonicalDomain string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Labels          map[string]string
}

func renderPortalDetail(data portalDetailData) string {
	const missing = "n/a"

	id := strings.TrimSpace(data.ID)
	if id == "" {
		id = missing
	}

	name := strings.TrimSpace(data.Name)
	if name == "" {
		name = missing
	}

	description := ""
	if data.Description != nil {
		description = strings.TrimSpace(*data.Description)
	}

	canonicalDomain := strings.TrimSpace(data.CanonicalDomain)
	if canonicalDomain == "" {
		canonicalDomain = missing
	}

	fields := map[string]string{
		"canonical_domain": canonicalDomain,
		"created_at":       data.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
		"updated_at":       data.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
	}

	switch {
	case data.Labels == nil:
		fields["labels"] = ""
	case len(data.Labels) == 0:
		fields["labels"] = ""
	default:
		fields["labels"] = fmt.Sprintf("%d label(s)", len(data.Labels))
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	for _, key := range keys {
		fmt.Fprintf(&b, "%s: %s\n", key, fields[key])
	}

	if description != "" {
		fmt.Fprintf(&b, "description:\n%s\n", description)
	}

	return strings.TrimRight(b.String(), "\n")
}

func portalListDetailView(p *kkComps.ListPortalsResponsePortal) string {
	if p == nil {
		return ""
	}

	data := portalDetailData{
		ID:              p.GetID(),
		Name:            p.GetName(),
		Description:     p.GetDescription(),
		CanonicalDomain: p.GetCanonicalDomain(),
		CreatedAt:       p.GetCreatedAt(),
		UpdatedAt:       p.GetUpdatedAt(),
		Labels:          p.GetLabels(),
	}
	return renderPortalDetail(data)
}

func portalResponseDetailView(p *kkComps.PortalResponse) string {
	if p == nil {
		return ""
	}

	data := portalDetailData{
		ID:              p.ID,
		Name:            p.Name,
		Description:     p.Description,
		CanonicalDomain: p.CanonicalDomain,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
		Labels:          p.Labels,
	}
	return renderPortalDetail(data)
}

type getPortalCmd struct {
	*cobra.Command
}

func runListByName(name string, kkClient helpers.PortalAPI, helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.ListPortalsResponsePortal, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.ListPortalsResponsePortal

	for {
		req := kkOps.ListPortalsRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := kkClient.ListPortals(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list Portals", err, helper.GetCmd(), attrs...)
		}

		// Filter by name since SDK doesn't support name filtering for portals
		for _, portal := range res.GetListPortalsResponse().Data {
			if portal.Name == name {
				return &portal, nil
			}
		}

		allData = append(allData, res.GetListPortalsResponse().Data...)
		totalItems := res.GetListPortalsResponse().Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return nil, fmt.Errorf("portal with name %s not found", name)
}

func runList(kkClient helpers.PortalAPI, helper cmd.Helper,
	cfg config.Hook,
) ([]kkComps.ListPortalsResponsePortal, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.ListPortalsResponsePortal

	for {
		req := kkOps.ListPortalsRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := kkClient.ListPortals(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list Portals", err, helper.GetCmd(), attrs...)
		}

		allData = append(allData, res.GetListPortalsResponse().Data...)
		totalItems := res.GetListPortalsResponse().Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func runGet(id string, kkClient helpers.PortalAPI, helper cmd.Helper,
) (*kkComps.PortalResponse, error) {
	res, err := kkClient.GetPortal(helper.GetContext(), id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get Portal", err, helper.GetCmd(), attrs...)
	}

	return res.GetPortalResponse(), nil
}

func (c *getPortalCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing portals requires 0 or 1 arguments (name or ID)"),
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

func (c *getPortalCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if err := c.validate(helper); err != nil {
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

	interactive, err := helper.IsInteractive()
	if err != nil {
		return err
	}

	var printer cli.PrintFlusher
	if !interactive {
		printer, err = cli.Format(outType.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	// 'get portals' can be run in various ways:
	//	> get portals <id>    # Get by UUID
	//  > get portals <name>	# Get by name
	//  > get portals         # List all
	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		id := strings.TrimSpace(helper.GetArgs()[0])

		isUUID := util.IsValidUUID(id)

		if !isUUID {
			// If the ID is not a UUID, then it is a name
			// search for the portal by name
			portal, err := runListByName(id, sdk.GetPortalAPI(), helper, cfg)
			if err != nil {
				return err
			}

			detailFn := func(index int) string {
				if index != 0 {
					return ""
				}
				return portalListDetailView(portal)
			}
			return tableview.RenderForFormat(
				interactive,
				outType,
				printer,
				helper.GetStreams(),
				portalToDisplayRecord(portal),
				portal,
				"",
				tableview.WithRootLabel(helper.GetCmd().Name()),
				tableview.WithDetailRenderer(detailFn),
				tableview.WithDetailHelper(helper),
				tableview.WithDetailContext("portal", func(index int) any {
					if index != 0 {
						return nil
					}
					return portal
				}),
			)
		}
		portalResponse, err := runGet(id, sdk.GetPortalAPI(), helper)
		if err != nil {
			return err
		}
		detailFn := func(index int) string {
			if index != 0 {
				return ""
			}
			return portalResponseDetailView(portalResponse)
		}
		return tableview.RenderForFormat(
			interactive,
			outType,
			printer,
			helper.GetStreams(),
			portalResponseToDisplayRecord(portalResponse),
			portalResponse,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
			tableview.WithDetailRenderer(detailFn),
			tableview.WithDetailHelper(helper),
			tableview.WithDetailContext("portal", func(index int) any {
				if index != 0 {
					return nil
				}
				return portalResponse
			}),
		)
	}

	if interactive {
		return navigator.Run(helper, navigator.Options{InitialResource: "portals"})
	}

	portals, err := runList(sdk.GetPortalAPI(), helper, cfg)
	if err != nil {
		return err
	}

	return renderPortalList(helper, helper.GetCmd().Name(), interactive, outType, printer, portals)
}

func renderPortalList(
	helper cmd.Helper,
	rootLabel string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	portals []kkComps.ListPortalsResponsePortal,
) error {
	displayRecords := make([]textDisplayRecord, 0, len(portals))
	for i := range portals {
		displayRecords = append(displayRecords, portalToDisplayRecord(&portals[i]))
	}

	childView := buildPortalChildView(portals)

	options := []tableview.Option{
		tableview.WithCustomTable(childView.Headers, childView.Rows),
		tableview.WithRootLabel(rootLabel),
		tableview.WithDetailHelper(helper),
	}
	if childView.DetailRenderer != nil {
		options = append(options, tableview.WithDetailRenderer(childView.DetailRenderer))
	}
	if childView.DetailContext != nil {
		options = append(options, tableview.WithDetailContext(childView.ParentType, childView.DetailContext))
	}

	return tableview.RenderForFormat(
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		displayRecords,
		portals,
		"",
		options...,
	)
}

func buildPortalChildView(portals []kkComps.ListPortalsResponsePortal) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(portals))
	for i := range portals {
		record := portalToDisplayRecord(&portals[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(portals) {
			return ""
		}
		return portalListDetailView(&portals[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Portals",
		ParentType:     "portal",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(portals) {
				return nil
			}
			return &portals[index]
		},
	}
}

func newGetPortalCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getPortalCmd {
	rv := getPortalCmd{
		Command: baseCmd,
	}

	rv.Short = getPortalsShort
	rv.Long = getPortalsLong
	rv.Example = getPortalsExample
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	if pagesCmd := newGetPortalPagesCmd(verb, addParentFlags, parentPreRun); pagesCmd != nil {
		rv.AddCommand(pagesCmd)
	}

	if snippetsCmd := newGetPortalSnippetsCmd(verb, addParentFlags, parentPreRun); snippetsCmd != nil {
		rv.AddCommand(snippetsCmd)
	}

	if applicationsCmd := newGetPortalApplicationsCmd(verb, addParentFlags, parentPreRun); applicationsCmd != nil {
		rv.AddCommand(applicationsCmd)
	}

	if registrationsCmd := newGetPortalApplicationRegistrationsCmd(
		verb,
		addParentFlags,
		parentPreRun,
	); registrationsCmd != nil {
		rv.AddCommand(registrationsCmd)
	}

	if teamRolesCmd := newGetPortalTeamRolesCmd(verb, addParentFlags, parentPreRun); teamRolesCmd != nil {
		rv.AddCommand(teamRolesCmd)
	}

	if teamsCmd := newGetPortalTeamsCmd(verb, addParentFlags, parentPreRun); teamsCmd != nil {
		rv.AddCommand(teamsCmd)
	}

	if developersCmd := newGetPortalDevelopersCmd(verb, addParentFlags, parentPreRun); developersCmd != nil {
		rv.AddCommand(developersCmd)
	}

	if authSettingsCmd := newGetPortalAuthSettingsCmd(verb, addParentFlags, parentPreRun); authSettingsCmd != nil {
		rv.AddCommand(authSettingsCmd)
	}

	if assetsCmd := newGetPortalAssetsCmd(verb, addParentFlags, parentPreRun); assetsCmd != nil {
		rv.AddCommand(assetsCmd)
	}

	if emailDomainsCmd := newGetPortalEmailDomainsCmd(verb, addParentFlags, parentPreRun); emailDomainsCmd != nil {
		rv.AddCommand(emailDomainsCmd)
	}

	return &rv
}
