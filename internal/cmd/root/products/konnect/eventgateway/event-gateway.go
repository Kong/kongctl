package eventgateway

import (
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	eventGatewayUse   = "event-gateway"
	eventGatewayShort = i18n.T("root.konnect.event-gateway.gatewayShort", "Manage Konnect Event Gateway resources")
	eventGatewayLong  = normalizers.LongDesc(i18n.T("root.konnect.event-gateway.gatewayLong",
		`The event-gateway command allows you to manage Konnect Event Gateway resources.`))
)

func NewEventGatewayCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     eventGatewayUse,
		Short:   eventGatewayShort,
		Long:    eventGatewayLong,
		Aliases: []string{"egw", "EGW", "event-gateways"},
	}

	switch verb {
	case verbs.Get:
		return newGetEventGatewayControlPlaneCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.List:
		return newGetEventGatewayControlPlaneCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.Create, verbs.Add, verbs.Apply, verbs.Dump, verbs.Update, verbs.Delete, verbs.Help, verbs.Login,
		verbs.Plan, verbs.Sync, verbs.Diff, verbs.Export, verbs.Adopt, verbs.API, verbs.Kai, verbs.View, verbs.Logout,
		verbs.Patch:
		return &baseCmd, nil
	default:
		return &baseCmd, nil
	}
}
