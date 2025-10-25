package api

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
	"github.com/muesli/reflow/wordwrap"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

var (
	getAPIsShort = i18n.T("root.products.konnect.api.getAPIsShort",
		"List or get Konnect APIs")
	getAPIsLong = i18n.T("root.products.konnect.api.getAPIsLong",
		`Use the get verb with the api command to query Konnect APIs.`)
	getAPIsExample = normalizers.Examples(
		i18n.T("root.products.konnect.api.getAPIExamples",
			fmt.Sprintf(`
	# List all the APIs for the organization
	%[1]s get apis
	# Get details for an API with a specific ID 
	%[1]s get api 22cd8a0b-72e7-4212-9099-0764f8e9c5ac
	# Get details for an API with a specific name
	%[1]s get api my-api
	# Get all the APIs using command aliases
	%[1]s get apis
	`, meta.CLIName)))
)

// Represents a text display record for an API
type textDisplayRecord struct {
	ID               string
	Name             string
	Description      string
	VersionCount     string
	PublicationCount string
	LocalCreatedTime string
	LocalUpdatedTime string
}

func apiToDisplayRecord(a *kkComps.APIResponseSchema) textDisplayRecord {
	missing := "n/a"

	var id, name string
	if a.ID != "" {
		id = util.AbbreviateUUID(a.ID)
	} else {
		id = missing
	}

	if apiName := util.StringValue(a.Name); apiName != "" {
		name = apiName
	} else {
		name = missing
	}

	description := missing
	if a.Description != nil && *a.Description != "" {
		description = *a.Description
	}

	versionCount := missing
	// Version count is not directly available in APIResponseSchema
	// It would require a separate API call to list versions

	publicationCount := missing
	if a.Portals != nil {
		publicationCount = fmt.Sprintf("%d", len(a.Portals))
	}

	createdAt := a.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := a.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return textDisplayRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		VersionCount:     versionCount,
		PublicationCount: publicationCount,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

func apiDetailView(api *kkComps.APIResponseSchema) string {
	if api == nil {
		return ""
	}

	const missing = "n/a"
	id := strings.TrimSpace(api.ID)
	if id == "" {
		id = missing
	}
	name := strings.TrimSpace(util.StringValue(api.Name))
	if name == "" {
		name = missing
	}

	type detailField struct {
		label     string
		value     string
		multiline bool
	}

	var fields []detailField

	addField := func(label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		fields = append(fields, detailField{
			label: label,
			value: value,
		})
	}

	addMultiline := func(label, value string) {
		value = strings.TrimRight(value, "\n")
		if strings.TrimSpace(value) == "" {
			return
		}
		fields = append(fields, detailField{
			label:     label,
			value:     value,
			multiline: true,
		})
	}

	if slugPtr := api.GetSlug(); slugPtr != nil {
		if slug := strings.TrimSpace(*slugPtr); slug != "" {
			addField("slug", slug)
		}
	}

	if versionPtr := api.GetVersion(); versionPtr != nil {
		if version := strings.TrimSpace(*versionPtr); version != "" {
			addField("version", version)
		}
	}

	if api.CurrentVersionSummary != nil {
		if spec := api.CurrentVersionSummary.Spec; spec != nil {
			if spec.Type != nil {
				if specType := strings.TrimSpace(string(*spec.Type)); specType != "" {
					addField("spec_type", specType)
				}
			}
		}
	}

	if specIDs := api.GetAPISpecIds(); len(specIDs) > 0 {
		ids := make([]string, 0, len(specIDs))
		for _, specID := range specIDs {
			ids = append(ids, util.AbbreviateUUID(specID))
		}
		addField("spec_ids", strings.Join(ids, ", "))
	}

	if api.Description != nil && *api.Description != "" {
		description := strings.TrimSpace(*api.Description)
		if description != "" {
			const wrapWidth = 80
			addMultiline("description", wordwrap.String(description, wrapWidth))
		}
	}

	if attrs := api.Attributes; attrs != nil {
		switch v := attrs.(type) {
		case map[string]any:
			if len(v) > 0 {
				keys := make([]string, 0, len(v))
				for k := range v {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				var sb strings.Builder
				for _, k := range keys {
					fmt.Fprintf(&sb, "  %s: %v\n", k, v[k])
				}
				addMultiline("attributes", sb.String())
			}
		case map[string]string:
			if len(v) > 0 {
				keys := make([]string, 0, len(v))
				for k := range v {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				var sb strings.Builder
				for _, k := range keys {
					fmt.Fprintf(&sb, "  %s: %s\n", k, v[k])
				}
				addMultiline("attributes", sb.String())
			}
		}
	}

	if portals := api.GetPortals(); len(portals) > 0 {
		var sb strings.Builder
		for _, portal := range portals {
			displayName := strings.TrimSpace(portal.DisplayName)
			portalName := strings.TrimSpace(portal.Name)
			var line string
			switch {
			case displayName != "" && portalName != "":
				line = fmt.Sprintf("%s (%s)", displayName, portalName)
			case displayName != "":
				line = displayName
			case portalName != "":
				line = portalName
			default:
				line = missing
			}
			fmt.Fprintf(&sb, "  %s - %s\n", line, portal.ID)
		}
		addMultiline("portals", sb.String())
	}

	if labels := api.GetLabels(); len(labels) > 0 {
		keys := make([]string, 0, len(labels))
		for k := range labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var sb strings.Builder
		for _, k := range keys {
			fmt.Fprintf(&sb, "  %s: %s\n", k, labels[k])
		}
		addMultiline("labels", sb.String())
	}

	addField("publication_count", fmt.Sprintf("%d", len(api.GetPortals())))
	addField("created_at", api.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05"))
	addField("updated_at", api.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05"))

	sort.Slice(fields, func(i, j int) bool {
		li := strings.ToLower(fields[i].label)
		lj := strings.ToLower(fields[j].label)
		if li == lj {
			return fields[i].label < fields[j].label
		}
		return li < lj
	})

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	for _, field := range fields {
		if field.multiline {
			value := strings.TrimRight(field.value, "\n")
			fmt.Fprintf(&b, "%s:\n%s\n", field.label, value)
			continue
		}
		fmt.Fprintf(&b, "%s: %s\n", field.label, field.value)
	}

	return b.String()
}

type getAPICmd struct {
	*cobra.Command
}

func runListByName(name string, kkClient helpers.APIAPI, helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.APIResponseSchema, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.APIResponseSchema

	for {
		req := kkOps.ListApisRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		// Note: The SDK's ListApisRequest doesn't support include parameter
		// Version and publication information would require separate API calls

		res, err := kkClient.ListApis(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list APIs", err, helper.GetCmd(), attrs...)
		}

		// Filter by name since SDK doesn't support name filtering for APIs
		for _, api := range res.ListAPIResponse.Data {
			if util.StringValue(api.Name) == name {
				return &api, nil
			}
		}

		allData = append(allData, res.ListAPIResponse.Data...)
		totalItems := res.ListAPIResponse.Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return nil, cmd.PrepareExecutionErrorMsg(helper,
		fmt.Sprintf("API with name %s not found", name))
}

func runList(kkClient helpers.APIAPI, helper cmd.Helper,
	cfg config.Hook,
) ([]kkComps.APIResponseSchema, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.APIResponseSchema

	for {
		req := kkOps.ListApisRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		// Note: The SDK's ListApisRequest doesn't support include parameter
		// Version and publication information would require separate API calls

		res, err := kkClient.ListApis(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list APIs", err, helper.GetCmd(), attrs...)
		}

		allData = append(allData, res.ListAPIResponse.Data...)
		totalItems := res.ListAPIResponse.Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func runGet(id string, kkClient helpers.APIAPI, helper cmd.Helper,
) (*kkComps.APIResponseSchema, error) {
	// Note: FetchAPI doesn't support include parameters
	// Version and publication information would require separate API calls
	res, err := kkClient.FetchAPI(helper.GetContext(), id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get API", err, helper.GetCmd(), attrs...)
	}

	return res.GetAPIResponseSchema(), nil
}

func (c *getAPICmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing APIs requires 0 or 1 arguments (name or ID)"),
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

func (c *getAPICmd) runE(cobraCmd *cobra.Command, args []string) error {
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

	interactive, e := helper.IsInteractive()
	if e != nil {
		return e
	}

	var printer cli.PrintFlusher
	if !interactive {
		printer, e = cli.Format(outType.String(), helper.GetStreams().Out)
		if e != nil {
			return e
		}
		defer printer.Flush()
	}

	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	sdk, e := helper.GetKonnectSDK(cfg, logger)
	if e != nil {
		return e
	}

	// 'get apis' can be run in various ways:
	//	> get apis <id>    # Get by UUID
	//  > get apis <name>	# Get by name
	//  > get apis         # List all
	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		id := strings.TrimSpace(helper.GetArgs()[0])

		isUUID := util.IsValidUUID(id)

		if !isUUID {
			// If the ID is not a UUID, then it is a name
			// search for the API by name
			api, e := runListByName(id, sdk.GetAPIAPI(), helper, cfg)
			if e != nil {
				return e
			}
			return tableview.RenderForFormat(
				interactive,
				outType,
				printer,
				helper.GetStreams(),
				apiToDisplayRecord(api),
				api,
				"",
				tableview.WithRootLabel(helper.GetCmd().Name()),
				tableview.WithDetailHelper(helper),
				tableview.WithDetailContext("api", func(index int) any {
					if index != 0 {
						return nil
					}
					return api
				}),
			)
		}

		api, e := runGet(id, sdk.GetAPIAPI(), helper)
		if e != nil {
			return e
		}

		return tableview.RenderForFormat(
			interactive,
			outType,
			printer,
			helper.GetStreams(),
			apiToDisplayRecord(api),
			api,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
			tableview.WithDetailHelper(helper),
			tableview.WithDetailContext("api", func(index int) any {
				if index != 0 {
					return nil
				}
				return api
			}),
		)
	}

	if interactive {
		return navigator.Run(helper, navigator.Options{InitialResource: "apis"})
	}

	apis, e := runList(sdk.GetAPIAPI(), helper, cfg)
	if e != nil {
		return e
	}

	return renderAPIList(helper, helper.GetCmd().Name(), interactive, outType, printer, apis)
}

func renderAPIList(
	helper cmd.Helper,
	rootLabel string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	apis []kkComps.APIResponseSchema,
) error {
	displayRecords := make([]textDisplayRecord, 0, len(apis))
	for i := range apis {
		displayRecords = append(displayRecords, apiToDisplayRecord(&apis[i]))
	}

	childView := buildAPIChildView(apis)

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
		apis,
		"",
		options...,
	)
}

func buildAPIChildView(apis []kkComps.APIResponseSchema) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(apis))
	for i := range apis {
		record := apiToDisplayRecord(&apis[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(apis) {
			return ""
		}
		return apiDetailView(&apis[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "APIs",
		ParentType:     "api",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(apis) {
				return nil
			}
			return &apis[index]
		},
	}
}

func newGetAPICmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getAPICmd {
	rv := getAPICmd{
		Command: baseCmd,
	}

	rv.Short = getAPIsShort
	rv.Long = getAPIsLong
	rv.Example = getAPIsExample
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	// Ensure parent-level flags are available on this command
	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	if documentsCmd := newGetAPIDocumentsCmd(verb, addParentFlags, parentPreRun); documentsCmd != nil {
		rv.AddCommand(documentsCmd)
	}

	if versionsCmd := newGetAPIVersionsCmd(verb, addParentFlags, parentPreRun); versionsCmd != nil {
		rv.AddCommand(versionsCmd)
	}

	if publicationsCmd := newGetAPIPublicationsCmd(verb, addParentFlags, parentPreRun); publicationsCmd != nil {
		rv.AddCommand(publicationsCmd)
	}

	if attributesCmd := newGetAPIAttributesCmd(verb, addParentFlags, parentPreRun); attributesCmd != nil {
		rv.AddCommand(attributesCmd)
	}

	if implementationsCmd := newGetAPIImplementationsCmd(verb, addParentFlags, parentPreRun); implementationsCmd != nil {
		rv.AddCommand(implementationsCmd)
	}

	return &rv
}
