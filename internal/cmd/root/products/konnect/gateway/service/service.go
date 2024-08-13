package service

import (
	"fmt"

	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	serviceUse   = "service"
	serviceShort = i18n.T("root.products.konnect.gateway.service.serviceShort",
		"Manage Konnect Gateway Services")
	serviceLong = normalizers.LongDesc(i18n.T("root.products.konnect.gateway.service.serviceLong",
		`The service command allows you to manage Konect gateway service resources.`))
	serviceExamples = normalizers.Examples(i18n.T("root.products.konnect.gateway.service.serviceExamples",
		fmt.Sprintf(`
	# List the Konnect control planes for the current organization
	%[1]s get konnect gateway services 
	# Get a specific Konnect control plane
	%[1]s get konnect gateway service <service-id>
	`, meta.CLIName)))
)

func NewServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     serviceUse,
		Short:   serviceShort,
		Long:    serviceLong,
		Example: serviceExamples,
		Aliases: []string{"services", "svc", "svcs"},
		Run: func(_ *cobra.Command, _ []string) {
			// rb := root.NewRunBucket(streams, cmd, args)
			// util.CheckError(validate(rb))
			// util.CheckError(run(rb))
		},
	}

	// cmd.Flags().StringVar(&cpID, "cp-id", "", "The control plane ID to use")

	return cmd
}

//func validate(rb *root.RunBucket) error {
//	return nil
//}
//
//func run(rb *root.RunBucket) error {
//	return nil
//}
