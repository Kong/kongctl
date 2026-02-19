package portal

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

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
	snippetsCommandName = "snippets"
)

type portalSnippetSummaryRecord struct {
	ID               string
	Name             string
	Title            string
	Visibility       string
	Status           string
	Description      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

type portalSnippetDetailRecord struct {
	ID               string
	Name             string
	Title            string
	Visibility       string
	Status           string
	Description      string
	Content          string
	LocalCreatedTime string
	LocalUpdatedTime string
	rawID            string
}

type portalSnippetContentContext struct {
	portalID  string
	snippetID string
	cache     *portalSnippetDetailCache
}

type portalSnippetDetailCache struct {
	mu      sync.RWMutex
	records map[string]portalSnippetDetailRecord
}

func newPortalSnippetDetailCache() *portalSnippetDetailCache {
	return &portalSnippetDetailCache{
		records: make(map[string]portalSnippetDetailRecord),
	}
}

func (c *portalSnippetDetailCache) Get(id string) (portalSnippetDetailRecord, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rec, ok := c.records[id]
	return rec, ok
}

func (c *portalSnippetDetailCache) Set(id string, record portalSnippetDetailRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records[id] = record
}

var (
	snippetsUse = snippetsCommandName

	snippetsShort = i18n.T("root.products.konnect.portal.snippetsShort",
		"Manage portal snippets for a Konnect portal")
	snippetsLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.snippetsLong",
		`Use the snippets command to list or retrieve custom snippets for a specific Konnect portal.`))
	snippetsExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.snippetsExamples",
			fmt.Sprintf(`
# List snippets for a portal by ID
%[1]s get portal snippets --portal-id <portal-id>
# List snippets for a portal by name
%[1]s get portal snippets --portal-name my-portal
# Get a specific snippet by ID
%[1]s get portal snippets --portal-id <portal-id> <snippet-id>
# Get a specific snippet by name
%[1]s get portal snippets --portal-id <portal-id> welcome-message
`, meta.CLIName)))
)

func newGetPortalSnippetsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     snippetsUse,
		Short:   snippetsShort,
		Long:    snippetsLong,
		Example: snippetsExample,
		Aliases: []string{"snippet", "snip"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindPortalChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := portalSnippetsHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addPortalChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type portalSnippetsHandler struct {
	cmd *cobra.Command
}

func (h portalSnippetsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing portal snippets requires 0 or 1 arguments (ID or name)"),
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

	snippetAPI := sdk.GetPortalSnippetAPI()
	if snippetAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal snippets client is not available",
			Err: fmt.Errorf("portal snippets client not configured"),
		}
	}

	if len(args) == 1 {
		snippetIdentifier := strings.TrimSpace(args[0])
		return h.getSingleSnippet(
			helper,
			snippetAPI,
			portalID,
			snippetIdentifier,
			outType,
			printer,
			cfg,
		)
	}

	return h.listSnippets(helper, snippetAPI, portalID, outType, printer, cfg)
}

func (h portalSnippetsHandler) listSnippets(
	helper cmd.Helper,
	snippetAPI helpers.PortalSnippetAPI,
	portalID string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	snippets, err := fetchPortalSnippetSummaries(helper, snippetAPI, portalID, cfg)
	if err != nil {
		return err
	}

	records := make([]portalSnippetSummaryRecord, 0, len(snippets))
	for _, snippet := range snippets {
		records = append(records, portalSnippetSummaryToRecord(snippet))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	cache := newPortalSnippetDetailCache()

	detailFn := func(index int) string {
		if index < 0 || index >= len(snippets) {
			return ""
		}
		snippet := snippets[index]
		if detail, ok := cache.Get(snippet.GetID()); ok {
			return portalSnippetInfoDetail(snippet, &detail)
		}
		return portalSnippetInfoDetail(snippet, nil)
	}

	return tableview.RenderForFormat(helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		snippets,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME"}, tableRows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext("portal-snippet", func(index int) any {
			if index < 0 || index >= len(snippets) {
				return nil
			}
			snippet := snippets[index]
			return &portalSnippetContentContext{
				portalID:  portalID,
				snippetID: strings.TrimSpace(snippet.GetID()),
				cache:     cache,
			}
		}),
	)
}

