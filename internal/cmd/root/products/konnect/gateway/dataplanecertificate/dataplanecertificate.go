package dataplanecertificate

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	dataPlaneCertificateUse   = "data-plane-certificates"
	dataPlaneCertificateShort = i18n.T("root.products.konnect.gateway.data-plane-certificate.short",
		"Manage Konnect Kong Gateway data plane certificates")
	dataPlaneCertificateLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.gateway.data-plane-certificate.long",
		`The data-plane-certificates command allows you to work with Konnect Kong Gateway data plane certificates.`,
	))
	dataPlaneCertificateExamples = normalizers.Examples(i18n.T(
		"root.products.konnect.gateway.data-plane-certificate.examples",
		fmt.Sprintf(`
	# List data plane certificates for a control plane
	%[1]s get konnect gateway control-plane data-plane-certificates --control-plane-id <id>
	# Get a specific data plane certificate
	%[1]s get konnect gateway control-plane data-plane-certificates --control-plane-id <id> <certificate-id>
	`, meta.CLIName),
	))
)

func NewDataPlaneCertificateCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     dataPlaneCertificateUse,
		Short:   dataPlaneCertificateShort,
		Long:    dataPlaneCertificateLong,
		Example: dataPlaneCertificateExamples,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return common.BindControlPlaneFlags(cmd, args)
		},
		Aliases: []string{
			"data-plane-certificate",
			"data-plane-cert",
			"data-plane-certs",
			"dp-cert",
			"dp-certs",
			"dpc",
			"dpcs",
		},
	}

	addFlagsFunc := func(verb verbs.VerbValue, cmd *cobra.Command) {
		common.AddControlPlaneFlags(cmd)
		if addParentFlags != nil {
			addParentFlags(verb, cmd)
		}
	}

	if verb == verbs.Get || verb == verbs.List {
		return newGetDataPlaneCertificateCmd(verb, &baseCmd, addFlagsFunc, parentPreRun).Command, nil
	}

	return &baseCmd, nil
}
