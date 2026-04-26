package eventgateway

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
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
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	tlsTrustBundlesCommandName = "tls-trust-bundles"
)

type tlsTrustBundleSummaryRecord struct {
	ID               string
	Name             string
	Description      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

var (
	tlsTrustBundlesUse = tlsTrustBundlesCommandName

	tlsTrustBundlesShort = i18n.T("root.products.konnect.eventgateway.tlsTrustBundlesShort",
		"Manage TLS trust bundles for an Event Gateway")
	tlsTrustBundlesLong = normalizers.LongDesc(
		i18n.T(
			"root.products.konnect.eventgateway.tlsTrustBundlesLong",
			`Use the tls-trust-bundles command to list or retrieve TLS trust bundles for a specific Event Gateway.

TLS trust bundles define trusted certificate authorities used for mTLS client certificate
verification and are referenced by TLS listener policies.`,
		),
	)
	tlsTrustBundlesExample = normalizers.Examples(
		i18n.T("root.products.konnect.eventgateway.tlsTrustBundlesExamples",
			fmt.Sprintf(`
# List TLS trust bundles for an event gateway by name
%[1]s get event-gateway tls-trust-bundles --gateway-name my-gateway
# List TLS trust bundles for an event gateway by ID
%[1]s get event-gateway tls-trust-bundles --gateway-id <gateway-id>
# Get a specific TLS trust bundle by name (positional argument)
%[1]s get event-gateway tls-trust-bundles --gateway-name my-gateway my-bundle
# Get a specific TLS trust bundle by ID (positional argument)
%[1]s get event-gateway tls-trust-bundles --gateway-id <gateway-id> <bundle-id>
# Get a specific TLS trust bundle by name (flag)
%[1]s get event-gateway tls-trust-bundles --gateway-name my-gateway --tls-trust-bundle-name my-bundle
# Get a specific TLS trust bundle by ID (flag)
%[1]s get event-gateway tls-trust-bundles --gateway-id <gateway-id> --tls-trust-bundle-id <bundle-id>
`, meta.CLIName)))
)

func newGetEventGatewayTLSTrustBundlesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     tlsTrustBundlesUse,
		Short:   tlsTrustBundlesShort,
		Long:    tlsTrustBundlesLong,
		Example: tlsTrustBundlesExample,
		Aliases: []string{"tls-trust-bundle", "ttb"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindEventGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindTLSTrustBundleChildFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := tlsTrustBundlesHandler{cmd: c}
			return handler.run(args)
		},
	}

	addEventGatewayChildFlags(c)
	addTLSTrustBundleChildFlags(c)

	if addParentFlags != nil {
		addParentFlags(verb, c)
	}

	return c
}

type tlsTrustBundlesHandler struct {
	cmd *cobra.Command
}

func (h tlsTrustBundlesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"too many arguments. Listing TLS trust bundles requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	// Check if positional arg and flags are both provided
	if len(args) == 1 {
		tbID, tbName := getTLSTrustBundleIdentifiers(cfg)
		if tbID != "" || tbName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					tlsTrustBundleIDFlagName,
					tlsTrustBundleNameFlagName,
				),
			}
		}
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

	gatewayID, gatewayName := getEventGatewayIdentifiers(cfg)
	if gatewayID != "" && gatewayName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", gatewayIDFlagName, gatewayNameFlagName),
		}
	}

	if gatewayID == "" && gatewayName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"an event gateway identifier is required. Provide --%s or --%s",
				gatewayIDFlagName,
				gatewayNameFlagName,
			),
		}
	}

	if gatewayID == "" {
		gatewayID, err = resolveEventGatewayIDByName(gatewayName, sdk.GetEventGatewayControlPlaneAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	bundleAPI := sdk.GetEventGatewayTLSTrustBundleAPI()
	if bundleAPI == nil {
		return &cmd.ExecutionError{
			Msg: "TLS trust bundle client is not available",
			Err: fmt.Errorf("TLS trust bundle client not configured"),
		}
	}

	// Validate mutual exclusivity of bundle ID and name flags
	tbID, tbName := getTLSTrustBundleIdentifiers(cfg)
	if tbID != "" && tbName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				tlsTrustBundleIDFlagName,
				tlsTrustBundleNameFlagName,
			),
		}
	}

	var tbIdentifier string
	if len(args) == 1 {
		tbIdentifier = strings.TrimSpace(args[0])
	} else if tbID != "" {
		tbIdentifier = tbID
	} else if tbName != "" {
		tbIdentifier = tbName
	}

	if tbIdentifier != "" {
		return h.getSingleTLSTrustBundle(helper, bundleAPI, gatewayID, tbIdentifier, outType, printer, cfg)
	}

	return h.listTLSTrustBundles(helper, bundleAPI, gatewayID, outType, printer, cfg)
}

