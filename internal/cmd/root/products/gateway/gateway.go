package gateway

import (
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
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

func NewGatewayCmd(verb verbs.VerbValue) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     gatewayUse,
		Short:   gatewayShort,
		Long:    gatewayLong,
		Aliases: []string{"g", "G"},
	}

	if verb == verbs.Apply {
		return newApplyGatewayCmd(verb, cmd).Command, nil
	}

	return cmd, nil
}
