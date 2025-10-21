package controlplane

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
	getControlPlanesShort = i18n.T("root.products.konnect.gateway.controlplane.getControlPlanesShort",
		"List or get Konnect Kong Gateway control planes")
	getControlPlanesLong = i18n.T("root.products.konnect.gateway.controlplane.getControlPlanesLong",
		`Use the get verb with the control-plane command to query Konnect Kong Gateway control planes.`)
	getControlPlanesExample = normalizers.Examples(
		i18n.T("root.products.konnect.gateway.gateway.controlplane.getControlPlaneExamples",
			fmt.Sprintf(`
	# List all the control planes for the authorized user
	%[1]s get konnect gateway control-planes
	# Get details for a control plane with a specific ID 
	%[1]s get konnect gateway control-plane 22cd8a0b-72e7-4212-9099-0764f8e9c5ac
	# Get details for a control plane with a specific name
	%[1]s get konnect gateway control-plane my-control-plane 
	# Get all the control planes for the authorized user using command aliases
	%[1]s get k gw cps
	`, meta.CLIName)))
)

// Represents a text display record for a Control Plane
//
// Because the SDK provides pointers for optional value fields,
// the segmentio/cli printer prints the address instead of the value.
// This will require a decent amount of boilerplate code to convert
// the types to a format that prints how we want.
type textDisplayRecord struct {
	Name                 string
	Description          string
	Labels               string
	ControlPlaneEndpoint string
	Config               string
	LocalCreatedTime     string
	LocalUpdatedTime     string
	ID                   string
}

func controlPlaneToDisplayRecord(c *kkComps.ControlPlane) textDisplayRecord {
	missing := "n/a"

	var id, name string
	if c.ID != "" {
		id = util.AbbreviateUUID(c.ID)
	} else {
		id = missing
	}

	if c.Name != "" {
		name = c.Name
	} else {
		name = missing
	}

	description := missing
	if c.Description != nil {
		description = *c.Description
	}

	labels := missing
	if len(c.Labels) > 0 {
		// 1) pull out the keys…
		keys := make([]string, 0, len(c.Labels))
		for k := range c.Labels {
			keys = append(keys, k)
		}
		// 2) sort them lexicographically
		sort.Strings(keys)
		// 3) build your pairs in that order
		labelPairs := make([]string, 0, len(c.Labels))
		for _, k := range keys {
			labelPairs = append(labelPairs, fmt.Sprintf("%s: %s", k, c.Labels[k]))
		}
		labels = strings.Join(labelPairs, ", ")
	}

	controlPlaneEndpoint := missing
	if c.Config.ControlPlaneEndpoint != "" {
		controlPlaneEndpoint = c.Config.ControlPlaneEndpoint
	}

	config := "[...]"

	createdAt := c.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := c.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return textDisplayRecord{
		ID:                   id,
		Name:                 name,
		Description:          description,
		Labels:               labels,
		ControlPlaneEndpoint: controlPlaneEndpoint,
		Config:               config,
		LocalCreatedTime:     createdAt,
		LocalUpdatedTime:     updatedAt,
	}
}

