package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
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
	"sigs.k8s.io/yaml"
)

const (
	versionsCommandName = "versions"
)

type apiVersionSummaryRecord struct {
	ID               string
	Version          string
	SpecType         string
	LocalCreatedTime string
	LocalUpdatedTime string
}

type apiVersionDetailRecord struct {
	ID               string
	RawID            string
	Version          string
	SpecType         string
	SpecContent      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

type apiVersionContentContext struct {
	apiID   string
	version string
	cache   *apiVersionDetailCache
}

type apiVersionDetailCache struct {
	mu      sync.RWMutex
	records map[string]apiVersionDetailRecord
}

func newAPIVersionDetailCache() *apiVersionDetailCache {
	return &apiVersionDetailCache{
		records: make(map[string]apiVersionDetailRecord),
	}
}

func (c *apiVersionDetailCache) Get(id string) (apiVersionDetailRecord, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rec, ok := c.records[id]
	return rec, ok
}

func (c *apiVersionDetailCache) Set(id string, record apiVersionDetailRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records[id] = record
}

const (
	apiVersionSpecIndicator    = "[...]"
	apiVersionSpecPreviewLimit = 80
)

var (
	versionsUse = versionsCommandName

	versionsShort = i18n.T("root.products.konnect.api.versionsShort",
		"Manage API versions for a Konnect API")
	versionsLong = normalizers.LongDesc(i18n.T("root.products.konnect.api.versionsLong",
		`Use the versions command to list or retrieve API versions for a specific Konnect API.`))
	versionsExample = normalizers.Examples(
		i18n.T("root.products.konnect.api.versionsExamples",
			fmt.Sprintf(`
# List versions for an API by ID
%[1]s get api versions --api-id <api-id>
# List versions for an API by name
%[1]s get api versions --api-name my-api
# Get a specific version by ID
%[1]s get api versions --api-id <api-id> <version-id>
# Get a specific version by semantic version
%[1]s get api versions --api-id <api-id> 1.0.0
`, meta.CLIName)))
)

func newGetAPIVersionsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     versionsUse,
		Short:   versionsShort,
		Long:    versionsLong,
		Example: versionsExample,
		Aliases: []string{"version", "vs", "ver"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindAPIChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := apiVersionsHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addAPIChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type apiVersionsHandler struct {
	cmd *cobra.Command
}

func (h apiVersionsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"too many arguments. Listing API versions requires 0 or 1 arguments (ID or version string)",
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

	apiVersionAPI := sdk.GetAPIVersionAPI()
	if apiVersionAPI == nil {
		return &cmd.ExecutionError{
			Msg: "API versions client is not available",
			Err: fmt.Errorf("api versions client not configured"),
		}
	}

	if len(args) == 1 {
		versionIdentifier := strings.TrimSpace(args[0])
		return h.getSingleVersion(
			helper,
			apiVersionAPI,
			apiID,
			versionIdentifier,
			interactive,
			outType,
			printer,
			cfg,
		)
	}

	return h.listVersions(helper, apiVersionAPI, apiID, interactive, outType, printer, cfg)
}

func (h apiVersionsHandler) listVersions(
	helper cmd.Helper,
	apiVersionAPI helpers.APIVersionAPI,
	apiID string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	summaries, err := fetchVersionSummaries(helper, apiVersionAPI, apiID, cfg)
	if err != nil {
		return err
	}

	displayRecords := make([]apiVersionSummaryRecord, 0, len(summaries))
	rows := make([]table.Row, 0, len(summaries))
	detailCache := newAPIVersionDetailCache()
	for _, summary := range summaries {
		record := versionSummaryToRecord(summary)
		displayRecords = append(displayRecords, record)
		rows = append(rows, table.Row{summary.GetVersion(), util.AbbreviateUUID(record.ID)})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(summaries) {
			return ""
		}
		summary := &summaries[index]
		if detail, ok := detailCache.Get(summary.GetID()); ok {
			return versionSummaryDetailView(summary, &detail)
		}
		return versionSummaryDetailView(summary, nil)
	}

	return tableview.RenderForFormat(
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		displayRecords,
		summaries,
		"",
		tableview.WithTitle("Versions"),
		tableview.WithCustomTable([]string{"VERSION", "ID"}, rows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailContext("api-version", func(index int) any {
			if index < 0 || index >= len(summaries) {
				return nil
			}
			summary := summaries[index]
			return &apiVersionContentContext{
				apiID:   apiID,
				version: strings.TrimSpace(summary.GetID()),
				cache:   detailCache,
			}
		}),
		tableview.WithDetailHelper(helper),
	)
}

func (h apiVersionsHandler) getSingleVersion(
	helper cmd.Helper,
	apiVersionAPI helpers.APIVersionAPI,
	apiID string,
	identifier string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	var versionID string

	if util.IsValidUUID(identifier) {
		versionID = identifier
	} else {
		summaries, err := fetchVersionSummaries(helper, apiVersionAPI, apiID, cfg)
		if err != nil {
			return err
		}
		match := findVersionByString(summaries, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("version %q not found", identifier),
			}
		}
		versionID = match.ID
	}

	res, err := apiVersionAPI.FetchAPIVersion(helper.GetContext(), apiID, versionID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get API version", err, helper.GetCmd(), attrs...)
	}

	version := res.GetAPIVersionResponse()
	if version == nil {
		return &cmd.ExecutionError{
			Msg: "API version response was empty",
			Err: fmt.Errorf("no version returned for id %s", versionID),
		}
	}

	record := versionDetailToRecord(version)

	cache := newAPIVersionDetailCache()
	cache.Set(record.RawID, record)

	rows := []table.Row{
		{record.Version, util.AbbreviateUUID(record.ID)},
	}

	display := any(record)
	if interactive {
		display = []apiVersionSummaryRecord{{
			ID:               record.ID,
			Version:          record.Version,
			SpecType:         record.SpecType,
			LocalCreatedTime: record.LocalCreatedTime,
			LocalUpdatedTime: record.LocalUpdatedTime,
		}}
	}

	return tableview.RenderForFormat(
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		display,
		version,
		"",
		tableview.WithCustomTable([]string{"VERSION", "ID"}, rows),
		tableview.WithDetailRenderer(func(int) string {
			return apiVersionDetailView(record)
		}),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext("api-version", func(index int) any {
			if index != 0 {
				return nil
			}
			return &apiVersionContentContext{
				apiID:   apiID,
				version: strings.TrimSpace(record.RawID),
				cache:   cache,
			}
		}),
	)
}

func fetchVersionSummaries(
	helper cmd.Helper,
	apiVersionAPI helpers.APIVersionAPI,
	apiID string,
	cfg config.Hook,
) ([]kkComps.ListAPIVersionResponseAPIVersionSummary, error) {
	var pageNumber int64 = 1
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}

	var all []kkComps.ListAPIVersionResponseAPIVersionSummary

	for {
		req := kkOps.ListAPIVersionsRequest{
			APIID:      apiID,
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := apiVersionAPI.ListAPIVersions(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list API versions", err, helper.GetCmd(), attrs...)
		}

		if res.GetListAPIVersionResponse() == nil {
			break
		}

		data := res.GetListAPIVersionResponse().GetData()
		all = append(all, data...)

		total := int(res.GetListAPIVersionResponse().GetMeta().Page.Total)
		if total == 0 || len(all) >= total || len(data) == 0 {
			break
		}

		pageNumber++
	}

	return all, nil
}

func findVersionByString(
	summaries []kkComps.ListAPIVersionResponseAPIVersionSummary,
	identifier string,
) *kkComps.ListAPIVersionResponseAPIVersionSummary {
	lowered := strings.ToLower(identifier)
	for i := range summaries {
		if strings.ToLower(summaries[i].GetVersion()) == lowered {
			return &summaries[i]
		}
	}
	return nil
}

func versionSummaryToRecord(summary kkComps.ListAPIVersionResponseAPIVersionSummary) apiVersionSummaryRecord {
	specType := valueNA
	if summary.GetSpec() != nil && summary.GetSpec().GetType() != nil {
		specType = string(*summary.GetSpec().GetType())
	}

	return apiVersionSummaryRecord{
		ID:               summary.GetID(),
		Version:          summary.GetVersion(),
		SpecType:         specType,
		LocalCreatedTime: summary.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: summary.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func versionDetailToRecord(version *kkComps.APIVersionResponse) apiVersionDetailRecord {
	specType := valueNA
	if version.GetSpec() != nil && version.GetSpec().GetType() != nil {
		specType = string(*version.GetSpec().GetType())
	}

	return apiVersionDetailRecord{
		ID:       util.AbbreviateUUID(version.GetID()),
		RawID:    strings.TrimSpace(version.GetID()),
		Version:  version.GetVersion(),
		SpecType: specType,
		SpecContent: func() string {
			if version.GetSpec() != nil && version.GetSpec().GetContent() != nil {
				return normalizeAPIVersionContent(*version.GetSpec().GetContent())
			}
			return ""
		}(),
		LocalCreatedTime: version.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: version.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func versionSummaryDetailView(
	summary *kkComps.ListAPIVersionResponseAPIVersionSummary,
	detail *apiVersionDetailRecord,
) string {
	if summary == nil {
		return ""
	}

	specType := valueNA
	if summary.GetSpec() != nil && summary.GetSpec().GetType() != nil {
		specType = string(*summary.GetSpec().GetType())
	}

	fields := map[string]string{
		"created_at": summary.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
		"spec_type":  specType,
		"updated_at": summary.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", summary.GetID())
	fmt.Fprintf(&b, "version: %s\n", summary.GetVersion())
	for _, key := range keys {
		fmt.Fprintf(&b, "%s: %s\n", key, fields[key])
	}

	fmt.Fprintf(&b, "spec: %s (press enter to view)", apiVersionSpecIndicator)
	if detail != nil {
		if preview := previewAPIVersionContent(detail.SpecContent, apiVersionSpecPreviewLimit); preview != "" {
			fmt.Fprintf(&b, " %s", preview)
		}
	}
	fmt.Fprintln(&b)

	return strings.TrimRight(b.String(), "\n")
}

func apiVersionDetailView(record apiVersionDetailRecord) string {
	fields := map[string]string{
		"created_at": record.LocalCreatedTime,
		"spec_type":  record.SpecType,
		"updated_at": record.LocalUpdatedTime,
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", record.RawID)
	fmt.Fprintf(&b, "version: %s\n", record.Version)
	for _, key := range keys {
		fmt.Fprintf(&b, "%s: %s\n", key, fields[key])
	}

	fmt.Fprintf(&b, "spec: %s (press enter to view)", apiVersionSpecIndicator)
	if preview := previewAPIVersionContent(record.SpecContent, apiVersionSpecPreviewLimit); preview != "" {
		fmt.Fprintf(&b, " %s", preview)
	}
	fmt.Fprintln(&b)

	return strings.TrimRight(b.String(), "\n")
}

func normalizeAPIVersionContent(content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	minIndent := -1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		leading := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent == -1 || leading < minIndent {
			minIndent = leading
		}
	}

	if minIndent > 0 {
		for i, line := range lines {
			if strings.TrimSpace(line) == "" {
				lines[i] = ""
				continue
			}
			if len(line) > minIndent {
				lines[i] = line[minIndent:]
			} else {
				lines[i] = strings.TrimLeft(line, " \t")
			}
		}
	}

	for i, line := range lines {
		lines[i] = strings.ReplaceAll(line, "\t", "    ")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func previewAPIVersionContent(content string, limit int) string {
	formatted, _ := formatAPIVersionSpecContent(content)
	trimmed := strings.TrimSpace(formatted)
	if trimmed == "" {
		return ""
	}
	preview := strings.Join(strings.Fields(strings.ReplaceAll(formatted, "\n", " ")), " ")
	if limit <= 0 || len([]rune(preview)) <= limit {
		return preview
	}
	runes := []rune(preview)
	if limit < 1 {
		return ""
	}
	return string(runes[:limit]) + "…"
}

func fetchAPIVersionDetail(helper cmd.Helper, apiID, versionID string) (apiVersionDetailRecord, error) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return apiVersionDetailRecord{}, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return apiVersionDetailRecord{}, err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return apiVersionDetailRecord{}, err
	}

	apiVersionAPI := sdk.GetAPIVersionAPI()
	if apiVersionAPI == nil {
		return apiVersionDetailRecord{}, fmt.Errorf("api versions client is not available")
	}

	res, err := apiVersionAPI.FetchAPIVersion(helper.GetContext(), apiID, versionID)
	if err != nil {
		return apiVersionDetailRecord{}, err
	}

	version := res.GetAPIVersionResponse()
	if version == nil {
		return apiVersionDetailRecord{}, fmt.Errorf("no version returned for id %s", versionID)
	}

	return versionDetailToRecord(version), nil
}

func loadAPIVersionSpec(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	ctx, ok := parent.(*apiVersionContentContext)
	if !ok {
		return tableview.ChildView{}, fmt.Errorf("unexpected api version context type %T", parent)
	}
	if strings.TrimSpace(ctx.apiID) == "" || strings.TrimSpace(ctx.version) == "" {
		return tableview.ChildView{}, fmt.Errorf("version identifiers are missing")
	}

	record, ok := ctx.cache.Get(ctx.version)
	if !ok {
		detail, err := fetchAPIVersionDetail(helper, ctx.apiID, ctx.version)
		if err != nil {
			return tableview.ChildView{}, err
		}
		ctx.cache.Set(ctx.version, detail)
		record = detail
	}

	markdown := renderAPIVersionSpecMarkdown(record)

	return tableview.ChildView{
		Headers: nil,
		Rows:    nil,
		DetailRenderer: func(int) string {
			return markdown
		},
		Title: "Spec",
	}, nil
}

func renderAPIVersionSpecMarkdown(record apiVersionDetailRecord) string {
	content, language := formatAPIVersionSpecContent(record.SpecContent)
	specType := strings.TrimSpace(record.SpecType)
	if specType == "" {
		specType = valueNA
	}

	var b strings.Builder
	fmt.Fprintf(&b, "**Spec Type:** %s\n\n", specType)

	if strings.TrimSpace(content) == "" {
		b.WriteString("(content is empty)")
		return b.String()
	}

	if language == "" {
		fmt.Fprintf(&b, "```\n%s\n```", content)
	} else {
		fmt.Fprintf(&b, "```%s\n%s\n```", language, content)
	}

	return b.String()
}

func formatAPIVersionSpecContent(raw string) (string, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}

	data, err := parseAPIVersionSpecJSON(trimmed)
	if err == nil {
		if yamlContent, err := marshalSpecYAML(data); err == nil {
			return yamlContent, "yaml"
		}
		if jsonContent, err := marshalSpecJSON(data); err == nil {
			return jsonContent, "json"
		}
	}

	return normalizeAPIVersionContent(raw), ""
}

func parseAPIVersionSpecJSON(raw string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()

	var data any
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	return convertJSONNumbers(data), nil
}

func marshalSpecYAML(data any) (string, error) {
	out, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func marshalSpecJSON(data any) (string, error) {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func convertJSONNumbers(value any) any {
	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			result[key] = convertJSONNumbers(val)
		}
		return result
	case []any:
		for i := range v {
			v[i] = convertJSONNumbers(v[i])
		}
		return v
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i
		}
		if f, err := v.Float64(); err == nil {
			return f
		}
		return v.String()
	default:
		return v
	}
}
