package route

import (
	"fmt"

	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	routeUse   = "route"
	routeShort = i18n.T("root.products.on-prem.route.routeShort",
		"Manage on-premises Kong Gateway Routes")
	routeLong = normalizers.LongDesc(i18n.T("root.products.on-prem.route.routeLong",
		`The route command allows you to manage on-premises Kong Gateway route resources.`))
	routeExamples = normalizers.Examples(i18n.T("root.products.on-prem.route.routeExamples",
		fmt.Sprintf(`
	# List the on-premises Kong Gateway Routes 
	%[1]s get on-prem routes 
	`, meta.CLIName)))
)

func NewRouteCmd(_ *iostreams.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     routeUse,
		Short:   routeShort,
		Long:    routeLong,
		Example: routeExamples,
		Aliases: []string{"routes", "rt", "rts"},
		Run: func(_ *cobra.Command, _ []string) {
			// rb := root.NewRunBucket(streams, cmd, args)
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
