package portal

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	pagesCommandName = "pages"
)

type portalPageSummaryRecord struct {
	ID               string
	Title            string
	Slug             string
	Visibility       string
	Status           string
	ParentPageID     string
	ChildrenCount    int
	LocalCreatedTime string
	LocalUpdatedTime string
}

type portalPageDetailRecord struct {
	ID               string
	Title            string
	Slug             string
	Visibility       string
	Status           string
	ParentPageID     string
	Content          string
	LocalCreatedTime string
	LocalUpdatedTime string
	rawID            string
}

type portalPageContentContext struct {
	portalID string
	pageID   string
	cache    *portalPageDetailCache
}

type portalPageDetailCache struct {
	mu      sync.RWMutex
	records map[string]portalPageDetailRecord
}

func newPortalPageDetailCache() *portalPageDetailCache {
	return &portalPageDetailCache{
		records: make(map[string]portalPageDetailRecord),
	}
}

const (
	portalPageContentIndicator    = "[...]"
	portalPageContentPreviewLimit = 80
)

func (c *portalPageDetailCache) Get(id string) (portalPageDetailRecord, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rec, ok := c.records[id]
	return rec, ok
}

func (c *portalPageDetailCache) Set(id string, record portalPageDetailRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records[id] = record
}

var (
	pagesUse = pagesCommandName

	pagesShort = i18n.T("root.products.konnect.portal.pagesShort",
		"Manage portal pages for a Konnect portal")
	pagesLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.pagesLong",
		`Use the pages command to list or retrieve custom pages for a specific Konnect portal.`))
	pagesExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.pagesExamples",
			fmt.Sprintf(`
# List pages for a portal by ID
%[1]s get portal pages --portal-id <portal-id>
# List pages for a portal by name
%[1]s get portal pages --portal-name my-portal
# Get a specific page by ID
%[1]s get portal pages --portal-id <portal-id> <page-id>
# Get a specific page by slug
%[1]s get portal pages --portal-id <portal-id> getting-started
`, meta.CLIName)))
)

func newGetPortalPagesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     pagesUse,
		Short:   pagesShort,
		Long:    pagesLong,
		Example: pagesExample,
		Aliases: []string{"page", "pgs"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindPortalChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := portalPagesHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addPortalChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type portalPagesHandler struct {
	cmd *cobra.Command
}

func (h portalPagesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing portal pages requires 0 or 1 arguments (ID or slug)"),
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

	pageAPI := sdk.GetPortalPageAPI()
	if pageAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal pages client is not available",
			Err: fmt.Errorf("portal pages client not configured"),
		}
	}

	if len(args) == 1 {
		return h.getSinglePage(helper, pageAPI, portalID, strings.TrimSpace(args[0]), outType, printer)
	}

	return h.listPages(helper, pageAPI, portalID, outType, printer)
}

func (h portalPagesHandler) listPages(
	helper cmd.Helper,
	pageAPI helpers.PortalPageAPI,
	portalID string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	pages, err := fetchPortalPageSummaries(helper, pageAPI, portalID)
	if err != nil {
		return err
	}

	flattened := flattenPortalPages(pages)
	records := make([]portalPageSummaryRecord, 0, len(flattened))
	for _, page := range flattened {
		records = append(records, portalPageSummaryToRecord(page))
	}

	cache := newPortalPageDetailCache()

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Title})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(flattened) {
			return ""
		}
		page := flattened[index]
		if detail, ok := cache.Get(page.GetID()); ok {
			return portalPageInfoDetail(page, &detail)
		}
		return portalPageInfoDetail(page, nil)
	}

	return tableview.RenderForFormat(helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		pages,
		"",
		tableview.WithCustomTable([]string{"ID", "TITLE"}, tableRows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext("portal-page", func(index int) any {
			if index < 0 || index >= len(flattened) {
				return nil
			}
			page := flattened[index]
			return &portalPageContentContext{
				portalID: portalID,
				pageID:   strings.TrimSpace(page.GetID()),
				cache:    cache,
			}
		}),
	)
}

