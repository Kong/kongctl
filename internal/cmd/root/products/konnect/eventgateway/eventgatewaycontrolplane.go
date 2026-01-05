package eventgateway

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "eventgatewaycontrolplane"
)

var (
	eventGatewayControlPlaneUse   = CommandName
	eventGatewayControlPlaneShort = i18n.T("root.products.konnect.eventgateway.eventGatewayControlPlaneShort",
		"Manage Konnect event gateway control plane resources")
	eventGatewayControlPlaneLong = normalizers.LongDesc(i18n.T("root.products.konnect.eventgateway.eventGatewayControlPlaneLong",
		`The event gateway control plane command allows you to work with Konnect event gateway control plane resources.`))
	eventGatewayControlPlaneExample = normalizers.Examples(
		i18n.T("root.products.konnect.eventgateway.eventGatewayControlPlaneExamples",
			fmt.Sprintf(`
# List all the Konnect event gateway control planes for the organization
%[1]s get eventgatewaycontrolplanes
# Get a specific Konnect event gateway control plane
%[1]s get eventgatewaycontrolplane <id|name>
# List portal pages
%[1]s get portal pages --portal-id <portal-id>
# List portal applications
%[1]s get portal applications --portal-id <portal-id>
# List portals using explicit konnect product
%[1]s get konnect portals
`, meta.CLIName)))
)

func NewEventGatewayControlPlaneCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     eventGatewayControlPlaneUse,
		Short:   eventGatewayControlPlaneShort,
		Long:    eventGatewayControlPlaneLong,
		Example: eventGatewayControlPlaneExample,
		Aliases: []string{"eventgatewaycontrolplanes", "p", "ps", "P", "PS"},
	}

	switch verb {
	case verbs.Get:
		return newGetEventGatewayControlPlaneCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.Create, verbs.Add, verbs.Apply, verbs.Dump, verbs.Update, verbs.Help, verbs.Login,
		verbs.Plan, verbs.Sync, verbs.Diff, verbs.Export, verbs.Adopt, verbs.API, verbs.Kai, verbs.View, verbs.Logout:
		return &baseCmd, nil
	}

	return &baseCmd, nil
}
