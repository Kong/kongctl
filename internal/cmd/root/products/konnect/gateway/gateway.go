package gateway

import (
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/consumer"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/controlplane"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/route"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/service"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	gatewayUse   = "gateway"
	gatewayShort = i18n.T("root.konnect.gateway.gatewayShort", "Manage Konnect Kong Gateway resources")
	gatewayLong  = normalizers.LongDesc(i18n.T("root.konnect.gateway.gatewayLong",
		`The gateway command allows you to manage Konnect Kong Gateway resources.`))
)

func NewGatewayCmd(verb verbs.VerbValue) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     gatewayUse,
		Short:   gatewayShort,
		Long:    gatewayLong,
		Aliases: []string{"gw", "GW"},
	}

	c, e := controlplane.NewControlPlaneCmd(verb)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

	c, e = service.NewServiceCmd(verb)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

	c, e = route.NewRouteCmd(verb)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

	c, e = consumer.NewConsumerCmd(verb)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

	return cmd, nil
}
