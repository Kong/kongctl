package route

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
	routeUse   = "route"
	routeShort = i18n.T("root.products.konnect.gateway.route.routeShort",
		"Manage Konnect Kong Gateway Routes")
	routeLong = normalizers.LongDesc(i18n.T("root.products.konnect.gateway.route.routeLong",
		`The route command allows you to work with Konect Kong Gateway Route resources.`))
	routeExamples = normalizers.Examples(i18n.T("root.products.konnect.gateway.route.routeExamples",
		fmt.Sprintf(`
	# List the Konnect routes 
	%[1]s get konnect gateway routes --control-plane-id <id>
	# Get a specific Konnect route
	%[1]s get konnect gateway route --control-plane-id <id> <id|name>
	`, meta.CLIName)))
)

func NewRouteCmd(verb verbs.VerbValue) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     routeUse,
		Short:   routeShort,
		Long:    routeLong,
		Example: routeExamples,
		Aliases: []string{"routes", "rt", "rts"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return common.BindControlPlaneFlags(cmd, args)
		},
	}

	common.AddControlPlaneFlags(&baseCmd)

	if verb == verbs.Get || verb == verbs.List {
		return newGetRouteCmd(&baseCmd).Command, nil
	}

	return &baseCmd, nil
}