func (h portalPagesHandler) getSinglePage(
	helper cmd.Helper,
	pageAPI helpers.PortalPageAPI,
	portalID string,
	identifier string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	pageID := identifier
	if !util.IsValidUUID(identifier) {
		pages, err := fetchPortalPageSummaries(helper, pageAPI, portalID)
		if err != nil {
			return err
		}
		flattened := flattenPortalPages(pages)
		match := findPageBySlugOrTitle(flattened, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("page %q not found", identifier),
			}
		}
		pageID = match.ID
	}

	res, err := pageAPI.GetPortalPage(helper.GetContext(), portalID, pageID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get portal page", err, helper.GetCmd(), attrs...)
	}

	page := res.GetPortalPageResponse()
	if page == nil {
		return &cmd.ExecutionError{
			Msg: "Portal page response was empty",
			Err: fmt.Errorf("no page returned for id %s", pageID),
		}
	}

	record := portalPageDetailToRecord(page)
	cache := newPortalPageDetailCache()
	cache.Set(page.GetID(), record)

	detailRenderer := func(index int) string {
		if index != 0 {
			return ""
		}
		return portalPageDetailView(record)
	}

	return tableview.RenderForFormat(helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		page,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailRenderer(detailRenderer),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext("portal-page", func(index int) any {
			if index != 0 {
				return nil
			}
			return &portalPageContentContext{
				portalID: portalID,
				pageID:   strings.TrimSpace(page.GetID()),
				cache:    cache,
			}
		}),
	)
}

func fetchPortalPageSummaries(
	helper cmd.Helper,
	pageAPI helpers.PortalPageAPI,
	portalID string,
) ([]kkComps.PortalPageInfo, error) {
	req := kkOps.ListPortalPagesRequest{
		PortalID: portalID,
	}

	res, err := pageAPI.ListPortalPages(helper.GetContext(), req)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to list portal pages", err, helper.GetCmd(), attrs...)
	}

	if res.GetListPortalPagesResponse() == nil {
		return []kkComps.PortalPageInfo{}, nil
	}

	return res.GetListPortalPagesResponse().GetData(), nil
}

func flattenPortalPages(pages []kkComps.PortalPageInfo) []kkComps.PortalPageInfo {
	var result []kkComps.PortalPageInfo
	var walk func(kkComps.PortalPageInfo)

	walk = func(page kkComps.PortalPageInfo) {
		result = append(result, page)
		for _, child := range page.Children {
			walk(child)
		}
	}

	for _, page := range pages {
		walk(page)
	}

	return result
}

func findPageBySlugOrTitle(pages []kkComps.PortalPageInfo, identifier string) *kkComps.PortalPageInfo {
	lowered := strings.ToLower(identifier)
	for _, page := range pages {
		if strings.ToLower(page.GetSlug()) == lowered || strings.ToLower(page.GetTitle()) == lowered {
			pageCopy := page
			return &pageCopy
		}
	}
	return nil
}

func portalPageInfoDetail(page kkComps.PortalPageInfo, detail *portalPageDetailRecord) string {
	const missing = valueNA

	id := strings.TrimSpace(page.GetID())
	if id == "" {
		id = missing
	}

	title := strings.TrimSpace(page.GetTitle())
	if title == "" {
		title = missing
	}

	parentID := missing
	if pid := page.GetParentPageID(); pid != nil && *pid != "" {
		parentID = *pid
	}

	fields := map[string]string{
		"children_count": strconv.Itoa(len(page.GetChildren())),
		"created_at":     formatTime(page.GetCreatedAt()),
		"parent_page_id": parentID,
		"slug":           nonEmptyStringOrNA(page.GetSlug()),
		"status":         string(page.GetStatus()),
		"updated_at":     formatTime(page.GetUpdatedAt()),
		"visibility":     string(page.GetVisibility()),
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "title: %s\n", title)
	for _, key := range keys {
		fmt.Fprintf(&b, "%s: %s\n", key, fields[key])
	}

	if desc := page.GetDescription(); desc != nil && strings.TrimSpace(*desc) != "" {
		fmt.Fprintf(&b, "description:\n%s\n", strings.TrimSpace(*desc))
	}

	fmt.Fprintf(&b, "content: %s (press enter to view)", portalPageContentIndicator)
	if detail != nil {
		if preview := previewPortalPageContent(detail.Content); preview != "" {
			fmt.Fprintf(&b, " %s", preview)
		}
	}
	fmt.Fprintln(&b)

	return strings.TrimRight(b.String(), "\n")
}

func portalPageDetailView(record portalPageDetailRecord) string {
	const missing = valueNA

	id := strings.TrimSpace(record.rawID)
	if id == "" {
		id = missing
	}

	parentID := record.ParentPageID
	if strings.TrimSpace(parentID) == "" {
		parentID = missing
	}

	fields := map[string]string{
		"created_at":     record.LocalCreatedTime,
		"parent_page_id": parentID,
		"slug":           nonEmptyStringOrNA(record.Slug),
		"status":         record.Status,
		"updated_at":     record.LocalUpdatedTime,
		"visibility":     record.Visibility,
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "title: %s\n", nonEmptyStringOrNA(record.Title))
	for _, key := range keys {
		fmt.Fprintf(&b, "%s: %s\n", key, fields[key])
	}

	fmt.Fprintf(&b, "content: %s", portalPageContentIndicator)
	if preview := previewPortalPageContent(record.Content); preview != "" {
		fmt.Fprintf(&b, " %s", preview)
	}
	fmt.Fprintln(&b)

	return strings.TrimRight(b.String(), "\n")
}

