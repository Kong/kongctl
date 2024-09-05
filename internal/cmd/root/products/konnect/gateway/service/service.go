package service

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

func NewServiceCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
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

	addFlagsFunc := func(verb verbs.VerbValue, cmd *cobra.Command) {
		common.AddControlPlaneFlags(cmd)
		if addParentFlags != nil {
			addParentFlags(verb, cmd)
		}
	}

	if verb == verbs.Get || verb == verbs.List {
		return newGetServiceCmd(verb, &baseCmd, addFlagsFunc, parentPreRun).Command, nil
	}

	return &baseCmd, nil
}
