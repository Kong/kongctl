package eventgateway

import (
	"fmt"
	"net/url"
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
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	dataPlaneCertificatesCommandName = "data-plane-certificates"
)

type dataPlaneCertSummaryRecord struct {
	ID               string
	Name             string
	Description      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

var (
	dataPlaneCertificatesUse = dataPlaneCertificatesCommandName

	dataPlaneCertificatesShort = i18n.T("root.products.konnect.eventgateway.dataPlaneCertificatesShort",
		"Manage data plane certificates for an Event Gateway")
	dataPlaneCertificatesLong = normalizers.LongDesc(
		i18n.T(
			"root.products.konnect.eventgateway.dataPlaneCertificatesLong",
			`Use the data-plane-certificates command to list or retrieve data plane certificates for a specific Event Gateway.`,
		),
	)
	dataPlaneCertificatesExample = normalizers.Examples(
		i18n.T("root.products.konnect.eventgateway.dataPlaneCertificatesExamples",
			fmt.Sprintf(`
# List data plane certificates for an event gateway by ID
%[1]s get event-gateway data-plane-certificates --gateway-id <gateway-id>
# List data plane certificates for an event gateway by name
%[1]s get event-gateway data-plane-certificates --gateway-name my-gateway
# Get a specific data plane certificate by ID (positional argument)
%[1]s get event-gateway data-plane-certificates --gateway-id <gateway-id> <certificate-id>
# Get a specific data plane certificate by name (positional argument)
%[1]s get event-gateway data-plane-certificates --gateway-id <gateway-id> my-certificate
# Get a specific data plane certificate by ID (flag)
%[1]s get event-gateway data-plane-certificates --gateway-id <gateway-id> --data-plane-certificate-id <certificate-id>
# Get a specific data plane certificate by name (flag)
%[1]s get event-gateway data-plane-certificates --gateway-name my-gateway --data-plane-certificate-name my-certificate
`, meta.CLIName)))
)

func newGetEventGatewayDataPlaneCertificatesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     dataPlaneCertificatesUse,
		Short:   dataPlaneCertificatesShort,
		Long:    dataPlaneCertificatesLong,
		Example: dataPlaneCertificatesExample,
		Aliases: []string{"data-plane-certificate", "dpc", "dpcs", "dp-cert", "dp-certs"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			if err := bindEventGatewayChildFlags(cmd, args); err != nil {
				return err
			}
			return bindDataPlaneCertChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := dataPlaneCertificatesHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addEventGatewayChildFlags(cmd)
	addDataPlaneCertChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type dataPlaneCertificatesHandler struct {
	cmd *cobra.Command
}

func (h dataPlaneCertificatesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"too many arguments. Listing data plane certificates requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	// Check if positional arg and flags are both provided
	if len(args) == 1 {
		certID, certName := getDataPlaneCertIdentifiers(cfg)
		if certID != "" || certName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					dataPlaneCertIDFlagName,
					dataPlaneCertNameFlagName,
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

	certAPI := sdk.GetEventGatewayDataPlaneCertificateAPI()
	if certAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Data plane certificates client is not available",
			Err: fmt.Errorf("data plane certificates client not configured"),
		}
	}

	// Determine if we're getting a single certificate or listing all
	certID, certName := getDataPlaneCertIdentifiers(cfg)
	var certIdentifier string

	if len(args) == 1 {
		certIdentifier = strings.TrimSpace(args[0])
	} else if certID != "" {
		certIdentifier = certID
	} else if certName != "" {
		certIdentifier = certName
	}

	// Validate mutual exclusivity of certificate ID and name flags
	if certID != "" && certName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				dataPlaneCertIDFlagName,
				dataPlaneCertNameFlagName,
			),
		}
	}

	if certIdentifier != "" {
		return h.getSingleDataPlaneCert(
			helper,
			certAPI,
			gatewayID,
			certIdentifier,
			outType,
			printer,
			cfg,
		)
	}

	return h.listDataPlaneCerts(helper, certAPI, gatewayID, outType, printer, cfg)
}

