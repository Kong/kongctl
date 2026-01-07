package api

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
	documentsCommandName = "documents"
)

type apiDocumentSummaryRecord struct {
	ID               string
	Title            string
	Slug             string
	Status           string
	ParentDocumentID string
	ChildrenCount    int
	LocalCreatedTime string
	LocalUpdatedTime string
}

type apiDocumentDetailRecord struct {
	ID               string
	RawID            string
	Title            string
	Slug             string
	Status           string
	ParentDocumentID string
	Content          string
	LocalCreatedTime string
	LocalUpdatedTime string
}

type apiDocumentContentContext struct {
	apiID string
	docID string
	cache *apiDocumentDetailCache
}

type apiDocumentDetailCache struct {
	mu      sync.RWMutex
	records map[string]apiDocumentDetailRecord
}

func newAPIDocumentDetailCache() *apiDocumentDetailCache {
	return &apiDocumentDetailCache{records: make(map[string]apiDocumentDetailRecord)}
}

func (c *apiDocumentDetailCache) Get(id string) (apiDocumentDetailRecord, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rec, ok := c.records[id]
	return rec, ok
}

func (c *apiDocumentDetailCache) Set(id string, record apiDocumentDetailRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records[id] = record
}

const (
	apiDocumentContentIndicator    = "[...]"
	apiDocumentContentPreviewLimit = 80
)

var (
	documentsUse = documentsCommandName

	documentsShort = i18n.T("root.products.konnect.api.documentsShort",
		"Manage API documents for a Konnect API")
	documentsLong = normalizers.LongDesc(i18n.T("root.products.konnect.api.documentsLong",
		`Use the documents command to list or retrieve API documents for a specific Konnect API.`))
	documentsExample = normalizers.Examples(
		i18n.T("root.products.konnect.api.documentsExamples",
			fmt.Sprintf(`
# List documents for an API by ID
%[1]s get api documents --api-id <api-id>
# List documents for an API by name
%[1]s get api documents --api-name my-api
# Get a specific document by ID
%[1]s get api documents --api-id <api-id> <document-id>
# Get a specific document by slug
%[1]s get api documents --api-id <api-id> getting-started
`, meta.CLIName)))
)

func newGetAPIDocumentsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     documentsUse,
		Short:   documentsShort,
		Long:    documentsLong,
		Example: documentsExample,
		Aliases: []string{"document", "docs", "doc"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindAPIChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := apiDocumentsHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addAPIChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type apiDocumentsHandler struct {
	cmd *cobra.Command
}

func (h apiDocumentsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing API documents requires 0 or 1 arguments (ID, slug, or title)"),
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

	apiDocAPI := sdk.GetAPIDocumentAPI()
	if apiDocAPI == nil {
		return &cmd.ExecutionError{
			Msg: "API documents client is not available",
			Err: fmt.Errorf("api documents client not configured"),
		}
	}

	if len(args) == 1 {
		return h.getSingleDocument(helper, apiDocAPI, apiID, args[0],  outType, printer)
	}

	return h.listDocuments(helper, apiDocAPI, apiID,  outType, printer)
}

func (h apiDocumentsHandler) listDocuments(
	helper cmd.Helper,
	apiDocAPI helpers.APIDocumentAPI,
	apiID string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	docs, err := fetchDocumentSummaries(helper, apiDocAPI, apiID)
	if err != nil {
		return err
	}

	flattened := flattenDocuments(docs)
	records := make([]apiDocumentSummaryRecord, 0, len(flattened))
	rows := make([]table.Row, 0, len(flattened))
	detailCache := newAPIDocumentDetailCache()
	for _, doc := range flattened {
		record := documentSummaryToRecord(doc)
		records = append(records, record)
		rows = append(rows, table.Row{record.ID, record.Title})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(flattened) {
			return ""
		}
		summary := &flattened[index]
		if detail, ok := detailCache.Get(summary.ID); ok {
			return documentSummaryDetailView(summary, &detail)
		}
		return documentSummaryDetailView(summary, nil)
	}

	return tableview.RenderForFormat(
			false,
			outType,
		printer,
		helper.GetStreams(),
		records,
		flattened,
		"",
		tableview.WithTitle("Documents"),
		tableview.WithCustomTable([]string{"DOCUMENT", "TITLE"}, rows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailContext("api-document", func(index int) any {
			if index < 0 || index >= len(flattened) {
				return nil
			}
			doc := flattened[index]
			return &apiDocumentContentContext{
				apiID: apiID,
				docID: strings.TrimSpace(doc.ID),
				cache: detailCache,
			}
		}),
		tableview.WithDetailHelper(helper),
	)
}

func (h apiDocumentsHandler) getSingleDocument(
	helper cmd.Helper,
	apiDocAPI helpers.APIDocumentAPI,
	apiID string,
	identifier string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	documentID := strings.TrimSpace(identifier)

	if !util.IsValidUUID(documentID) {
		docs, err := fetchDocumentSummaries(helper, apiDocAPI, apiID)
		if err != nil {
			return err
		}

		flattened := flattenDocuments(docs)
		match := findDocumentBySlugOrTitle(flattened, documentID)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("document %q not found", identifier),
			}
		}
		documentID = match.ID
	}

	res, err := apiDocAPI.FetchAPIDocument(helper.GetContext(), apiID, documentID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get API document", err, helper.GetCmd(), attrs...)
	}

	doc := res.GetAPIDocumentResponse()
	if doc == nil {
		return &cmd.ExecutionError{
			Msg: "API document response was empty",
			Err: fmt.Errorf("no document returned for id %s", documentID),
		}
	}

	record := apiDocumentSummaryRecord{
		ID:    util.AbbreviateUUID(doc.GetID()),
		Title: doc.GetTitle(),
		Slug:  doc.GetSlug(),
		Status: func() string {
			if doc.GetStatus() != nil {
				return string(*doc.GetStatus())
			}
			return valueNA
		}(),
		ParentDocumentID: func() string {
			if doc.GetParentDocumentID() != nil && *doc.GetParentDocumentID() != "" {
				return util.AbbreviateUUID(*doc.GetParentDocumentID())
			}
			return valueNA
		}(),
		ChildrenCount:    0,
		LocalCreatedTime: doc.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: doc.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
	}

	rows := []table.Row{
		{record.ID, record.Title},
	}

	detailRecord := apiDocumentDetailToRecord(doc)
	cache := newAPIDocumentDetailCache()
	cache.Set(doc.GetID(), detailRecord)

	return tableview.RenderForFormat(
			false,
			outType,
		printer,
		helper.GetStreams(),
		[]apiDocumentSummaryRecord{record},
		doc,
		"",
		tableview.WithCustomTable([]string{"DOCUMENT", "TITLE"}, rows),
		tableview.WithDetailRenderer(func(int) string {
			return apiDocumentDetailView(detailRecord)
		}),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext("api-document", func(index int) any {
			if index != 0 {
				return nil
			}
			return &apiDocumentContentContext{
				apiID: apiID,
				docID: strings.TrimSpace(doc.GetID()),
				cache: cache,
			}
		}),
	)
}

