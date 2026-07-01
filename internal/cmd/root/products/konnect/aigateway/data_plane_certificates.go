package aigateway

import (
	"encoding/json"
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
	declresources "github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type aiGatewayDataPlaneCertificateRecord struct {
	ID               string
	Title            string
	Description      string
	LocalUpdatedTime string
}

var (
	aiGatewayDataPlaneCertificatesUse   = "data-plane-certificates [certificate-id|title]"
	aiGatewayDataPlaneCertificatesShort = i18n.T(
		"root.products.konnect.ai-gateway.data-plane-certificatesShort",
		"List or get data plane certificates for a Konnect AI Gateway",
	)
	aiGatewayDataPlaneCertificatesLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.data-plane-certificatesLong",
		`Use the data-plane-certificates command to list or retrieve data plane certificates for a specific `+
			`Konnect AI Gateway.`,
	))
	aiGatewayDataPlaneCertificatesExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.data-plane-certificatesExamples",
			fmt.Sprintf(`# List data plane certificates for an AI Gateway by display name
%[1]s get ai-gateway data-plane-certificates --gateway-name "Customer Support Gateway"
# List data plane certificates for an AI Gateway by ID
%[1]s get ai-gateway data-plane-certificates --gateway-id <gateway-id>
# Get a data plane certificate by title
%[1]s get ai-gateway data-plane-certificates --gateway-name "Customer Support Gateway" support-data-plane-cert
# Get a data plane certificate by ID
%[1]s get ai-gateway data-plane-certificates --gateway-id <gateway-id> --data-plane-certificate-id <certificate-id>
# Get a data plane certificate by title flag
%[1]s get ai-gateway data-plane-certificates --gateway-id <gateway-id> \
  --data-plane-certificate-title support-data-plane-cert
`, meta.CLIName)),
	)
)

func newGetAIGatewayDataPlaneCertificatesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayDataPlaneCertificatesUse,
		Short:   aiGatewayDataPlaneCertificatesShort,
		Long:    aiGatewayDataPlaneCertificatesLong,
		Example: aiGatewayDataPlaneCertificatesExample,
		Aliases: []string{"data-plane-certificate", "dpc", "dpcs", "dp-cert", "dp-certs"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayDataPlaneCertificateFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayDataPlaneCertificatesHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayDataPlaneCertificateFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayDataPlaneCertificatesHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayDataPlaneCertificatesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"too many arguments. Listing AI Gateway data plane certificates requires 0 or 1 arguments (ID or title)",
			),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		certID, certTitle := getAIGatewayDataPlaneCertificateIdentifiers(cfg)
		if certID != "" || certTitle != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayDataPlaneCertificateIDFlagName,
					aiGatewayDataPlaneCertificateTitleFlagName,
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

	gatewayID, gatewayName := getAIGatewayIdentifiers(cfg)
	if gatewayID != "" && gatewayName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", aiGatewayIDFlagName, aiGatewayNameFlagName),
		}
	}
	if gatewayID == "" && gatewayName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"an AI Gateway identifier is required. Provide --%s or --%s",
				aiGatewayIDFlagName,
				aiGatewayNameFlagName,
			),
		}
	}
	if gatewayID == "" {
		gatewayID, err = resolveAIGatewayIDByName(gatewayName, sdk.GetAIGatewayAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	certAPI := sdk.GetAIGatewayDataPlaneCertificatesAPI()
	if certAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway data plane certificates client is not available",
			Err: fmt.Errorf("AI Gateway data plane certificates client not configured"),
		}
	}

	certID, certTitle := getAIGatewayDataPlaneCertificateIdentifiers(cfg)
	if certID != "" && certTitle != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayDataPlaneCertificateIDFlagName,
				aiGatewayDataPlaneCertificateTitleFlagName,
			),
		}
	}

	identifier := ""
	forceTitleLookup := false
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else if certID != "" {
		identifier = certID
	} else if certTitle != "" {
		identifier = certTitle
		forceTitleLookup = true
	}

	if identifier != "" {
		return h.getSingleDataPlaneCertificate(
			helper,
			certAPI,
			gatewayID,
			identifier,
			forceTitleLookup,
			outType,
			printer,
			cfg,
		)
	}
	return h.listDataPlaneCertificates(helper, certAPI, gatewayID, outType, printer, cfg)
}

func (h aiGatewayDataPlaneCertificatesHandler) listDataPlaneCertificates(
	helper cmd.Helper,
	certAPI helpers.AIGatewayDataPlaneCertificatesAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	certs, err := fetchAIGatewayDataPlaneCertificates(helper, certAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayDataPlaneCertificateRecord, 0, len(certs))
	tableRows := make([]table.Row, 0, len(certs))
	for _, cert := range certs {
		record := aiGatewayDataPlaneCertificateToRecord(cert)
		records = append(records, record)
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Title,
			record.Description,
			record.LocalUpdatedTime,
		})
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		certs,
		"",
		tableview.WithCustomTable(
			[]string{aiGatewayHeaderID, "TITLE", "DESCRIPTION", aiGatewayHeaderUpdated},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index < 0 || index >= len(certs) {
				return ""
			}
			return aiGatewayDataPlaneCertificateDetailView(certs[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayDataPlaneCertificate, func(index int) any {
			if index < 0 || index >= len(certs) {
				return nil
			}
			return &certs[index]
		}),
	)
}

