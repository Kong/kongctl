package controlplane

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "control-plane"
)

var (
	eventGatewayControlPlaneUse   = CommandName
	eventGatewayControlPlaneShort = i18n.T("root.products.konnect.event-gateway.controlPlaneShort",
		"Manage Konnect event gateway control plane resources")
	eventGatewayControlPlaneLong = normalizers.LongDesc(i18n.T("root.products.konnect.event-gateway.controlPlaneLong",
		`The event gateway control plane command allows you to work with Konnect event gateway control plane resources.`))
	eventGatewayControlPlaneExample = normalizers.Examples(
		i18n.T("root.products.konnect.event-gateway.controlPlaneExamples",
			fmt.Sprintf(`
# List all the Konnect event gateway control planes for the organization
%[1]s get event-gateway control-planes
# Get a specific Konnect event gateway control plane
%[1]s get event-gateway control-plane <id|name>
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
		Aliases: []string{"control-planes", "cp", "cps", "CP", "CPS"},
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
