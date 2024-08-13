package route

import (
	"fmt"

	"github.com/kong/kong-cli/internal/iostreams"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	routeUse   = "route"
	routeShort = i18n.T("root.products.gateway.route.routeShort",
		"Manage Kong Gateway Services")
	routeLong = normalizers.LongDesc(i18n.T("root.products.gateway.route.routeLong",
		`The route command allows you to manage Kong Gateway route resources.`))
	routeExamples = normalizers.Examples(i18n.T("root.products.gateway.route.routeExamples",
		fmt.Sprintf(`
	# List the Kong Gateway Routes 
	%[1]s get gateway routes 
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
