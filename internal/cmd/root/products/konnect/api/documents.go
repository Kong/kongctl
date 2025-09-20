package api

import (
	"fmt"
	"strings"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
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
	Title            string
	Slug             string
	Status           string
	ParentDocumentID string
	Content          string
	LocalCreatedTime string
	LocalUpdatedTime string
}

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
		return h.getSingleDocument(helper, apiDocAPI, apiID, args[0], outType, printer)
	}

	return h.listDocuments(helper, apiDocAPI, apiID, outType, printer)
}

func (h apiDocumentsHandler) listDocuments(
	helper cmd.Helper,
	apiDocAPI helpers.APIDocumentAPI,
	apiID string,
	outType cmdCommon.OutputFormat,
	printer cli.Printer,
) error {
	docs, err := fetchDocumentSummaries(helper, apiDocAPI, apiID)
	if err != nil {
		return err
	}

	if outType == cmdCommon.TEXT {
		flattened := flattenDocuments(docs)
		records := make([]apiDocumentSummaryRecord, 0, len(flattened))
		for _, doc := range flattened {
			records = append(records, documentSummaryToRecord(doc))
		}
		printer.Print(records)
		return nil
	}

	printer.Print(docs)
	return nil
}

func (h apiDocumentsHandler) getSingleDocument(
	helper cmd.Helper,
	apiDocAPI helpers.APIDocumentAPI,
	apiID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.Printer,
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

	if outType == cmdCommon.TEXT {
		printer.Print(documentDetailToRecord(doc))
		return nil
	}

	printer.Print(doc)
	return nil
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
		ID:               doc.ID,
		Title:            doc.Title,
		Slug:             doc.Slug,
		Status:           status,
		ParentDocumentID: parentID,
		ChildrenCount:    len(doc.Children),
		LocalCreatedTime: doc.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: doc.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func documentDetailToRecord(doc *kkComps.APIDocumentResponse) apiDocumentDetailRecord {
	status := valueNA
	if doc.GetStatus() != nil {
		status = string(*doc.GetStatus())
	}

	parentID := valueNA
	if doc.GetParentDocumentID() != nil && *doc.GetParentDocumentID() != "" {
		parentID = *doc.GetParentDocumentID()
	}

	return apiDocumentDetailRecord{
		ID:               doc.GetID(),
		Title:            doc.GetTitle(),
		Slug:             doc.GetSlug(),
		Status:           status,
		ParentDocumentID: parentID,
		Content:          doc.GetContent(),
		LocalCreatedTime: doc.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: doc.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func findDocumentBySlugOrTitle(
	docs []kkComps.APIDocumentSummaryWithChildren,
	identifier string,
) *kkComps.APIDocumentSummaryWithChildren {
	for _, d := range docs {
		doc := d
		if strings.EqualFold(doc.Slug, identifier) || strings.EqualFold(doc.Title, identifier) {
			return &doc
		}
	}
	return nil
}
