package api

import (
	"fmt"
	"sort"
	"strings"
	"time"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"
	"github.com/kong/kongctl/internal/cmd"
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
	publicationsCommandName = "publications"
)

type apiPublicationRecord struct {
	PortalID         string
	Visibility       string
	AuthStrategyIDs  string
	LocalCreatedTime string
	LocalUpdatedTime string
}

var (
	publicationsUse = publicationsCommandName

	publicationsShort = i18n.T("root.products.konnect.api.publicationsShort",
		"Manage API publications for a Konnect API")
	publicationsLong = normalizers.LongDesc(i18n.T("root.products.konnect.api.publicationsLong",
		`Use the publications command to list API publications for a specific Konnect API.`))
	publicationsExample = normalizers.Examples(
		i18n.T("root.products.konnect.api.publicationsExamples",
			fmt.Sprintf(`
# List publications for an API by ID
%[1]s get api publications --api-id <api-id>
# List publications for an API by name
%[1]s get api publications --api-name my-api
# Get a publication for a specific portal ID
%[1]s get api publications --api-id <api-id> <portal-id>
`, meta.CLIName)))
)

func newGetAPIPublicationsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     publicationsUse,
		Short:   publicationsShort,
		Long:    publicationsLong,
		Example: publicationsExample,
		Aliases: []string{"publication", "pubs", "pub"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindAPIChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := apiPublicationsHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addAPIChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type apiPublicationsHandler struct {
	cmd *cobra.Command
}

func (h apiPublicationsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing API publications requires 0 or 1 arguments (portal ID)"),
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

	publicationAPI := sdk.GetAPIPublicationAPI()
	if publicationAPI == nil {
		return &cmd.ExecutionError{
			Msg: "API publications client is not available",
			Err: fmt.Errorf("api publications client not configured"),
		}
	}

	publications, err := fetchPublications(helper, publicationAPI, apiID, cfg)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		portalID := strings.TrimSpace(args[0])
		filtered := filterPublicationsByPortal(publications, portalID)
		if len(filtered) == 0 {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("publication for portal %q not found", portalID),
			}
		}
		publications = filtered
	}

	records := make([]apiPublicationRecord, 0, len(publications))
	rows := make([]table.Row, 0, len(publications))
	for _, publication := range publications {
		record := publicationToRecord(publication)
		records = append(records, record)
		rows = append(rows, table.Row{util.AbbreviateUUID(record.PortalID), strings.ToUpper(record.Visibility)})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(publications) {
			return ""
		}
		return publicationDetailView(&publications[index])
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		publications,
		"",
		tableview.WithTitle("Publications"),
		tableview.WithCustomTable([]string{"PORTAL", "VISIBILITY"}, rows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailContext("api-publication", func(index int) any {
			if index < 0 || index >= len(publications) {
				return nil
			}
			return &publications[index]
		}),
		tableview.WithDetailHelper(helper),
	)
}

func fetchPublications(
	helper cmd.Helper,
	publicationAPI helpers.APIPublicationAPI,
	apiID string,
	cfg config.Hook,
) ([]kkComps.APIPublicationListItem, error) {
	var pageNumber int64 = 1
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}

	var all []kkComps.APIPublicationListItem

	filter := &kkComps.APIPublicationFilterParameters{
		APIID: &kkComps.UUIDFieldFilter{Eq: kk.String(apiID)},
	}

	for {
		req := kkOps.ListAPIPublicationsRequest{
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
			Filter:     filter,
		}

		res, err := publicationAPI.ListAPIPublications(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list API publications", err, helper.GetCmd(), attrs...)
		}

		if res.GetListAPIPublicationResponse() == nil {
			break
		}

		data := res.GetListAPIPublicationResponse().GetData()
		all = append(all, data...)

		total := int(res.GetListAPIPublicationResponse().GetMeta().Page.Total)
		if total == 0 || len(all) >= total || len(data) == 0 {
			break
		}

		pageNumber++
	}

	return all, nil
}

func filterPublicationsByPortal(
	publications []kkComps.APIPublicationListItem,
	portalID string,
) []kkComps.APIPublicationListItem {
	if util.IsValidUUID(portalID) {
		matches := make([]kkComps.APIPublicationListItem, 0, 1)
		for _, publication := range publications {
			if publication.GetPortalID() == portalID {
				matches = append(matches, publication)
			}
		}
		return matches
	}

	// Fall back to case-insensitive match against portal IDs in case users pass non-UUID identifiers.
	lowered := strings.ToLower(portalID)
	matches := make([]kkComps.APIPublicationListItem, 0)
	for _, publication := range publications {
		if strings.ToLower(publication.GetPortalID()) == lowered {
			matches = append(matches, publication)
		}
	}
	return matches
}

func publicationToRecord(publication kkComps.APIPublicationListItem) apiPublicationRecord {
	visibility := "n/a"
	if publication.GetVisibility() != nil {
		visibility = string(*publication.GetVisibility())
	}

	authStrategies := "n/a"
	if ids := publication.GetAuthStrategyIds(); len(ids) > 0 {
		authStrategies = strings.Join(ids, ", ")
	}

	return apiPublicationRecord{
		PortalID:         publication.GetPortalID(),
		Visibility:       visibility,
		AuthStrategyIDs:  authStrategies,
		LocalCreatedTime: publication.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: publication.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func publicationDetailView(publication *kkComps.APIPublicationListItem) string {
	if publication == nil {
		return ""
	}

	visibility := valueNA
	if publication.GetVisibility() != nil && *publication.GetVisibility() != "" {
		visibility = string(*publication.GetVisibility())
	}

	authStrategies := valueNA
	if ids := publication.GetAuthStrategyIds(); len(ids) > 0 {
		authStrategies = strings.Join(ids, ", ")
	}

	fields := map[string]string{
		"auth_strategy_ids": authStrategies,
		"created_at":        publication.GetCreatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
		"visibility":        visibility,
		"updated_at":        publication.GetUpdatedAt().In(time.Local).Format("2006-01-02 15:04:05"),
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "api_id: %s\n", publication.GetAPIID())
	fmt.Fprintf(&b, "portal_id: %s\n", publication.GetPortalID())
	for _, key := range keys {
		fmt.Fprintf(&b, "%s: %s\n", key, fields[key])
	}

	return strings.TrimRight(b.String(), "\n")
}
