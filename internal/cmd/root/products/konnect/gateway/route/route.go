package route

import (
	"fmt"

	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	routeUse   = "route"
	routeShort = i18n.T("root.products.konnect.gateway.route.routeShort",
		"Manage Konnect Routes")
	routeLong = normalizers.LongDesc(i18n.T("root.products.konnect.gateway.route.routeLong",
		`The route command allows you to manage Konect gateway route resources.`))
	routeExamples = normalizers.Examples(i18n.T("root.products.konnect.gateway.route.routeExamples",
		fmt.Sprintf(`
	# List the Konnect routes 
	%[1]s get konnect gateway routes 
	# Get a specific Konnect route
	%[1]s get konnect gateway route <route-id>
	`, meta.CLIName)))
)

func NewRouteCmd() *cobra.Command {
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