func (h portalSnippetsHandler) getSingleSnippet(
	helper cmd.Helper,
	snippetAPI helpers.PortalSnippetAPI,
	portalID string,
	identifier string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	snippetID := identifier
	if !util.IsValidUUID(identifier) {
		snippets, err := fetchPortalSnippetSummaries(helper, snippetAPI, portalID, cfg)
		if err != nil {
			return err
		}
		match := findSnippetByNameOrTitle(snippets, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("snippet %q not found", identifier),
			}
		}
		snippetID = match.ID
	}

	res, err := snippetAPI.GetPortalSnippet(helper.GetContext(), portalID, snippetID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get portal snippet", err, helper.GetCmd(), attrs...)
	}

	snippet := res.GetPortalSnippetResponse()
	if snippet == nil {
		return &cmd.ExecutionError{
			Msg: "Portal snippet response was empty",
			Err: fmt.Errorf("no snippet returned for id %s", snippetID),
		}
	}

	record := portalSnippetDetailToRecord(snippet)
	cache := newPortalSnippetDetailCache()
	cache.Set(snippet.GetID(), record)

	detailRenderer := func(index int) string {
		if index != 0 {
			return ""
		}
		return portalSnippetDetailView(record)
	}

	return tableview.RenderForFormat(helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		snippet,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailRenderer(detailRenderer),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext("portal-snippet", func(index int) any {
			if index != 0 {
				return nil
			}
			return &portalSnippetContentContext{
				portalID:  portalID,
				snippetID: strings.TrimSpace(snippet.GetID()),
				cache:     cache,
			}
		}),
	)
}

func fetchPortalSnippetSummaries(
	helper cmd.Helper,
	snippetAPI helpers.PortalSnippetAPI,
	portalID string,
	cfg config.Hook,
) ([]kkComps.PortalSnippetInfo, error) {
	var pageNumber int64 = 1
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}

	var all []kkComps.PortalSnippetInfo

	for {
		req := kkOps.ListPortalSnippetsRequest{
			PortalID:   portalID,
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := snippetAPI.ListPortalSnippets(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list portal snippets", err, helper.GetCmd(), attrs...)
		}

		if res.GetListPortalSnippetsResponse() == nil {
			break
		}

		data := res.GetListPortalSnippetsResponse().GetData()
		all = append(all, data...)

		total := int(res.GetListPortalSnippetsResponse().GetMeta().Page.Total)
		if total == 0 || len(all) >= total || len(data) == 0 {
			break
		}

		pageNumber++
	}

	return all, nil
}

func findSnippetByNameOrTitle(snippets []kkComps.PortalSnippetInfo, identifier string) *kkComps.PortalSnippetInfo {
	lowered := strings.ToLower(identifier)
	for _, snippet := range snippets {
		if strings.ToLower(snippet.GetName()) == lowered || strings.ToLower(snippet.GetTitle()) == lowered {
			snippetCopy := snippet
			return &snippetCopy
		}
	}
	return nil
}

func portalSnippetSummaryToRecord(snippet kkComps.PortalSnippetInfo) portalSnippetSummaryRecord {
	return portalSnippetSummaryRecord{
		ID:               util.AbbreviateUUID(snippet.GetID()),
		Name:             snippet.GetName(),
		Title:            snippet.GetTitle(),
		Visibility:       string(snippet.GetVisibility()),
		Status:           string(snippet.GetStatus()),
		Description:      formatOptionalString(snippet.GetDescription()),
		LocalCreatedTime: formatTime(snippet.GetCreatedAt()),
		LocalUpdatedTime: formatTime(snippet.GetUpdatedAt()),
	}
}

func portalSnippetDetailToRecord(snippet *kkComps.PortalSnippetResponse) portalSnippetDetailRecord {
	content := normalizePortalPageContent(snippet.GetContent())
	record := portalSnippetDetailRecord{
		ID:               util.AbbreviateUUID(snippet.GetID()),
		Name:             snippet.GetName(),
		Title:            optionalString(snippet.GetTitle()),
		Visibility:       string(snippet.GetVisibility()),
		Status:           string(snippet.GetStatus()),
		Description:      formatOptionalString(snippet.GetDescription()),
		Content:          content,
		LocalCreatedTime: formatTime(snippet.GetCreatedAt()),
		LocalUpdatedTime: formatTime(snippet.GetUpdatedAt()),
		rawID:            strings.TrimSpace(snippet.GetID()),
	}
	return record
}

func formatOptionalString(s *string) string {
	if s == nil || *s == "" {
		return valueNA
	}
	return *s
}

func optionalString(s *string) string {
	if s == nil {
		return valueNA
	}
	if *s == "" {
		return valueNA
	}
	return *s
}