func controlPlaneDetailView(cp *kkComps.ControlPlane) string {
	if cp == nil {
		return ""
	}

	missing := "n/a"

	id := strings.TrimSpace(cp.ID)
	if id == "" {
		id = missing
	}

	name := strings.TrimSpace(cp.Name)
	if name == "" {
		name = missing
	}

	description := missing
	if cp.Description != nil && strings.TrimSpace(*cp.Description) != "" {
		description = strings.TrimSpace(*cp.Description)
	}

	labelsValue := missing
	if len(cp.Labels) > 0 {
		pairs := make([]string, 0, len(cp.Labels))
		for k, v := range cp.Labels {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(pairs)
		labelsValue = strings.Join(pairs, ", ")
	}

	endpoint := missing
	if strings.TrimSpace(cp.Config.ControlPlaneEndpoint) != "" {
		endpoint = strings.TrimSpace(cp.Config.ControlPlaneEndpoint)
	}

	created := cp.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updated := cp.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", description)
	fmt.Fprintf(&b, "labels: %s\n", labelsValue)
	fmt.Fprintf(&b, "control_plane_endpoint: %s\n", endpoint)
	fmt.Fprintf(&b, "config: [...]\n")
	fmt.Fprintf(&b, "created_at: %s\n", created)
	fmt.Fprintf(&b, "updated_at: %s\n", updated)

	return strings.TrimRight(b.String(), "\n")
}

type getControlPlaneCmd struct {
	*cobra.Command
}

func runListByName(name string, kkClient helpers.ControlPlaneAPI, helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.ControlPlane, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.ControlPlane

	for {
		req := kkOps.ListControlPlanesRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
			Filter: &kkComps.ControlPlaneFilterParameters{
				Name: &kkComps.Name{
					Eq: kk.String(name),
				},
			},
		}

		res, err := kkClient.ListControlPlanes(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list Control Planes", err, helper.GetCmd(), attrs...)
		}

		allData = append(allData, res.GetListControlPlanesResponse().Data...)
		totalItems := res.GetListControlPlanesResponse().Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	// Making the determination to always take the first element in a list of return values.
	//    It's possible this logic is flawed ?
	if len(allData) > 0 {
		return &allData[0], nil
	}
	return nil, fmt.Errorf("control plane with name %s not found", name)
}

func runList(kkClient helpers.ControlPlaneAPI, helper cmd.Helper,
	cfg config.Hook,
) ([]kkComps.ControlPlane, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.ControlPlane

	for {
		req := kkOps.ListControlPlanesRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := kkClient.ListControlPlanes(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list Control Planes", err, helper.GetCmd(), attrs...)
		}

		allData = append(allData, res.GetListControlPlanesResponse().Data...)
		totalItems := res.GetListControlPlanesResponse().Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func runGet(id string, kkClient helpers.ControlPlaneAPI, helper cmd.Helper,
) (*kkComps.ControlPlane, error) {
	res, err := kkClient.GetControlPlane(helper.GetContext(), id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get Control Plane", err, helper.GetCmd(), attrs...)
	}

	return res.GetControlPlane(), nil
}

func (c *getControlPlaneCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing control planes requires 0 or 1 arguments (name or ID)"),
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

func (c *getControlPlaneCmd) runE(cobraCmd *cobra.Command, args []string) error {
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

	// 'get konnect gateway cps' can be run like various ways:
	//	> get konnect gateway cps <id>    # Get by UUID
	//  > get konnect gateway cps <name>	# Get by name
	//  > get konnect gateway cps					# List all
	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		id := helper.GetArgs()[0]

		isUUID := util.IsValidUUID(id)
		// TODO: Is capturing that blanked error necessary?

		var cp *kkComps.ControlPlane
		if !isUUID {
			// If the ID is not a UUID, then it is a name
			// search for the control plane by name
			cp, err = runListByName(id, sdk.GetControlPlaneAPI(), helper, cfg)
		} else {
			cp, err = runGet(id, sdk.GetControlPlaneAPI(), helper)
		}
		if err != nil {
			return err
		}
		return tableview.RenderForFormat(
			interactive,
			outType,
			printer,
			helper.GetStreams(),
			controlPlaneToDisplayRecord(cp),
			cp,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
			tableview.WithDetailContext("control-plane", func(int) any {
				return cp
			}),
			tableview.WithDetailHelper(helper),
		)
	}

	if interactive {
		return navigator.Run(helper, navigator.Options{InitialResource: "gateway control-planes"})
	}

	// list all control planes
	cps, err := runList(sdk.GetControlPlaneAPI(), helper, cfg)
	if err != nil {
		return err
	}

	return renderControlPlaneList(helper, helper.GetCmd().Name(), interactive, outType, printer, cps)
}

func renderControlPlaneList(
	helper cmd.Helper,
	rootLabel string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cps []kkComps.ControlPlane,
) error {
	displayRecords := make([]textDisplayRecord, 0, len(cps))
	for i := range cps {
		displayRecords = append(displayRecords, controlPlaneToDisplayRecord(&cps[i]))
	}

	childView := buildControlPlaneChildView(cps)

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
		cps,
		"",
		options...,
	)
}

func buildControlPlaneChildView(cps []kkComps.ControlPlane) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(cps))
	for i := range cps {
		record := controlPlaneToDisplayRecord(&cps[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(cps) {
			return ""
		}
		return controlPlaneDetailView(&cps[index])
	}

	return tableview.ChildView{
		Headers:        []string{"id", "name"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Control Planes",
		ParentType:     "control-plane",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(cps) {
				return nil
			}
			return &cps[index]
		},
	}
}

func newGetControlPlaneCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getControlPlaneCmd {
	rv := getControlPlaneCmd{
		Command: baseCmd,
	}

	rv.Short = getControlPlanesShort
	rv.Long = getControlPlanesLong
	rv.Example = getControlPlanesExample
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	return &rv
}
