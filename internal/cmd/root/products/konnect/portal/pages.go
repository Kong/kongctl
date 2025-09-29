package portal

import (
	"fmt"
	"strings"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
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
	portalID string,
	outType cmdCommon.OutputFormat,
	printer cli.Printer,
) error {
	pages, err := fetchPortalPageSummaries(helper, pageAPI, portalID)
	if err != nil {
		return err
	}

	if outType == cmdCommon.TEXT {
		flattened := flattenPortalPages(pages)
		records := make([]portalPageSummaryRecord, 0, len(flattened))
		for _, page := range flattened {
			records = append(records, portalPageSummaryToRecord(page))
		}
		printer.Print(records)
		return nil
	}

	printer.Print(pages)
	return nil
}

func (h portalPagesHandler) getSinglePage(
	helper cmd.Helper,
	pageAPI helpers.PortalPageAPI,
	portalID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.Printer,
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

	if outType == cmdCommon.TEXT {
		printer.Print(portalPageDetailToRecord(page))
		return nil
	}

	printer.Print(page)
	return nil
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

func portalPageSummaryToRecord(page kkComps.PortalPageInfo) portalPageSummaryRecord {
	parentID := valueNA
	if parent := page.GetParentPageID(); parent != nil && *parent != "" {
		parentID = *parent
	}

	return portalPageSummaryRecord{
		ID:               page.GetID(),
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
		parentID = *parent
	}

	return portalPageDetailRecord{
		ID:               page.GetID(),
		Title:            page.GetTitle(),
		Slug:             page.GetSlug(),
		Visibility:       string(page.GetVisibility()),
		Status:           string(page.GetStatus()),
		ParentPageID:     parentID,
		Content:          page.GetContent(),
		LocalCreatedTime: formatTime(page.GetCreatedAt()),
		LocalUpdatedTime: formatTime(page.GetUpdatedAt()),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return valueNA
	}
	return t.In(time.Local).Format("2006-01-02 15:04:05")
}