func fetchDocumentSummaries(
	helper cmd.Helper,
	apiDocAPI helpers.APIDocumentAPI,
	apiID string,
) ([]kkComps.APIDocumentSummaryWithChildren, error) {
	res, err := apiDocAPI.ListAPIDocuments(helper.GetContext(), apiID, nil)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to list API documents", err, helper.GetCmd(), attrs...)
	}

	if res == nil || res.GetListAPIDocumentResponse() == nil {
		return []kkComps.APIDocumentSummaryWithChildren{}, nil
	}

	return res.GetListAPIDocumentResponse().GetData(), nil
}

func flattenDocuments(docs []kkComps.APIDocumentSummaryWithChildren) []kkComps.APIDocumentSummaryWithChildren {
	var result []kkComps.APIDocumentSummaryWithChildren
	var walk func(kkComps.APIDocumentSummaryWithChildren)

	walk = func(doc kkComps.APIDocumentSummaryWithChildren) {
		result = append(result, doc)
		for _, child := range doc.Children {
			walk(child)
		}
	}

	for _, doc := range docs {
		walk(doc)
	}

	return result
}

func documentSummaryToRecord(doc kkComps.APIDocumentSummaryWithChildren) apiDocumentSummaryRecord {
	status := valueNA
	if doc.Status != nil {
		status = string(*doc.Status)
	}

	parentID := valueNA
	if doc.ParentDocumentID != nil && *doc.ParentDocumentID != "" {
		parentID = *doc.ParentDocumentID
	}

	return apiDocumentSummaryRecord{
		ID:               util.AbbreviateUUID(doc.ID),
		Title:            doc.Title,
		Slug:             doc.Slug,
		Status:           status,
		ParentDocumentID: parentID,
		ChildrenCount:    len(doc.Children),
		LocalCreatedTime: doc.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: doc.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func findDocumentBySlugOrTitle(
	docs []kkComps.APIDocumentSummaryWithChildren,
	identifier string,
) *kkComps.APIDocumentSummaryWithChildren {
	for _, d := range docs {
		if strings.EqualFold(d.Slug, identifier) || strings.EqualFold(d.Title, identifier) {
			return &d
		}
	}
	return nil
}

func documentSummaryDetailView(doc *kkComps.APIDocumentSummaryWithChildren, detail *apiDocumentDetailRecord) string {
	if doc == nil {
		return ""
	}

	const missing = "n/a"

	status := missing
	if doc.Status != nil && *doc.Status != "" {
		status = string(*doc.Status)
	}

	parentID := missing
	if doc.ParentDocumentID != nil && *doc.ParentDocumentID != "" {
		parentID = *doc.ParentDocumentID
	}

	fields := map[string]string{
		"children_count":     strconv.Itoa(len(doc.Children)),
		"created_at":         doc.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
		"parent_document_id": parentID,
		"slug":               doc.Slug,
		"status":             status,
		"updated_at":         doc.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
	}

	var keys []string
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", doc.ID)
	fmt.Fprintf(&b, "title: %s\n", doc.Title)
	for _, key := range keys {
		fmt.Fprintf(&b, "%s: %s\n", key, fields[key])
	}

	fmt.Fprintf(&b, "content: %s (press enter to view)", apiDocumentContentIndicator)
	if detail != nil {
		if preview := previewAPIDocumentContent(detail.Content, apiDocumentContentPreviewLimit); preview != "" {
			fmt.Fprintf(&b, " %s", preview)
		}
	}
	fmt.Fprintln(&b)

	return strings.TrimRight(b.String(), "\n")
}

func apiDocumentDetailView(record apiDocumentDetailRecord) string {
	const missing = "n/a"

	parentID := record.ParentDocumentID
	if strings.TrimSpace(parentID) == "" {
		parentID = missing
	}

	otherFields := map[string]string{
		"created_at":         record.LocalCreatedTime,
		"parent_document_id": parentID,
		"slug":               record.Slug,
		"status":             record.Status,
		"updated_at":         record.LocalUpdatedTime,
	}

	keys := make([]string, 0, len(otherFields))
	for key := range otherFields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", record.RawID)
	fmt.Fprintf(&b, "title: %s\n", record.Title)
	for _, key := range keys {
		fmt.Fprintf(&b, "%s: %s\n", key, otherFields[key])
	}

	fmt.Fprintf(&b, "content: %s (press enter to view)", apiDocumentContentIndicator)
	if preview := previewAPIDocumentContent(record.Content, apiDocumentContentPreviewLimit); preview != "" {
		fmt.Fprintf(&b, " %s", preview)
	}
	fmt.Fprintln(&b)

	return strings.TrimRight(b.String(), "\n")
}

func apiDocumentDetailToRecord(doc *kkComps.APIDocumentResponse) apiDocumentDetailRecord {
	if doc == nil {
		return apiDocumentDetailRecord{}
	}

	parentID := valueNA
	if doc.GetParentDocumentID() != nil && *doc.GetParentDocumentID() != "" {
		parentID = util.AbbreviateUUID(*doc.GetParentDocumentID())
	}

	content := normalizeAPIDocumentContent(doc.GetContent())

	return apiDocumentDetailRecord{
		ID:    util.AbbreviateUUID(doc.GetID()),
		RawID: strings.TrimSpace(doc.GetID()),
		Title: doc.GetTitle(),
		Slug:  doc.GetSlug(),
		Status: func() string {
			if doc.GetStatus() != nil && *doc.GetStatus() != "" {
				return string(*doc.GetStatus())
			}
			return valueNA
		}(),
		ParentDocumentID: parentID,
		Content:          content,
		LocalCreatedTime: doc.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: doc.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func normalizeAPIDocumentContent(content string) string {
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

func previewAPIDocumentContent(content string, limit int) string {
	normalized := normalizeAPIDocumentContent(content)
	trimmed := strings.TrimSpace(normalized)
	if trimmed == "" {
		return ""
	}
	preview := strings.Join(strings.Fields(strings.ReplaceAll(normalized, "\n", " ")), " ")
	if limit <= 0 || len([]rune(preview)) <= limit {
		return preview
	}
	runes := []rune(preview)
	if limit < 1 {
		return ""
	}
	return string(runes[:limit]) + "…"
}

func loadAPIDocumentContent(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	contentCtx, ok := parent.(*apiDocumentContentContext)
	if !ok {
		return tableview.ChildView{}, fmt.Errorf("unexpected api document context type %T", parent)
	}
	if strings.TrimSpace(contentCtx.apiID) == "" || strings.TrimSpace(contentCtx.docID) == "" {
		return tableview.ChildView{}, fmt.Errorf("document identifiers are missing")
	}

	record, ok := contentCtx.cache.Get(contentCtx.docID)
	if !ok {
		detail, err := fetchAPIDocumentDetail(helper, contentCtx.apiID, contentCtx.docID)
		if err != nil {
			return tableview.ChildView{}, err
		}
		contentCtx.cache.Set(contentCtx.docID, detail)
		record = detail
	}

	raw := normalizeAPIDocumentContent(record.Content)
	if strings.TrimSpace(raw) == "" {
		raw = "(content is empty)"
	}

	return tableview.ChildView{
		Headers: nil,
		Rows:    nil,
		DetailRenderer: func(int) string {
			return raw
		},
		Title: "Document Content",
	}, nil
}

func fetchAPIDocumentDetail(helper cmd.Helper, apiID, docID string) (apiDocumentDetailRecord, error) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return apiDocumentDetailRecord{}, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return apiDocumentDetailRecord{}, err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return apiDocumentDetailRecord{}, err
	}

	docAPI := sdk.GetAPIDocumentAPI()
	if docAPI == nil {
		return apiDocumentDetailRecord{}, fmt.Errorf("api documents client is not available")
	}

	res, err := docAPI.FetchAPIDocument(helper.GetContext(), apiID, docID)
	if err != nil {
		return apiDocumentDetailRecord{}, err
	}

	doc := res.GetAPIDocumentResponse()
	if doc == nil {
		return apiDocumentDetailRecord{}, fmt.Errorf("no document returned for id %s", docID)
	}

	return apiDocumentDetailToRecord(doc), nil
}