func (h dataPlaneCertificatesHandler) listDataPlaneCerts(
	helper cmd.Helper,
	certAPI helpers.EventGatewayDataPlaneCertificateAPI,
	gatewayID string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	certs, err := fetchDataPlaneCertificates(helper, certAPI, gatewayID, cfg, "")
	if err != nil {
		return err
	}

	records := make([]dataPlaneCertSummaryRecord, 0, len(certs))
	for _, cert := range certs {
		records = append(records, dataPlaneCertToRecord(cert))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
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
		tableview.WithCustomTable([]string{"ID", "NAME"}, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h dataPlaneCertificatesHandler) getSingleDataPlaneCert(
	helper cmd.Helper,
	certAPI helpers.EventGatewayDataPlaneCertificateAPI,
	gatewayID string,
	identifier string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	certID := identifier
	if !util.IsValidUUID(identifier) {
		// Use name filter to optimize the API query
		certs, err := fetchDataPlaneCertificates(helper, certAPI, gatewayID, cfg, identifier)
		if err != nil {
			return err
		}
		match := findDataPlaneCertByName(certs, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("data plane certificate %q not found", identifier),
			}
		}
		if match.ID != "" {
			certID = match.ID
		} else {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("data plane certificate %q does not have an ID", identifier),
			}
		}
	}

	res, err := certAPI.FetchEventGatewayDataPlaneCertificate(helper.GetContext(), gatewayID, certID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get data plane certificate", err, helper.GetCmd(), attrs...)
	}

	cert := res.GetEventGatewayDataPlaneCertificate()
	if cert == nil {
		return &cmd.ExecutionError{
			Msg: "Data plane certificate response was empty",
			Err: fmt.Errorf("no data plane certificate returned for id %s", certID),
		}
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		dataPlaneCertToRecord(*cert),
		cert,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func fetchDataPlaneCertificates(
	helper cmd.Helper,
	certAPI helpers.EventGatewayDataPlaneCertificateAPI,
	gatewayID string,
	cfg config.Hook,
	nameFilter string,
) ([]kkComps.EventGatewayDataPlaneCertificate, error) {
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.EventGatewayDataPlaneCertificate
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewayDataPlaneCertificatesRequest{
			GatewayID: gatewayID,
			PageSize:  new(requestPageSize),
		}

		// Apply name filter if provided
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

		res, err := certAPI.ListEventGatewayDataPlaneCertificates(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError(
				"Failed to list data plane certificates", err, helper.GetCmd(), attrs...)
		}

		if res.GetListEventGatewayDataPlaneCertificatesResponse() == nil {
			break
		}

		data := res.GetListEventGatewayDataPlaneCertificatesResponse().Data
		allData = append(allData, data...)

		if res.GetListEventGatewayDataPlaneCertificatesResponse().Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.GetListEventGatewayDataPlaneCertificatesResponse().Meta.Page.Next)
		if err != nil {
			return nil, cmd.PrepareExecutionError(
				"Failed to list data plane certificates: invalid cursor",
				err,
				helper.GetCmd(),
			)
		}

		values := u.Query()
		pageAfter = new(values.Get("page[after]"))
	}

	return allData, nil
}

func findDataPlaneCertByName(
	certs []kkComps.EventGatewayDataPlaneCertificate,
	name string,
) *kkComps.EventGatewayDataPlaneCertificate {
	lowered := strings.ToLower(name)
	for _, cert := range certs {
		if cert.Name != nil && strings.ToLower(*cert.Name) == lowered {
			certCopy := cert
			return &certCopy
		}
	}
	return nil
}

