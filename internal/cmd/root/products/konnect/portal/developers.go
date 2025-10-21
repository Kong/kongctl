package portal

import (
	"fmt"
	"strings"

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
	developersCommandName = "developers"
)

type portalDeveloperSummaryRecord struct {
	ID               string
	Email            string
	FullName         string
	Status           string
	LocalCreatedTime string
	LocalUpdatedTime string
}

var (
	developersUse = developersCommandName

	developersShort = i18n.T("root.products.konnect.portal.developersShort",
		"Manage portal developers for a Konnect portal")
	developersLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.developersLong",
		`Use the developers command to list or retrieve developers for a specific Konnect portal.`))
	developersExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.developersExamples",
			fmt.Sprintf(`
# List developers for a portal by ID
%[1]s get portal developers --portal-id <portal-id>
# List developers for a portal by name
%[1]s get portal developers --portal-name my-portal
# Get a specific developer by ID
%[1]s get portal developers --portal-id <portal-id> <developer-id>
# Get a specific developer by email
%[1]s get portal developers --portal-id <portal-id> dev@example.com
`, meta.CLIName)))
)

func newGetPortalDevelopersCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     developersUse,
		Short:   developersShort,
		Long:    developersLong,
		Example: developersExample,
		Aliases: []string{"developer", "devs"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindPortalChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := portalDevelopersHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addPortalChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type portalDevelopersHandler struct {
	cmd *cobra.Command
}

func (h portalDevelopersHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing portal developers requires 0 or 1 arguments (ID or email)"),
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

	devAPI := sdk.GetPortalDeveloperAPI()
	if devAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal developers client is not available",
			Err: fmt.Errorf("portal developers client not configured"),
		}
	}

	if len(args) == 1 {
		return h.getSingleDeveloper(
			helper,
			devAPI,
			portalID,
			strings.TrimSpace(args[0]),
			interactive,
			outType,
			printer,
			cfg,
		)
	}

	return h.listDevelopers(helper, devAPI, portalID, interactive, outType, printer, cfg)
}

func (h portalDevelopersHandler) listDevelopers(
	helper cmd.Helper,
	devAPI helpers.PortalDeveloperAPI,
	portalID string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	developers, err := fetchPortalDevelopers(helper, devAPI, portalID, cfg)
	if err != nil {
		return err
	}

	records := make([]portalDeveloperSummaryRecord, 0, len(developers))
	for _, developer := range developers {
		records = append(records, portalDeveloperSummaryToRecord(developer))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Email})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(developers) {
			return ""
		}
		return portalDeveloperDetailView(developers[index])
	}

	return tableview.RenderForFormat(
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		records,
		developers,
		"",
		tableview.WithCustomTable([]string{"ID", "EMAIL"}, tableRows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h portalDevelopersHandler) getSingleDeveloper(
	helper cmd.Helper,
	devAPI helpers.PortalDeveloperAPI,
	portalID string,
	identifier string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	developerID := identifier
	if !util.IsValidUUID(identifier) {
		developers, err := fetchPortalDevelopers(helper, devAPI, portalID, cfg)
		if err != nil {
			return err
		}
		match := findDeveloperByEmailOrID(developers, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("developer %q not found", identifier),
			}
		}
		developerID = match.GetID()
	}

	res, err := devAPI.GetDeveloper(helper.GetContext(), portalID, developerID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get portal developer", err, helper.GetCmd(), attrs...)
	}

	developer := res.GetPortalDeveloper()
	if developer == nil {
		return &cmd.ExecutionError{
			Msg: "Portal developer response was empty",
			Err: fmt.Errorf("no developer returned for id %s", developerID),
		}
	}

	return tableview.RenderForFormat(
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		portalDeveloperSummaryToRecord(*developer),
		developer,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func fetchPortalDevelopers(
	helper cmd.Helper,
	devAPI helpers.PortalDeveloperAPI,
	portalID string,
	cfg config.Hook,
) ([]kkComps.PortalDeveloper, error) {
	var pageNumber int64 = 1
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}

	var all []kkComps.PortalDeveloper

	for {
		req := kkOps.ListPortalDevelopersRequest{
			PortalID:   portalID,
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := devAPI.ListPortalDevelopers(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list portal developers", err, helper.GetCmd(), attrs...)
		}

		if res.GetListDevelopersResponse() == nil {
			break
		}

		data := res.GetListDevelopersResponse().GetData()
		all = append(all, data...)

		total := int(res.GetListDevelopersResponse().GetMeta().Page.Total)
		if total == 0 || len(all) >= total || len(data) == 0 {
			break
		}

		pageNumber++
	}

	return all, nil
}

func findDeveloperByEmailOrID(developers []kkComps.PortalDeveloper, identifier string) *kkComps.PortalDeveloper {
	lowered := strings.ToLower(identifier)
	for _, developer := range developers {
		if strings.ToLower(developer.GetID()) == lowered || strings.ToLower(developer.GetEmail()) == lowered {
			developerCopy := developer
			return &developerCopy
		}
	}
	return nil
}

func portalDeveloperSummaryToRecord(developer kkComps.PortalDeveloper) portalDeveloperSummaryRecord {
	return portalDeveloperSummaryRecord{
		ID:               util.AbbreviateUUID(developer.GetID()),
		Email:            developer.GetEmail(),
		FullName:         developer.GetFullName(),
		Status:           string(developer.GetStatus()),
		LocalCreatedTime: formatTime(developer.GetCreatedAt()),
		LocalUpdatedTime: formatTime(developer.GetUpdatedAt()),
	}
}

func portalDeveloperDetailView(developer kkComps.PortalDeveloper) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Email: %s\n", developer.GetEmail())
	fmt.Fprintf(&b, "ID: %s\n", developer.GetID())
	fmt.Fprintf(&b, "Full Name: %s\n", developer.GetFullName())
	fmt.Fprintf(&b, "Status: %s\n", string(developer.GetStatus()))
	fmt.Fprintf(&b, "Created: %s\n", formatTime(developer.GetCreatedAt()))
	fmt.Fprintf(&b, "Updated: %s\n", formatTime(developer.GetUpdatedAt()))

	return b.String()
}
