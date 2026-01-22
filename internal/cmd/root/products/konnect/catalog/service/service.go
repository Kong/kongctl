package service

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	serviceUse   = "service"
	serviceShort = i18n.T("root.products.konnect.catalog.serviceShort",
		"Manage Konnect Service Catalog services")
	serviceLong = normalizers.LongDesc(i18n.T("root.products.konnect.catalog.serviceLong",
		`The service command allows you to work with Konnect Service Catalog services.`))
	serviceExample = normalizers.Examples(i18n.T("root.products.konnect.catalog.serviceExamples",
		fmt.Sprintf(`
	# List Service Catalog services
	%[1]s get catalog services
	# Get a specific Service Catalog service
	%[1]s get catalog service <id|name>
	`, meta.CLIName)))
)

// NewServiceCmd builds the catalog service command.
func NewServiceCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     serviceUse,
		Short:   serviceShort,
		Long:    serviceLong,
		Example: serviceExample,
		Aliases: []string{"services", "svc", "svcs"},
	}

	//nolint:exhaustive
	switch verb {
	case verbs.Get, verbs.List:
		return newGetServiceCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	default:
		return &baseCmd, nil
	}
}