func dataPlaneCertToRecord(cert kkComps.EventGatewayDataPlaneCertificate) dataPlaneCertSummaryRecord {
	id := cert.ID
	if id != "" {
		id = util.AbbreviateUUID(id)
	} else {
		id = valueNA
	}

	name := valueNA
	if cert.Name != nil && *cert.Name != "" {
		name = *cert.Name
	}

	description := valueNA
	if cert.Description != nil && *cert.Description != "" {
		description = *cert.Description
	}

	createdAt := cert.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	updatedAt := cert.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return dataPlaneCertSummaryRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

func dataPlaneCertDetailView(cert *kkComps.EventGatewayDataPlaneCertificate) string {
	if cert == nil {
		return ""
	}

	id := strings.TrimSpace(cert.ID)
	if id == "" {
		id = valueNA
	}

	name := valueNA
	if cert.Name != nil && strings.TrimSpace(*cert.Name) != "" {
		name = strings.TrimSpace(*cert.Name)
	}

	description := valueNA
	if cert.Description != nil && strings.TrimSpace(*cert.Description) != "" {
		description = strings.TrimSpace(*cert.Description)
	}

	certificate := valueNA
	if strings.TrimSpace(cert.Certificate) != "" {
		certificate = cert.Certificate
	}

	// Format certificate metadata
	metadataStr := formatCertificateMetadata(cert.Metadata)

	createdAt := cert.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := cert.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", description)
	fmt.Fprintf(&b, "certificate: %s\n", certificate)
	fmt.Fprintf(&b, "metadata:\n%s\n", metadataStr)
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)

	return strings.TrimRight(b.String(), "\n")
}

func formatCertificateMetadata(meta *kkComps.CertificateMetadata) string {
	if meta == nil {
		return "  " + valueNA
	}

	var lines []string

	if meta.Subject != nil && *meta.Subject != "" {
		lines = append(lines, fmt.Sprintf("  subject: %s", *meta.Subject))
	}

	if meta.Issuer != nil && *meta.Issuer != "" {
		lines = append(lines, fmt.Sprintf("  issuer: %s", *meta.Issuer))
	}

	if meta.Expiry != nil {
		expiryTime := time.Unix(*meta.Expiry, 0)
		lines = append(lines, fmt.Sprintf("  expiry: %s", expiryTime.Format("2006-01-02 15:04:05")))
	}

	if meta.Sha256Fingerprint != nil && *meta.Sha256Fingerprint != "" {
		lines = append(lines, fmt.Sprintf("  sha256_fingerprint: %s", *meta.Sha256Fingerprint))
	}

	if len(meta.KeyUsages) > 0 {
		lines = append(lines, fmt.Sprintf("  key_usages: [%s]", strings.Join(meta.KeyUsages, ", ")))
	}

	if len(meta.DNSNames) > 0 {
		lines = append(lines, fmt.Sprintf("  dns_names: [%s]", strings.Join(meta.DNSNames, ", ")))
	}

	if len(meta.SanNames) > 0 {
		lines = append(lines, fmt.Sprintf("  san_names: [%s]", strings.Join(meta.SanNames, ", ")))
	}

	if len(meta.EmailAddresses) > 0 {
		lines = append(lines, fmt.Sprintf("  email_addresses: [%s]", strings.Join(meta.EmailAddresses, ", ")))
	}

	if len(meta.IPAddresses) > 0 {
		lines = append(lines, fmt.Sprintf("  ip_addresses: [%s]", strings.Join(meta.IPAddresses, ", ")))
	}

	if len(meta.Uris) > 0 {
		lines = append(lines, fmt.Sprintf("  uris: [%s]", strings.Join(meta.Uris, ", ")))
	}

	if len(lines) == 0 {
		return "  " + valueNA
	}

	return strings.Join(lines, "\n")
}

func buildDataPlaneCertChildView(certs []kkComps.EventGatewayDataPlaneCertificate) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(certs))
	for i := range certs {
		record := dataPlaneCertToRecord(certs[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(certs) {
			return ""
		}
		return dataPlaneCertDetailView(&certs[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Data Plane Certificates",
		ParentType:     "data-plane-certificate",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(certs) {
				return nil
			}
			return &certs[index]
		},
	}
}
