package analytics

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
	getAnalyticsDashboardsShort = i18n.T("root.products.konnect.analytics.getDashboardsShort",
		"List or get Konnect Analytics dashboards")
	getAnalyticsDashboardsLong = i18n.T("root.products.konnect.analytics.getDashboardsLong",
		`Use the get verb with the analytics dashboard command to query Konnect Analytics dashboards.`)
	getAnalyticsDashboardsExample = normalizers.Examples(
		i18n.T("root.products.konnect.analytics.getDashboardsExamples",
			fmt.Sprintf(`
	# List all analytics dashboards
	%[1]s get analytics dashboards
	# Get details for an analytics dashboard by name
	%[1]s get analytics dashboard "API Summary"
	# List analytics dashboards using command aliases
	%[1]s get analytic dashboards
	`, meta.CLIName)))
)

type dashboardDisplayRecord struct {
	ID                string
	Name              string
	TileCount         string
	PresetFilterCount string
	LabelCount        string
	LocalCreatedTime  string
	LocalUpdatedTime  string
}

type getAnalyticsDashboardsCmd struct {
	*cobra.Command
}

func dashboardToDisplayRecord(dashboard kkComps.DashboardResponse) dashboardDisplayRecord {
	const missing = "n/a"

	record := dashboardDisplayRecord{
		ID:                missing,
		Name:              missing,
		TileCount:         fmt.Sprintf("%d", len(dashboard.Definition.Tiles)),
		PresetFilterCount: fmt.Sprintf("%d", len(dashboard.Definition.PresetFilters)),
		LabelCount:        fmt.Sprintf("%d", len(dashboard.Labels)),
		LocalCreatedTime:  missing,
		LocalUpdatedTime:  missing,
	}

	if dashboard.ID != nil && strings.TrimSpace(*dashboard.ID) != "" {
		record.ID = util.AbbreviateUUID(*dashboard.ID)
	}
	if strings.TrimSpace(dashboard.Name) != "" {
		record.Name = dashboard.Name
	}
	if dashboard.CreatedAt != nil && !dashboard.CreatedAt.IsZero() {
		record.LocalCreatedTime = dashboard.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	if dashboard.UpdatedAt != nil && !dashboard.UpdatedAt.IsZero() {
		record.LocalUpdatedTime = dashboard.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	return record
}

func dashboardDetailView(dashboard kkComps.DashboardResponse) string {
	const missing = "n/a"

	valueOrMissing := func(value string) string {
		value = strings.TrimSpace(value)
		if value == "" {
			return missing
		}
		return value
	}

	id := missing
	if dashboard.ID != nil {
		id = valueOrMissing(*dashboard.ID)
	}

	createdAt := missing
	if dashboard.CreatedAt != nil && !dashboard.CreatedAt.IsZero() {
		createdAt = dashboard.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	updatedAt := missing
	if dashboard.UpdatedAt != nil && !dashboard.UpdatedAt.IsZero() {
		updatedAt = dashboard.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", valueOrMissing(dashboard.Name))
	fmt.Fprintf(&b, "tiles: %d\n", len(dashboard.Definition.Tiles))
	fmt.Fprintf(&b, "preset_filters: %d\n", len(dashboard.Definition.PresetFilters))
	fmt.Fprintf(&b, "labels: %s\n", summarizeDashboardLabels(dashboard.Labels, missing))
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)

	return strings.TrimRight(b.String(), "\n")
}

func summarizeDashboardLabels(labels map[string]string, missing string) string {
	switch {
	case labels == nil:
		return missing
	case len(labels) == 0:
		return "[]"
	default:
		keys := slices.Sorted(maps.Keys(labels))
		pairs := make([]string, 0, len(labels))
		for _, key := range keys {
			pairs = append(pairs, fmt.Sprintf("%s=%s", key, labels[key]))
		}
		return strings.Join(pairs, ", ")
	}
}

func runDashboardList(
	kkClient helpers.DashboardsAPI,
	helper cmd.Helper,
	cfg config.Hook,
) ([]kkComps.DashboardResponse, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.DashboardResponse
	for {
		req := kkOps.DashboardsListRequest{
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
		}

		res, err := kkClient.DashboardsList(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list analytics dashboards", err, helper.GetCmd(), attrs...)
		}
		if res == nil || res.Object == nil {
			return allData, nil
		}

		pageData := res.Object.Data
		allData = append(allData, pageData...)

		totalItems := 0
		if res.Object.Meta != nil {
			totalItems = int(res.Object.Meta.Page.Total)
		}
		if totalItems > 0 {
			if len(allData) >= totalItems {
				break
			}
		} else if len(pageData) < int(requestPageSize) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func runDashboardGetByName(
	name string,
	kkClient helpers.DashboardsAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.DashboardResponse, error) {
	name = strings.TrimSpace(name)
	dashboards, err := runDashboardList(kkClient, helper, cfg)
	if err != nil {
		return nil, err
	}

	var matches []kkComps.DashboardResponse
	for _, dashboard := range dashboards {
		if dashboard.Name == name {
			matches = append(matches, dashboard)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("analytics dashboard with name %q not found", name)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("analytics dashboard name %q matches %d dashboards", name, len(matches))
	}
}

func (c *getAnalyticsDashboardsCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing analytics dashboards requires 0 or 1 arguments (name)"),
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

func (c *getAnalyticsDashboardsCmd) runE(cobraCmd *cobra.Command, args []string) error {
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

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	if len(helper.GetArgs()) == 1 {
		dashboard, err := runDashboardGetByName(helper.GetArgs()[0], sdk.GetDashboardsAPI(), helper, cfg)
		if err != nil {
			return err
		}

		detailFn := func(index int) string {
			if index != 0 {
				return ""
			}
			return dashboardDetailView(*dashboard)
		}

		return tableview.RenderForFormat(
			helper,
			false,
			outType,
			printer,
			helper.GetStreams(),
			dashboardToDisplayRecord(*dashboard),
			dashboard,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
			tableview.WithDetailRenderer(detailFn),
			tableview.WithDetailHelper(helper),
			tableview.WithDetailContext(common.ViewParentAnalyticsDashboard, func(index int) any {
				if index != 0 {
					return nil
				}
				return dashboard
			}),
		)
	}

	dashboards, err := runDashboardList(sdk.GetDashboardsAPI(), helper, cfg)
	if err != nil {
		return err
	}

	return renderDashboardList(helper, helper.GetCmd().Name(), outType, printer, dashboards)
}

func renderDashboardList(
	helper cmd.Helper,
	rootLabel string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	dashboards []kkComps.DashboardResponse,
) error {
	displayRecords := make([]dashboardDisplayRecord, 0, len(dashboards))
	for _, dashboard := range dashboards {
		displayRecords = append(displayRecords, dashboardToDisplayRecord(dashboard))
	}

	childView := buildDashboardChildView(dashboards)
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
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		displayRecords,
		dashboards,
		"",
		options...,
	)
}

func buildDashboardChildView(dashboards []kkComps.DashboardResponse) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(dashboards))
	for _, dashboard := range dashboards {
		record := dashboardToDisplayRecord(dashboard)
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(dashboards) {
			return ""
		}
		return dashboardDetailView(dashboards[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Analytics Dashboards",
		ParentType:     common.ViewParentAnalyticsDashboard,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(dashboards) {
				return nil
			}
			return &dashboards[index]
		},
	}
}

func newGetAnalyticsDashboardsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getAnalyticsDashboardsCmd {
	rv := getAnalyticsDashboardsCmd{
		Command: &cobra.Command{
			Use:     "dashboard [name]",
			Short:   getAnalyticsDashboardsShort,
			Long:    getAnalyticsDashboardsLong,
			Example: getAnalyticsDashboardsExample,
			Aliases: []string{"dashboards"},
		},
	}

	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	return &rv
}
