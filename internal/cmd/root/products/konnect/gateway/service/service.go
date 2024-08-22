package service

import (
	"fmt"

	"github.com/kong/kong-cli/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kong-cli/internal/cmd/root/verbs"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	serviceUse   = "service"
	serviceShort = i18n.T("root.products.konnect.gateway.service.serviceShort",
		"Manage Konnect Kong Gateway Services")
	serviceLong = normalizers.LongDesc(i18n.T("root.products.konnect.gateway.service.serviceLong",
		`The gateway service command allows you to work with Konnect Kong Gateway Service resources.`))
	serviceExamples = normalizers.Examples(i18n.T("root.products.konnect.gateway.service.serviceExamples",
		fmt.Sprintf(`
	# List the Konnect Kong Gateway Services for the current organization
	%[1]s get konnect gateway services 
	# Get a specific Konnect Kong Gateway Service 
	%[1]s get konnect gateway service <id|name>
	`, meta.CLIName)))
)

func NewServiceCmd(verb verbs.VerbValue) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     serviceUse,
		Short:   serviceShort,
		Long:    serviceLong,
		Example: serviceExamples,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return common.BindControlPlaneFlags(cmd, args)
		},
		Aliases: []string{"services", "svc", "svcs"},
	}

	common.AddControlPlaneFlags(&baseCmd)

	if verb == verbs.Get || verb == verbs.List {
		return newGetServiceCmd(&baseCmd).Command, nil
	}

	return &baseCmd, nil
}