func (h aiGatewayDataPlaneCertificatesHandler) getSingleDataPlaneCertificate(
	helper cmd.Helper,
	certAPI helpers.AIGatewayDataPlaneCertificatesAPI,
	gatewayID string,
	identifier string,
	forceTitleLookup bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	certificateID := identifier
	if forceTitleLookup || !util.IsValidUUID(identifier) {
		certs, err := fetchAIGatewayDataPlaneCertificates(helper, certAPI, gatewayID, cfg)
		if err != nil {
			return err
		}
		match := findAIGatewayDataPlaneCertificateByTitleOrID(certs, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway data plane certificate %q not found", identifier),
			}
		}
		certificateID = declresources.AIGatewayDataPlaneCertificateID(*match)
		if certificateID == "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway data plane certificate %q does not have an ID", identifier),
			}
		}
	}

	res, err := certAPI.GetAiGatewayDataPlaneCertificate(helper.GetContext(), gatewayID, certificateID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError(
			"Failed to get AI Gateway data plane certificate",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}
	if res == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway data plane certificate response was empty",
			Err: fmt.Errorf("no data plane certificate returned for id or title %s", identifier),
		}
	}
	cert := res.GetAIGatewayDataPlaneClientCertificate()
	if cert == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway data plane certificate response was empty",
			Err: fmt.Errorf("no data plane certificate returned for id or title %s", identifier),
		}
	}

	record := aiGatewayDataPlaneCertificateToRecord(*cert)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		cert,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index != 0 {
				return ""
			}
			return aiGatewayDataPlaneCertificateDetailView(*cert)
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayDataPlaneCertificate, func(index int) any {
			if index != 0 {
				return nil
			}
			return cert
		}),
	)
}

func fetchAIGatewayDataPlaneCertificates(
	helper cmd.Helper,
	certAPI helpers.AIGatewayDataPlaneCertificatesAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayDataPlaneClientCertificate, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayDataPlaneClientCertificate

	for {
		req := kkOps.ListAiGatewayDataPlaneCertificatesRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := certAPI.ListAiGatewayDataPlaneCertificates(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError(
				"Failed to list AI Gateway data plane certificates",
				err,
				helper.GetCmd(),
				attrs...,
			)
		}
		if res.GetListAIGatewayDataPlaneCertificatesResponse() == nil {
			break
		}

		allData = append(allData, res.GetListAIGatewayDataPlaneCertificatesResponse().Data...)
		nextCursor := pagination.ExtractPageAfterCursor(
			res.GetListAIGatewayDataPlaneCertificatesResponse().Meta.Page.Next,
		)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func aiGatewayDataPlaneCertificateToRecord(
	cert kkComps.AIGatewayDataPlaneClientCertificate,
) aiGatewayDataPlaneCertificateRecord {
	record := aiGatewayDataPlaneCertificateRecord{
		ID:               aiGatewayMissingValue,
		Title:            valueOrMissing(cert.Title),
		Description:      aiGatewayMissingValue,
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if id := declresources.AIGatewayDataPlaneCertificateID(cert); id != "" {
		record.ID = util.AbbreviateUUID(id)
	}
	if cert.Description != nil {
		record.Description = valueOrMissing(*cert.Description)
	}
	if !cert.UpdatedAt.IsZero() {
		record.LocalUpdatedTime = cert.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func findAIGatewayDataPlaneCertificateByTitleOrID(
	certs []kkComps.AIGatewayDataPlaneClientCertificate,
	identifier string,
) *kkComps.AIGatewayDataPlaneClientCertificate {
	for i := range certs {
		if declresources.AIGatewayDataPlaneCertificateID(certs[i]) == identifier ||
			declresources.AIGatewayDataPlaneCertificateTitle(certs[i]) == identifier {
			return &certs[i]
		}
	}
	return nil
}

func aiGatewayDataPlaneCertificateDetailView(cert kkComps.AIGatewayDataPlaneClientCertificate) string {
	payload := make(map[string]any)
	data, err := json.Marshal(cert)
	if err == nil {
		// Detail views are best-effort; leave missing fields as n/a if SDK union data cannot round-trip.
		_ = json.Unmarshal(data, &payload)
	}

	order := []string{
		"id",
		"title",
		"description",
		"cert",
		"metadata",
		aiGatewayFieldCreatedAt,
		aiGatewayFieldUpdatedAt,
	}

	var b strings.Builder
	for _, field := range order {
		fmt.Fprintf(&b, "%s: %s\n", field, formatAIGatewayModelDetailValue(payload[field]))
	}
	return strings.TrimRight(b.String(), "\n")
}

func buildAIGatewayDataPlaneCertificateChildView(
	certs []kkComps.AIGatewayDataPlaneClientCertificate,
) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(certs))
	for _, cert := range certs {
		record := aiGatewayDataPlaneCertificateToRecord(cert)
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Title,
			record.Description,
			record.LocalUpdatedTime,
		})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(certs) {
			return ""
		}
		return aiGatewayDataPlaneCertificateDetailView(certs[index])
	}

	return tableview.ChildView{
		Headers:        []string{aiGatewayHeaderID, "TITLE", "DESCRIPTION", aiGatewayHeaderUpdated},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "AI Gateway Data Plane Certificates",
		ParentType:     common.ViewParentAIGatewayDataPlaneCertificate,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(certs) {
				return nil
			}
			return &certs[index]
		},
	}
}
