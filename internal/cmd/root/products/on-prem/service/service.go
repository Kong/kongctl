package service

import (
	"fmt"

	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	serviceUse   = "service"
	serviceShort = i18n.T("root.products.on-prem.service.serviceShort",
		"Manage on-premises Kong Gateway Services")
	serviceLong = normalizers.LongDesc(i18n.T("root.products.on-prem.service.serviceLong",
		`The service command allows you to manage on-premises Kong Gateway service resources.`))
	serviceExamples = normalizers.Examples(i18n.T("root.products.on-prem.service.serviceExamples",
		fmt.Sprintf(`
	# List the on-premises Kong Gateway Services
	%[1]s get on-prem services 
	`, meta.CLIName)))
)

func NewServiceCmd(_ *iostreams.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     serviceUse,
		Short:   serviceShort,
		Long:    serviceLong,
		Example: serviceExamples,
		Aliases: []string{"services", "svc", "svcs"},
		Run: func(_ *cobra.Command, _ []string) {
			// rb, err := root.NewRunBucket(streams, cmd, args)
			// util.CheckError(err)
			// util.CheckError(validate(rb))
			// util.CheckError(run(rb))
		},
	}

	return cmd
}

//func validate(rb *root.RunBucket) error {
//	return nil
//}
//
//func run(rb *root.RunBucket) error {
//	return nil
//}
