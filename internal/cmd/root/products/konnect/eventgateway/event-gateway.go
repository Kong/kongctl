package eventgateway

import (
	controlplane "github.com/kong/kongctl/internal/cmd/root/products/konnect/eventgateway/control-plane"

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
	cmd := &cobra.Command{
		Use:     eventGatewayUse,
		Short:   eventGatewayShort,
		Long:    eventGatewayLong,
		Aliases: []string{"egw", "EGW"},
	}

	c, e := controlplane.NewEventGatewayControlPlaneCmd(verb, addParentFlags, parentPreRun)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

	return cmd, nil
}
