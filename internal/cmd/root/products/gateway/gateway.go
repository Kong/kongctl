package gateway

import (
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Product = products.ProductValue("gateway")
)

var (
	gatewayUse   = Product.String()
	gatewayShort = i18n.T("root.gateway.gatewayShort", "Manage Kong Gateway (on-prem) resources")
	gatewayLong  = normalizers.LongDesc(i18n.T("root.gateway.gatewayLong",
		`The gateway command allows you to manage Kong Gateway resources.`))
)

func NewGatewayCmd(_ *iostreams.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   gatewayUse,
		Short: gatewayShort,
		Long:  gatewayLong,
	}

	// TODO: Add service and route commands back in when they are implemented
	// cmd.AddCommand(service.NewServiceCmd(streams))
	// cmd.AddCommand(route.NewRouteCmd(streams))

	return cmd
}