func (h tlsTrustBundlesHandler) listTLSTrustBundles(
	helper cmd.Helper,
	bundleAPI helpers.EventGatewayTLSTrustBundleAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	bundles, err := fetchTLSTrustBundles(helper, bundleAPI, gatewayID, cfg, "")
	if err != nil {
		return err
	}

	records := make([]tlsTrustBundleSummaryRecord, 0, len(bundles))
	for _, tb := range bundles {
		records = append(records, tlsTrustBundleToRecord(tb))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Description})
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		bundles,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME", "DESCRIPTION"}, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h tlsTrustBundlesHandler) getSingleTLSTrustBundle(
	helper cmd.Helper,
	bundleAPI helpers.EventGatewayTLSTrustBundleAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	bundleID := identifier
	if !util.IsValidUUID(identifier) {
		// Resolve name to ID by listing and matching exactly
		bundles, err := fetchTLSTrustBundles(helper, bundleAPI, gatewayID, cfg, identifier)
		if err != nil {
			return err
		}
		match := findTLSTrustBundleByName(bundles, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("TLS trust bundle %q not found", identifier),
			}
		}
		bundleID = match.ID
	}

	res, err := bundleAPI.GetEventGatewayTLSTrustBundle(helper.GetContext(),
		kkOps.GetEventGatewayTLSTrustBundleRequest{
			GatewayID:        gatewayID,
			TLSTrustBundleID: bundleID,
		})
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get TLS trust bundle", err, helper.GetCmd(), attrs...)
	}

	if res.GetTLSTrustBundle() == nil {
		return &cmd.ExecutionError{
			Msg: "TLS trust bundle response was empty",
			Err: fmt.Errorf("no TLS trust bundle returned for id %s", bundleID),
		}
	}

	tb := res.GetTLSTrustBundle()

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		tlsTrustBundleToRecord(*tb),
		tb,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func fetchTLSTrustBundles(
	helper cmd.Helper,
	bundleAPI helpers.EventGatewayTLSTrustBundleAPI,
	gatewayID string,
	cfg config.Hook,
	nameFilter string,
) ([]kkComps.TLSTrustBundle, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)

	var allBundles []kkComps.TLSTrustBundle
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewayTLSTrustBundlesRequest{
			GatewayID: gatewayID,
			PageSize:  new(requestPageSize),
		}

		if nameFilter != "" {
			req.Filter = &kkComps.EventGatewayCommonFilter{
				Name: &kkComps.StringFieldContainsFilter{
					Contains: nameFilter,
				},
			}
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := bundleAPI.ListEventGatewayTLSTrustBundles(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list TLS trust bundles", err, helper.GetCmd(), attrs...)
		}

		if res.GetListTLSTrustBundlesResponse() == nil {
			break
		}

		allBundles = append(allBundles, res.GetListTLSTrustBundlesResponse().Data...)

		if res.GetListTLSTrustBundlesResponse().Meta == nil ||
			res.GetListTLSTrustBundlesResponse().Meta.Page.Next == nil {
			break
		}

		nextCursor := pagination.ExtractPageAfterCursor(res.GetListTLSTrustBundlesResponse().Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allBundles, nil
}

func findTLSTrustBundleByName(
	bundles []kkComps.TLSTrustBundle,
	name string,
) *kkComps.TLSTrustBundle {
	lowered := strings.ToLower(name)
	for i := range bundles {
		if strings.ToLower(bundles[i].Name) == lowered {
			return &bundles[i]
		}
	}
	return nil
}

func tlsTrustBundleToRecord(tb kkComps.TLSTrustBundle) tlsTrustBundleSummaryRecord {
	id := tb.ID
	if id != "" {
		id = util.AbbreviateUUID(id)
	} else {
		id = valueNA
	}

	name := tb.Name
	if name == "" {
		name = valueNA
	}

	description := valueNA
	if tb.Description != nil && *tb.Description != "" {
		description = *tb.Description
	}

	createdAt := tb.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := tb.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return tlsTrustBundleSummaryRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

func tlsTrustBundleDetailView(tb *kkComps.TLSTrustBundle) string {
	if tb == nil {
		return ""
	}

	id := strings.TrimSpace(tb.ID)
	if id == "" {
		id = valueNA
	}

	name := tb.Name
	if name == "" {
		name = valueNA
	}

	description := valueNA
	if tb.Description != nil && strings.TrimSpace(*tb.Description) != "" {
		description = strings.TrimSpace(*tb.Description)
	}

	trustedCa := strings.TrimSpace(tb.Config.TrustedCa)
	if trustedCa == "" {
		trustedCa = valueNA
	}

	createdAt := tb.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := tb.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", description)
	fmt.Fprintf(&b, "config.trusted_ca: %s\n", trustedCa)
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)

	return strings.TrimRight(b.String(), "\n")
}

func buildTLSTrustBundleChildView(bundles []kkComps.TLSTrustBundle) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(bundles))
	for i := range bundles {
		record := tlsTrustBundleToRecord(bundles[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Description})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(bundles) {
			return ""
		}
		return tlsTrustBundleDetailView(&bundles[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME", "DESCRIPTION"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "TLS Trust Bundles",
		ParentType:     common.ViewParentTLSTrustBundle,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(bundles) {
				return nil
			}
			return &bundles[index]
		},
	}
}