func portalPageSummaryToRecord(page kkComps.PortalPageInfo) portalPageSummaryRecord {
	parentID := valueNA
	if parent := page.GetParentPageID(); parent != nil && *parent != "" {
		parentID = util.AbbreviateUUID(*parent)
	}

	return portalPageSummaryRecord{
		ID:               util.AbbreviateUUID(page.GetID()),
		Title:            page.GetTitle(),
		Slug:             page.GetSlug(),
		Visibility:       string(page.GetVisibility()),
		Status:           string(page.GetStatus()),
		ParentPageID:     parentID,
		ChildrenCount:    len(page.GetChildren()),
		LocalCreatedTime: formatTime(page.GetCreatedAt()),
		LocalUpdatedTime: formatTime(page.GetUpdatedAt()),
	}
}

func portalPageDetailToRecord(page *kkComps.PortalPageResponse) portalPageDetailRecord {
	parentID := valueNA
	if parent := page.GetParentPageID(); parent != nil && *parent != "" {
		parentID = util.AbbreviateUUID(*parent)
	}

	record := portalPageDetailRecord{
		ID:               util.AbbreviateUUID(page.GetID()),
		Title:            page.GetTitle(),
		Slug:             page.GetSlug(),
		Visibility:       string(page.GetVisibility()),
		Status:           string(page.GetStatus()),
		ParentPageID:     parentID,
		Content:          normalizePortalPageContent(page.GetContent()),
		LocalCreatedTime: formatTime(page.GetCreatedAt()),
		LocalUpdatedTime: formatTime(page.GetUpdatedAt()),
	}
	record.rawID = strings.TrimSpace(page.GetID())
	return record
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return valueNA
	}
	return t.In(time.Local).Format("2006-01-02 15:04:05")
}

func nonEmptyStringOrNA(value string) string {
	if strings.TrimSpace(value) == "" {
		return valueNA
	}
	return value
}

func normalizePortalPageContent(content string) string {
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

	// normalize tabs to spaces for consistent rendering
	for i, line := range lines {
		lines[i] = strings.ReplaceAll(line, "\t", "    ")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func previewPortalPageContent(content string) string {
	normalized := normalizePortalPageContent(content)
	trimmed := strings.TrimSpace(normalized)
	if trimmed == "" {
		return ""
	}
	preview := strings.ReplaceAll(normalized, "\n", " ")
	preview = strings.Join(strings.Fields(preview), " ")
	runes := []rune(preview)
	limit := portalPageContentPreviewLimit
	if limit <= 0 || len(runes) <= limit {
		return preview
	}
	return string(runes[:limit]) + "…"
}

func loadPortalPageContent(ctx context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	contentCtx, ok := parent.(*portalPageContentContext)
	if !ok {
		return tableview.ChildView{}, fmt.Errorf("unexpected portal page context type %T", parent)
	}
	if strings.TrimSpace(contentCtx.pageID) == "" || strings.TrimSpace(contentCtx.portalID) == "" {
		return tableview.ChildView{}, fmt.Errorf("portal page identifiers are missing")
	}

	record, ok := contentCtx.cache.Get(contentCtx.pageID)
	if !ok {
		detail, err := fetchPortalPageDetail(ctx, helper, contentCtx.portalID, contentCtx.pageID)
		if err != nil {
			return tableview.ChildView{}, err
		}
		contentCtx.cache.Set(contentCtx.pageID, detail)
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
		Title: "Page Content",
	}, nil
}

func fetchPortalPageDetail(
	ctx context.Context,
	helper cmd.Helper,
	portalID string,
	pageID string,
) (portalPageDetailRecord, error) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return portalPageDetailRecord{}, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return portalPageDetailRecord{}, err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return portalPageDetailRecord{}, err
	}

	pageAPI := sdk.GetPortalPageAPI()
	if pageAPI == nil {
		return portalPageDetailRecord{}, fmt.Errorf("portal pages client is not available")
	}

	res, err := pageAPI.GetPortalPage(ctx, portalID, pageID)
	if err != nil {
		return portalPageDetailRecord{}, err
	}

	page := res.GetPortalPageResponse()
	if page == nil {
		return portalPageDetailRecord{}, fmt.Errorf("no portal page returned for id %s", pageID)
	}

	return portalPageDetailToRecord(page), nil
}
