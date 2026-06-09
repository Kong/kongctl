package dataplanecertificate

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	gatewayCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type getDataPlaneCertificateCmd struct {
	*cobra.Command
}

type dataPlaneCertificateRecord struct {
	ID          string `json:"id"          yaml:"id"`
	Fingerprint string `json:"fingerprint" yaml:"fingerprint"`
	CreatedAt   string `json:"created_at"   yaml:"created_at"`
	UpdatedAt   string `json:"updated_at"   yaml:"updated_at"`
}

var (
	getDataPlaneCertificateShort = i18n.T("root.products.konnect.gateway.data-plane-certificate.get.short",
		"List or get Konnect Kong Gateway data plane certificates")
	getDataPlaneCertificateLong = i18n.T("root.products.konnect.gateway.data-plane-certificate.get.long",
		`Use the get verb with the data-plane-certificates command to query Konnect Kong Gateway data plane certificates.`)
	getDataPlaneCertificateExamples = normalizers.Examples(i18n.T(
		"root.products.konnect.gateway.data-plane-certificate.get.examples",
		fmt.Sprintf(`
	# List all data plane certificates for a given control plane
	%[1]s get konnect gateway control-plane data-plane-certificates --control-plane-id <id>
	# Get a specific data plane certificate for a given control plane
	%[1]s get konnect gateway control-plane data-plane-certificates --control-plane-id <id> <certificate-id>
	`, meta.CLIName),
	))
)

func (c *getDataPlaneCertificateCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing data plane certificates requires 0 or 1 arguments (ID)"),
		}
	}

	if len(helper.GetArgs()) == 1 && !util.IsValidUUID(helper.GetArgs()[0]) {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("data plane certificate ID must be a UUID"),
		}
	}

	return nil
}

func (c *getDataPlaneCertificateCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if err := c.validate(helper); err != nil {
		return err
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

	kkClient, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	controlPlaneID, err := resolveControlPlaneID(helper, cfg, kkClient)
	if err != nil {
		return err
	}

	certAPI := kkClient.GetDataPlaneCertificateAPI()
	if certAPI == nil {
		return &cmd.ConfigurationError{Err: fmt.Errorf("data plane certificates client not configured")}
	}

	if len(helper.GetArgs()) == 1 {
		return c.runGet(controlPlaneID, helper.GetArgs()[0], certAPI, helper, printer, outType)
	}
	return c.runList(controlPlaneID, certAPI, helper, printer, outType)
}

func resolveControlPlaneID(helper cmd.Helper, cfg config.Hook, kkClient helpers.SDKAPI) (string, error) {
	controlPlaneID := cfg.GetString(gatewayCommon.ControlPlaneIDConfigPath)
	if controlPlaneID != "" {
		return controlPlaneID, nil
	}

	controlPlaneName := cfg.GetString(gatewayCommon.ControlPlaneNameConfigPath)
	if controlPlaneName == "" {
		return "", &cmd.ConfigurationError{Err: fmt.Errorf("control plane ID or name is required")}
	}

	controlPlaneID, err := helpers.GetControlPlaneID(helper.GetContext(), kkClient.GetControlPlaneAPI(), controlPlaneName)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return "", cmd.PrepareExecutionError("Failed to get Control Plane ID", err, helper.GetCmd(), attrs...)
	}
	return controlPlaneID, nil
}

