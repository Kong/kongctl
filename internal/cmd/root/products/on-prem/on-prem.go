package onprem

import (
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	// Product represents on-premises Kong Gateway
	// Note: This naming may change in the future based on product decisions
	Product = products.ProductValue("on-prem")
)

var (
	onPremUse   = Product.String()
	onPremShort = i18n.T("root.on-prem.onPremShort", "Manage on-premises Kong Gateway resources")
	onPremLong  = normalizers.LongDesc(i18n.T("root.on-prem.onPremLong",
		`The on-prem command allows you to manage on-premises Kong Gateway resources.

This is distinct from Konnect-hosted gateway resources which are accessed via the 'konnect gateway' commands.`))
)

func NewOnPremCmd(_ *iostreams.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   onPremUse,
		Short: onPremShort,
		Long:  onPremLong,
	}

	// TODO: Add service and route commands back in when they are implemented
	// cmd.AddCommand(service.NewServiceCmd(streams))
	// cmd.AddCommand(route.NewRouteCmd(streams))

	return cmd
}
