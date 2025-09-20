package portal

import (
	"fmt"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
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
			Err: fmt.Errorf("a portal identifier is required. Provide --%s or --%s", portalIDFlagName, portalNameFlagName),
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
		return h.getSingleSnippet(helper, snippetAPI, portalID, strings.TrimSpace(args[0]), outType, printer, cfg)
	}

	return h.listSnippets(helper, snippetAPI, portalID, outType, printer, cfg)
}

func (h portalSnippetsHandler) listSnippets(
	helper cmd.Helper,
	snippetAPI helpers.PortalSnippetAPI,
	portalID string,
	outType cmdCommon.OutputFormat,
	printer cli.Printer,
	cfg config.Hook,
) error {
	snippets, err := fetchPortalSnippetSummaries(helper, snippetAPI, portalID, cfg)
	if err != nil {
		return err
	}

	if outType == cmdCommon.TEXT {
		records := make([]portalSnippetSummaryRecord, 0, len(snippets))
		for _, snippet := range snippets {
			records = append(records, portalSnippetSummaryToRecord(snippet))
		}
		printer.Print(records)
		return nil
	}

	printer.Print(snippets)
	return nil
}

func (h portalSnippetsHandler) getSingleSnippet(
	helper cmd.Helper,
	snippetAPI helpers.PortalSnippetAPI,
	portalID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.Printer,
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

	if outType == cmdCommon.TEXT {
		printer.Print(portalSnippetDetailToRecord(snippet))
		return nil
	}

	printer.Print(snippet)
	return nil
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
		ID:               snippet.GetID(),
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
	return portalSnippetDetailRecord{
		ID:               snippet.GetID(),
		Name:             snippet.GetName(),
		Title:            optionalString(snippet.GetTitle()),
		Visibility:       string(snippet.GetVisibility()),
		Status:           string(snippet.GetStatus()),
		Description:      formatOptionalString(snippet.GetDescription()),
		Content:          snippet.GetContent(),
		LocalCreatedTime: formatTime(snippet.GetCreatedAt()),
		LocalUpdatedTime: formatTime(snippet.GetUpdatedAt()),
	}
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