func (c *getDataPlaneCertificateCmd) runGet(
	controlPlaneID string,
	certificateID string,
	certAPI helpers.DataPlaneCertificateAPI,
	helper cmd.Helper,
	printer cli.PrintFlusher,
	outputFormat cmdCommon.OutputFormat,
) error {
	res, err := certAPI.GetDataplaneCertificate(helper.GetContext(), controlPlaneID, certificateID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get data plane certificate", err, helper.GetCmd(), attrs...)
	}
	body := res.GetDataPlaneClientCertificateResponse()
	if body == nil || body.GetItem() == nil {
		return &cmd.ExecutionError{
			Msg: "Data plane certificate response was empty",
			Err: fmt.Errorf("no data plane certificate returned for id %s", certificateID),
		}
	}
	cert := body.GetItem()

	return tableview.RenderForFormat(
		helper,
		false,
		outputFormat,
		printer,
		helper.GetStreams(),
		dataPlaneCertificateToRecord(cert),
		cert,
		"Data Plane Certificate",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (c *getDataPlaneCertificateCmd) runList(
	controlPlaneID string,
	certAPI helpers.DataPlaneCertificateAPI,
	helper cmd.Helper,
	printer cli.PrintFlusher,
	outputFormat cmdCommon.OutputFormat,
) error {
	res, err := certAPI.ListDpClientCertificates(helper.GetContext(), controlPlaneID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to list data plane certificates", err, helper.GetCmd(), attrs...)
	}

	var certs []kkComps.DataPlaneClientCertificate
	if res != nil && res.GetListDataPlaneCertificatesResponse() != nil {
		certs = res.GetListDataPlaneCertificatesResponse().GetItems()
	}
	records := make([]dataPlaneCertificateRecord, 0, len(certs))
	rows := make([]table.Row, 0, len(certs))
	for i := range certs {
		record := dataPlaneCertificateToRecord(&certs[i])
		records = append(records, record)
		rows = append(rows, table.Row{record.ID, record.Fingerprint, record.CreatedAt, record.UpdatedAt})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(certs) {
			return ""
		}
		return dataPlaneCertificateDetailView(&certs[index])
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outputFormat,
		printer,
		helper.GetStreams(),
		records,
		certs,
		"",
		tableview.WithCustomTable([]string{"ID", "FINGERPRINT", "CREATED_AT", "UPDATED_AT"}, rows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func dataPlaneCertificateToRecord(cert *kkComps.DataPlaneClientCertificate) dataPlaneCertificateRecord {
	if cert == nil {
		return dataPlaneCertificateRecord{}
	}

	certValue := ""
	if cert.Cert != nil {
		certValue = *cert.Cert
	}

	return dataPlaneCertificateRecord{
		ID:          pointerString(cert.ID),
		Fingerprint: resources.ShortControlPlaneDataPlaneCertificateIdentity(certValue),
		CreatedAt:   intPointerString(cert.CreatedAt),
		UpdatedAt:   intPointerString(cert.UpdatedAt),
	}
}

func dataPlaneCertificateDetailView(cert *kkComps.DataPlaneClientCertificate) string {
	if cert == nil {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", pointerString(cert.ID))
	fmt.Fprintf(&b, "fingerprint: %s\n", dataPlaneCertificateToRecord(cert).Fingerprint)
	fmt.Fprintf(&b, "created_at: %s\n", intPointerString(cert.CreatedAt))
	fmt.Fprintf(&b, "updated_at: %s\n", intPointerString(cert.UpdatedAt))
	fmt.Fprintf(&b, "cert: %s\n", pointerString(cert.Cert))
	return strings.TrimRight(b.String(), "\n")
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func intPointerString(value *int64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}

func newGetDataPlaneCertificateCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getDataPlaneCertificateCmd {
	rv := getDataPlaneCertificateCmd{
		Command: baseCmd,
	}

	baseCmd.Short = getDataPlaneCertificateShort
	baseCmd.Long = getDataPlaneCertificateLong
	baseCmd.Example = getDataPlaneCertificateExamples

	if addParentFlags != nil {
		addParentFlags(verb, baseCmd)
	}

	originalPreRunE := baseCmd.PreRunE
	baseCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if parentPreRun != nil {
			if err := parentPreRun(cmd, args); err != nil {
				return err
			}
		}
		if originalPreRunE != nil {
			if err := originalPreRunE(cmd, args); err != nil {
				return err
			}
		}
		return nil
	}
	baseCmd.RunE = rv.runE

	return &rv
}