func portalSnippetInfoDetail(snippet kkComps.PortalSnippetInfo, detail *portalSnippetDetailRecord) string {
	const missing = valueNA

	id := strings.TrimSpace(snippet.GetID())
	if id == "" {
		id = missing
	}

	name := strings.TrimSpace(snippet.GetName())
	if name == "" {
		name = missing
	}

	title := strings.TrimSpace(snippet.GetTitle())
	if title == "" {
		title = missing
	}

	description := formatOptionalString(snippet.GetDescription())

	fields := map[string]string{
		"title":      title,
		"visibility": string(snippet.GetVisibility()),
		"status":     string(snippet.GetStatus()),
		"created_at": formatTime(snippet.GetCreatedAt()),
		"updated_at": formatTime(snippet.GetUpdatedAt()),
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

	if description != missing && strings.TrimSpace(description) != "" {
		fmt.Fprintf(&b, "description:\n%s\n", strings.TrimSpace(description))
	}

	fmt.Fprintf(&b, "content: %s", portalPageContentIndicator)
	if detail != nil {
		if preview := previewPortalPageContent(detail.Content); preview != "" {
			fmt.Fprintf(&b, " %s", preview)
		}
	}
	fmt.Fprintln(&b)

	return strings.TrimRight(b.String(), "\n")
}

func portalSnippetDetailView(record portalSnippetDetailRecord) string {
	const missing = valueNA

	id := strings.TrimSpace(record.rawID)
	if id == "" {
		id = missing
	}

	name := nonEmptyStringOrNA(record.Name)
	title := nonEmptyStringOrNA(record.Title)

	fields := map[string]string{
		"created_at": record.LocalCreatedTime,
		"status":     record.Status,
		"updated_at": record.LocalUpdatedTime,
		"visibility": record.Visibility,
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "title: %s\n", title)
	for _, key := range keys {
		fmt.Fprintf(&b, "%s: %s\n", key, fields[key])
	}

	desc := strings.TrimSpace(record.Description)
	if desc != "" && !strings.EqualFold(desc, valueNA) {
		fmt.Fprintf(&b, "description:\n%s\n", desc)
	}

	fmt.Fprintf(&b, "content: %s", portalPageContentIndicator)
	if preview := previewPortalPageContent(record.Content); preview != "" {
		fmt.Fprintf(&b, " %s", preview)
	}
	fmt.Fprintln(&b)

	return strings.TrimRight(b.String(), "\n")
}

func loadPortalSnippetContent(ctx context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	contentCtx, ok := parent.(*portalSnippetContentContext)
	if !ok {
		return tableview.ChildView{}, fmt.Errorf("unexpected portal snippet context type %T", parent)
	}

	if strings.TrimSpace(contentCtx.portalID) == "" || strings.TrimSpace(contentCtx.snippetID) == "" {
		return tableview.ChildView{}, fmt.Errorf("portal snippet identifiers are missing")
	}

	record, ok := contentCtx.cache.Get(contentCtx.snippetID)
	if !ok {
		detail, err := fetchPortalSnippetDetail(ctx, helper, contentCtx.portalID, contentCtx.snippetID)
		if err != nil {
			return tableview.ChildView{}, err
		}
		contentCtx.cache.Set(contentCtx.snippetID, detail)
		record = detail
	}

	raw := normalizePortalPageContent(record.Content)
	if strings.TrimSpace(raw) == "" {
		raw = "(content is empty)"
	}

	return tableview.ChildView{
		Headers: nil,
		Rows:    nil,
		DetailRenderer: func(int) string {
			return raw
		},
		Title: "Snippet Content",
	}, nil
}

func fetchPortalSnippetDetail(
	ctx context.Context,
	helper cmd.Helper,
	portalID string,
	snippetID string,
) (portalSnippetDetailRecord, error) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return portalSnippetDetailRecord{}, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return portalSnippetDetailRecord{}, err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return portalSnippetDetailRecord{}, err
	}

	snippetAPI := sdk.GetPortalSnippetAPI()
	if snippetAPI == nil {
		return portalSnippetDetailRecord{}, fmt.Errorf("portal snippets client is not available")
	}

	res, err := snippetAPI.GetPortalSnippet(ctx, portalID, snippetID)
	if err != nil {
		return portalSnippetDetailRecord{}, err
	}

	snippet := res.GetPortalSnippetResponse()
	if snippet == nil {
		return portalSnippetDetailRecord{}, fmt.Errorf("no portal snippet returned for id %s", snippetID)
	}

	record := portalSnippetDetailToRecord(snippet)
	return record, nil
}
